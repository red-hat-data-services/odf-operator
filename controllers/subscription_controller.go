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
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorv2 "github.com/operator-framework/api/pkg/operators/v2"
	"github.com/operator-framework/operator-lib/conditions"
	odfv1alpha1 "github.com/red-hat-storage/odf-operator/api/v1alpha1"
	"github.com/red-hat-storage/odf-operator/pkg/util"
)

// SubscriptionReconciler reconciles a Subscription object
type SubscriptionReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	Recorder          *EventReporter
	ConditionName     string
	OperatorCondition conditions.Condition
}

//+kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operators.coreos.com,resources=subscriptions/finalizers,verbs=update
//+kubebuilder:rbac:groups=operators.coreos.com,resources=installplans,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operators.coreos.com,resources=clusterserviceversions/finalizers,verbs=update
//+kubebuilder:rbac:groups=operators.coreos.com,resources=operatorconditions,verbs=get;list;watch

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *SubscriptionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {

	logger := log.FromContext(ctx)

	instance := &operatorv1alpha1.Subscription{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Subscription instance not found.")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	err = r.setOperatorCondition(logger, req.NamespacedName.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.ensureSubscriptions(logger, req.NamespacedName.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *SubscriptionReconciler) ensureSubscriptions(logger logr.Logger, namespace string) error {

	var err error

	subsList := map[odfv1alpha1.StorageKind][]*operatorv1alpha1.Subscription{}
	subsList[StorageClusterKind] = GetSubscriptions(StorageClusterKind)

	ssList := &odfv1alpha1.StorageSystemList{}
	err = r.Client.List(context.TODO(), ssList, &client.ListOptions{Namespace: namespace})
	if err != nil {
		return err
	}

	for _, ss := range ssList.Items {
		subsList[ss.Spec.Kind] = GetSubscriptions(ss.Spec.Kind)
	}

	for _, subs := range subsList {
		for _, sub := range subs {
			errSub := EnsureDesiredSubscription(r.Client, sub)
			if errSub != nil {
				logger.Error(errSub, "failed to ensure Subscription", "Subscription", sub.Name)
				err = fmt.Errorf("failed to ensure Subscriptions")
				multierr.AppendInto(&err, fmt.Errorf("failed to ensure Subscriptions"))
			}
		}
	}

	if err != nil {
		return err
	}

	for kind := range subsList {
		csvNames, csvErr := GetVendorCsvNames(r.Client, kind)
		if csvErr != nil {
			return csvErr
		}

		for _, csvName := range csvNames {
			_, csvErr := EnsureVendorCsv(r.Client, csvName)
			if csvErr != nil {
				multierr.AppendInto(&err, csvErr)
			}
		}
	}

	return err
}

func (r *SubscriptionReconciler) setOperatorCondition(logger logr.Logger, namespace string) error {
	ocdList := &operatorv2.OperatorConditionList{}
	err := r.Client.List(context.TODO(), ocdList, client.InNamespace(namespace))
	if err != nil {
		logger.Error(err, "failed to list OperatorConditions")
		return err
	}

	condNames, err := GetVendorCsvNames(r.Client, StorageClusterKind)
	if err != nil {
		return err
	}

	condNamesFlashSystem, err := GetVendorCsvNames(r.Client, FlashSystemKind)
	if err != nil {
		return err
	}

	condNames = append(condNames, condNamesFlashSystem...)

	condMap := make(map[string]struct{}, len(condNames))
	for i := range condNames {
		condMap[condNames[i]] = struct{}{}
	}

	for ocdIdx := range ocdList.Items {
		// skip operatorconditions of not dependent operators
		if _, exist := condMap[ocdList.Items[ocdIdx].GetName()]; !exist {
			continue
		}

		ocd := &ocdList.Items[ocdIdx]
		cond := getNotUpgradeableCond(ocd)
		if cond != nil {
			// operator is not upgradeable
			msg := fmt.Sprintf("%s:%s", ocd.GetName(), cond.Message)
			logger.Info("setting operator upgradeable status", "status", cond.Status)
			return r.OperatorCondition.Set(context.TODO(), cond.Status,
				conditions.WithReason(cond.Reason), conditions.WithMessage(msg))
		}
	}

	// all operators are upgradeable
	status := metav1.ConditionTrue
	logger.Info("setting operator upgradeable status", "status", status)
	return r.OperatorCondition.Set(context.TODO(), status,
		conditions.WithReason("Dependents"), conditions.WithMessage("No dependent reports not upgradeable status"))
}

// SetupWithManager sets up the controller with the Manager.
func (r *SubscriptionReconciler) SetupWithManager(mgr ctrl.Manager) error {

	generationChangedPredicate := predicate.GenerationChangedPredicate{}

	predicateFunc := func(obj runtime.Object) bool {
		instance, ok := obj.(*operatorv1alpha1.Subscription)
		if !ok {
			return false
		}

		// ignore if not a odf-operator subscription
		if instance.Spec.Package != "odf-operator" {
			return false
		}

		return true
	}

	subscriptionPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return predicateFunc(e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return predicateFunc(e.Object)
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return predicateFunc(e.ObjectNew)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return predicateFunc(e.Object)
		},
	}

	conditionPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			// not mandatory but these checks wouldn't harm
			if e.ObjectOld == nil || e.ObjectNew == nil {
				return false
			}
			oldObj, _ := e.ObjectOld.(*operatorv2.OperatorCondition)
			newObj, _ := e.ObjectNew.(*operatorv2.OperatorCondition)
			if oldObj == nil || newObj == nil {
				return false
			}

			// skip sending a reconcile event if our own condition is updated
			if newObj.GetName() == r.ConditionName {
				return false
			}

			// change in admin set conditions for upgradeability
			oldOverride := util.Find(oldObj.Spec.Overrides, func(cond *metav1.Condition) bool {
				return cond.Type == operatorv2.Upgradeable
			})
			newOverride := util.Find(newObj.Spec.Overrides, func(cond *metav1.Condition) bool {
				return cond.Type == operatorv2.Upgradeable
			})
			if oldOverride != nil && newOverride == nil {
				// override is removed
				return true
			}
			if newOverride != nil {
				if oldOverride == nil {
					return true
				}
				return oldOverride.Status != newOverride.Status
			}

			// change in operator set conditions for upgradeability
			oldCond := util.Find(oldObj.Status.Conditions, func(cond *metav1.Condition) bool {
				return cond.Type == operatorv2.Upgradeable
			})
			newCond := util.Find(newObj.Status.Conditions, func(cond *metav1.Condition) bool {
				return cond.Type == operatorv2.Upgradeable
			})
			if newCond != nil {
				if oldCond == nil {
					return true
				}
				return oldCond.Status != newCond.Status
			}

			return false
		},
	}
	enqueueFromCondition := handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, obj client.Object) []reconcile.Request {
			if _, ok := obj.(*operatorv2.OperatorCondition); !ok {
				return []reconcile.Request{}
			}
			logger := log.FromContext(ctx)
			sub, err := GetOdfSubscription(r.Client)
			if err != nil {
				logger.Error(err, "failed to get ODF Subscription")
				return []reconcile.Request{}
			}
			return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: sub.Name, Namespace: sub.Namespace}}}
		},
	)

	err := mgr.Add(manager.RunnableFunc(func(ctx context.Context) error {
		odfDepsSub := GetStorageClusterSubscriptions()[0]
		return EnsureDesiredSubscription(r.Client, odfDepsSub)
	}))
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.Subscription{},
			builder.WithPredicates(generationChangedPredicate, subscriptionPredicate)).
		Owns(&operatorv1alpha1.Subscription{},
			builder.WithPredicates(generationChangedPredicate)).
		Watches(&operatorv2.OperatorCondition{}, enqueueFromCondition, builder.WithPredicates(conditionPredicate)).
		Complete(r)
}

func getNotUpgradeableCond(ocd *operatorv2.OperatorCondition) *metav1.Condition {
	cond := util.Find(ocd.Spec.Overrides, func(cd *metav1.Condition) bool {
		return cd.Type == operatorv2.Upgradeable
	})
	if cond != nil {
		if cond.Status != "True" {
			return cond
		}
		// if upgradeable is overridden we should skip checking operator set conditions
		return nil
	}

	return util.Find(ocd.Status.Conditions, func(cd *metav1.Condition) bool {
		return cd.Type == operatorv2.Upgradeable && cd.Status != "True"
	})
}
