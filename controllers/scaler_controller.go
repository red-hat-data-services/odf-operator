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
	"slices"

	"github.com/go-logr/logr"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var csvLabelKey = "operators.coreos.com/%s.%s"

var (
	kindStorageCluster = &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "ocs.openshift.io/v1",
			Kind:       "StorageCluster",
		},
	}
	kindCephCluster = &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "ceph.rook.io/v1",
			Kind:       "CephCluster",
		},
	}
	kindNoobaa = &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "noobaa.io/v1alpha1",
			Kind:       "NooBaa",
		},
	}
	kindFlashSystemCluster = &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "odf.ibm.com/v1alpha1",
			Kind:       "FlashSystemCluster",
		},
	}

	//nolint:unused
	kindList = []metav1.PartialObjectMetadata{
		*kindStorageCluster,
		*kindCephCluster,
		*kindNoobaa,
		*kindFlashSystemCluster,
	}
)

var (
	crdStorageCluster     = "storageclusters.ocs.openshift.io"
	crdCephCluster        = "cephclusters.ceph.rook.io"
	crdNoobaa             = "noobaas.noobaa.io"
	crdFlashSystemCluster = "flashsystemclusters.odf.ibm.com"

	crdList = []string{
		crdStorageCluster,
		crdCephCluster,
		crdNoobaa,
		crdFlashSystemCluster,
	}
)

var (
	csvCephcsiOperator       = "cephcsi-operator" //nolint:unused
	csvMcgOperator           = "mcg-operator"
	csvOcsClientOperator     = "ocs-client-operator" //nolint:unused
	csvOcsOperator           = "ocs-operator"
	csvOdfCsiAddonsOperator  = "odf-csi-addons-operator" //nolint:unused
	csvOdfDependencies       = "odf-dependencies"        //nolint:unused
	csvOdfOperator           = "odf-operator"            //nolint:unused
	csvOdfPrometheusOperator = "odf-prometheus-operator" //nolint:unused
	csvRecipe                = "recipe"                  //nolint:unused
	csvRookCephOperator      = "rook-ceph-operator"

	csvIbmStorageOdfOperator = "ibm-storage-odf-operator"
)

var (
	crdToKindMapping = map[string]metav1.PartialObjectMetadata{
		crdStorageCluster:     *kindStorageCluster,
		crdCephCluster:        *kindCephCluster,
		crdNoobaa:             *kindNoobaa,
		crdFlashSystemCluster: *kindFlashSystemCluster,
	}

	kindToCsvsMapping = map[*metav1.PartialObjectMetadata][]string{
		kindStorageCluster:     []string{csvOcsOperator},
		kindCephCluster:        []string{csvRookCephOperator},
		kindNoobaa:             []string{csvMcgOperator},
		kindFlashSystemCluster: []string{csvIbmStorageOdfOperator},
	}
)

type ScalerReconciler struct {
	ctx               context.Context
	logger            logr.Logger
	Client            client.Client
	Scheme            *runtime.Scheme
	OperatorNamespace string
	controller        controller.Controller
	mgr               ctrl.Manager
}

//+kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
//+kubebuilder:rbac:groups=operators.coreos.com,resources=clusterserviceversions,verbs=get;list;update

//+kubebuilder:rbac:groups=ocs.openshift.io,resources=storageclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=ceph.rook.io,resources=cephclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=noobaa.io,resources=noobaas,verbs=get;list;watch
//+kubebuilder:rbac:groups=odf.ibm.com,resources=flashsystemclusters,verbs=get;list;watch

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ctx = ctx
	r.logger = log.FromContext(ctx)

	r.logger.Info("starting reconcile")

	err := r.addWatches(req)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.scaleOperators()
	if err != nil {
		return ctrl.Result{}, err
	}

	r.logger.Info("ending reconcile")
	return ctrl.Result{}, nil
}

func (r *ScalerReconciler) scaleOperators() error {

	for kind, csvNames := range kindToCsvsMapping {

		var replicas int32 = 1
		objects := &metav1.PartialObjectMetadataList{}
		objects.TypeMeta = kind.TypeMeta

		err := r.Client.List(r.ctx, objects)
		if err != nil {
			if meta.IsNoMatchError(err) {
				continue
			}
			r.logger.Error(err, "failed to list objects")
			return err
		}

		if len(objects.Items) == 0 {
			replicas = 0
		}

		for _, csvName := range csvNames {
			key := fmt.Sprintf(csvLabelKey, csvName, r.OperatorNamespace)

			csvList := &operatorv1alpha1.ClusterServiceVersionList{}
			err = r.Client.List(
				r.ctx, csvList,
				&client.ListOptions{
					LabelSelector: labels.SelectorFromSet(map[string]string{key: ""}),
					Namespace:     r.OperatorNamespace,
				},
			)
			if err != nil {
				r.logger.Error(err, "failed to list csvs of label", "label", key)
				return err
			}

			for _, csvObj := range csvList.Items {
				var isUpdateRequired bool

				for i := range csvObj.Spec.InstallStrategy.StrategySpec.DeploymentSpecs {
					if replicas != *csvObj.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[i].Spec.Replicas {
						csvObj.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[i].Spec.Replicas = &replicas
						isUpdateRequired = true
					}
				}

				if isUpdateRequired {
					err = r.Client.Update(r.ctx, &csvObj)
					if err != nil {
						r.logger.Error(err, "failed to update csv replica", "replicas", replicas)
						return err
					}
					r.logger.Info("csv updated successfully", "csvName", csvObj.Name, "replicas", replicas)
				}
			}
		}
	}

	return nil
}

func (r *ScalerReconciler) addWatches(req ctrl.Request) error {

	if !slices.Contains(crdList, req.Name) {
		return nil
	}

	r.logger.Info("adding dynamic watch", "object", req.Name)

	kind := crdToKindMapping[req.Name]
	err := r.addDynamicWatch(&kind)
	if err != nil {
		r.logger.Error(err, "failed to add dynamic watch", "object", req.Name)
		return err
	}

	return nil
}

func (r *ScalerReconciler) addDynamicWatch(kind *metav1.PartialObjectMetadata) error {
	return r.controller.Watch(
		source.Kind(
			r.mgr.GetCache(),
			client.Object(kind),
			&handler.EnqueueRequestForObject{},
			predicate.Funcs{
				// Trigger the reconcile for both creation and deletion events of the object.
				// This ensures the replicas in the CSV are updated based on the presence or absence of the Custom Resource (CR).
				CreateFunc: func(e event.CreateEvent) bool {
					return true
				},
				DeleteFunc: func(e event.DeleteEvent) bool {
					return true
				},
				UpdateFunc: func(e event.UpdateEvent) bool {
					return false
				},
				GenericFunc: func(e event.GenericEvent) bool {
					return false
				},
			},
			predicate.GenerationChangedPredicate{},
		),
	)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {

	controller, err := ctrl.NewControllerManagedBy(mgr).
		Named("scaler").
		WatchesMetadata(
			&extv1.CustomResourceDefinition{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(
				predicate.NewPredicateFuncs(func(obj client.Object) bool {
					return slices.Contains(crdList, obj.GetName())
				}),
				predicate.Funcs{
					// Trigger a reconcile only during the creation of a specific CRD to ensure it runs exactly once for that CRD.
					// This is required to dynamically add a watch for the corresponding Custom Resource (CR) based on the CRD name.
					// The reconcile will be triggered with the CRD name as `req.Name`, and the reconciler will set up a watch for the CR using the CRD name.
					CreateFunc: func(e event.CreateEvent) bool {
						return true
					},
					DeleteFunc: func(e event.DeleteEvent) bool {
						return false
					},
					UpdateFunc: func(e event.UpdateEvent) bool {
						return false
					},
					GenericFunc: func(e event.GenericEvent) bool {
						return false
					},
				},
				predicate.GenerationChangedPredicate{},
			),
		).
		Build(r)

	r.controller = controller
	r.mgr = mgr

	return err
}
