/*
Copyright 2021 Red Hat OpenShift Data Foundation.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"go.uber.org/multierr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	ocsv1 "github.com/red-hat-storage/ocs-operator/api/v4/v1"
	odfv1alpha1 "github.com/red-hat-storage/odf-operator/api/v1alpha1"
	"github.com/red-hat-storage/odf-operator/metrics"
	"github.com/red-hat-storage/odf-operator/pkg/util"
)

const (
	storageSystemFinalizer = "storagesystem.odf.openshift.io"
)

// StorageSystemReconciler reconciles a StorageSystem object
type StorageSystemReconciler struct {
	ctx               context.Context
	Client            client.Client
	Scheme            *runtime.Scheme
	Recorder          *EventReporter
	OperatorNamespace string
}

//+kubebuilder:rbac:groups=odf.openshift.io,resources=storagesystems,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=odf.openshift.io,resources=storagesystems/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=odf.openshift.io,resources=storagesystems/finalizers,verbs=update
//+kubebuilder:rbac:groups=ocs.openshift.io,resources=storageclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=odf.ibm.com,resources=flashsystemclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=catalogsources,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=clusterserviceversions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions/finalizers,verbs=update
//+kubebuilder:rbac:groups=console.openshift.io,resources=consolequickstarts,verbs=get;list;create;update;delete
//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch;create;update
//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=delete

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *StorageSystemReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	instance := &odfv1alpha1.StorageSystem{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("storagesystem instance not found")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	logger.Info("storagesystem instance found")

	metrics.ReportODFSystemMapMetrics(instance.Name, instance.Spec.Name, instance.Spec.Namespace, string(instance.Spec.Kind))

	// Reconcile changes
	result, reconcileError := r.reconcile(instance, logger)

	// Apply status changes
	statusError := r.Client.Status().Update(context.TODO(), instance)
	if statusError != nil {
		logger.Error(statusError, "failed to update status")
	}

	// Reconcile errors have higher priority than status update errors
	if reconcileError != nil {
		r.Recorder.ReportIfNotPresent(instance, corev1.EventTypeWarning, EventReasonReconcileFailed, reconcileError.Error())
		return result, reconcileError
	} else if statusError != nil {
		return result, statusError
	} else {
		return result, nil
	}
}

func (r *StorageSystemReconciler) reconcile(instance *odfv1alpha1.StorageSystem, logger logr.Logger) (ctrl.Result, error) {

	var err error

	if instance.Status.Conditions == nil {
		SetReconcileInitConditions(&instance.Status.Conditions, "Init", "Initializing StorageSystem")
	} else {
		SetReconcileStartConditions(&instance.Status.Conditions, "Reconciling", "Reconcile is in progress")
	}

	if err = r.validateStorageSystemSpec(instance, logger); err != nil {
		logger.Error(err, "failed to validate storagesystem")
		return reconcile.Result{}, err
	}

	// deletion phase
	if !instance.GetDeletionTimestamp().IsZero() {
		if util.FindInSlice(instance.GetFinalizers(), storageSystemFinalizer) {
			SetDeletionInProgressConditions(&instance.Status.Conditions, "Deleting", "Deletion is in progress")

			err = r.deleteResources(instance, logger)
			if err != nil {
				return ctrl.Result{}, err
			}

			logger.Info("storagesystem is in deletion phase remove finalizer", "Finalizer", storageSystemFinalizer)
			instance.ObjectMeta.Finalizers = util.RemoveFromSlice(instance.ObjectMeta.Finalizers, storageSystemFinalizer)
			if err := r.updateStorageSystem(instance); err != nil {
				logger.Error(err, "failed to remove finalizer from storagesystem", "Finalizer", storageSystemFinalizer)
				return ctrl.Result{}, err
			}
		}
		logger.Info("storagesystem object is terminated, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// ensure finalizer
	if !util.FindInSlice(instance.GetFinalizers(), storageSystemFinalizer) {
		logger.Info("finalizer not found Add finalizer", "Finalizer", storageSystemFinalizer)
		instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, storageSystemFinalizer)
		if err = r.updateStorageSystem(instance); err != nil {
			logger.Error(err, "failed to update storagesystem with finalizer", "Finalizer", storageSystemFinalizer)
			return ctrl.Result{}, err
		}
	}

	err = r.ensureQuickStarts(logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = ensureWebhook(r.ctx, r.Client, logger, r.OperatorNamespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.ensureSubscriptions(instance, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.isVendorCsvReady(instance, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.isVendorSystemPresent(instance, logger)
	if err != nil {
		return ctrl.Result{}, err
	}

	SetReconcileCompleteConditions(&instance.Status.Conditions, "ReconcileCompleted", "Reconcile is completed successfully")

	return ctrl.Result{}, nil
}

func (r *StorageSystemReconciler) updateStorageSystem(instance *odfv1alpha1.StorageSystem) error {

	// save the status locally before the Update call, as update call does not update the status and we lost it
	instanceStatus := instance.Status.DeepCopy()
	err := r.Client.Update(context.TODO(), instance)
	instance.Status = *(instanceStatus)
	return err
}

func (r *StorageSystemReconciler) validateStorageSystemSpec(instance *odfv1alpha1.StorageSystem, logger logr.Logger) error {

	if instance.Spec.Kind != VendorStorageCluster() && instance.Spec.Kind != VendorFlashSystemCluster() {
		err := fmt.Errorf("unsupported kind %s", instance.Spec.Kind)
		r.Recorder.ReportIfNotPresent(instance, corev1.EventTypeWarning, EventReasonValidationFailed, err.Error())
		SetStorageSystemInvalidConditions(&instance.Status.Conditions, "NotValid", err.Error())
		return err
	} else {
		SetStorageSystemInvalidCondition(&instance.Status.Conditions, corev1.ConditionFalse, "Valid", "StorageSystem CR is valid")
	}

	return nil
}

func (r *StorageSystemReconciler) ensureSubscriptions(instance *odfv1alpha1.StorageSystem, logger logr.Logger) error {

	var err error

	subs := GetSubscriptions(instance.Spec.Kind)
	if len(subs) == 0 {
		return fmt.Errorf("no subscriptions defined for kind: %v", instance.Spec.Kind)
	}

	for _, desiredSubscription := range subs {
		err = EnsureDesiredSubscription(r.Client, desiredSubscription)
		if err != nil && !errors.IsAlreadyExists(err) {
			logger.Error(err, "failed to ensure subscription", "Subscription", desiredSubscription.Name)
			return err
		}
	}

	return nil
}

func (r *StorageSystemReconciler) isVendorCsvReady(instance *odfv1alpha1.StorageSystem, logger logr.Logger) error {

	csvNames, err := GetVendorCsvNames(r.Client, instance.Spec.Kind)
	if err != nil {
		return err
	}

	var returnErr error
	for _, csvName := range csvNames {

		csvObj, err := EnsureVendorCsv(r.Client, csvName)
		if err != nil {
			logger.Error(err, "failed to validate CSV", "ClusterServiceVersion", csvName)
			multierr.AppendInto(&returnErr, err)
			continue
		}

		logger.Info("vendor CSV is installed and ready", "ClusterServiceVersion", csvObj.Name)

	}

	if returnErr != nil {
		SetVendorCsvReadyCondition(&instance.Status.Conditions, corev1.ConditionFalse, "NotReady", returnErr.Error())
	} else {
		SetVendorCsvReadyCondition(&instance.Status.Conditions, corev1.ConditionTrue, "Ready", "")
	}

	return returnErr
}

// SetupWithManager sets up the controller with the Manager.
func (r *StorageSystemReconciler) SetupWithManager(mgr ctrl.Manager) error {

	generationChangedPredicate := predicate.GenerationChangedPredicate{}

	ignoreCreatePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Ignore create events as resource created by us
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&odfv1alpha1.StorageSystem{}, builder.WithPredicates(generationChangedPredicate)).
		Owns(&operatorv1alpha1.Subscription{}, builder.WithPredicates(generationChangedPredicate, ignoreCreatePredicate)).
		// Although we own the storage cluster, we are not a controller owner.
		// Not being a controller owner requires us to pass builder.MatchEveryOwner.
		Owns(&ocsv1.StorageCluster{}, builder.MatchEveryOwner, builder.WithPredicates(generationChangedPredicate)).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
