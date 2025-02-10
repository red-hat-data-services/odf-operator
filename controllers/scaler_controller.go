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

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ScalerReconciler struct {
	ctx               context.Context
	logger            logr.Logger
	Client            client.Client
	Scheme            *runtime.Scheme
	OperatorNamespace string
}

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
	kindFlashSystemCluster = &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "odf.ibm.com/v1alpha1",
			Kind:       "FlashSystemCluster",
		},
	}
)

//+kubebuilder:rbac:groups=ocs.openshift.io,resources=storageclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=ceph.rook.io,resources=cephclusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=odf.ibm.com,resources=flashsystemclusters,verbs=get;list;watch

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *ScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.ctx = ctx
	r.logger = log.FromContext(ctx)

	r.logger.Info("nigoyal starting reconcile")

	for _, kind := range []metav1.PartialObjectMetadata{
		*kindStorageCluster,
		*kindCephCluster,
		*kindFlashSystemCluster,
	} {
		objects := &metav1.PartialObjectMetadataList{}
		objects.TypeMeta = kind.TypeMeta

		r.logger.Info("nigoyal listing", "kind", kind.TypeMeta.Kind)
		err := r.Client.List(ctx, objects)
		if err != nil {
			r.logger.Error(err, "nigoyal failed to list objects")
		}

		for _, item := range objects.Items {
			r.logger.Info("nigoyal list", "name", item.GetName())
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("scaler").
		WatchesMetadata(
			kindStorageCluster,
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		WatchesMetadata(
			kindCephCluster,
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		//WatchesRawSource(
		//	source.Kind(
		//		mgr.GetCache(),
		//		client.Object(kindFlashSystemCluster),
		//		handler.EnqueueRequestsFromMapFunc(
		//			func(ctx context.Context, obj client.Object) []reconcile.Request {
		//				return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: "", Namespace: ""}}}
		//			},
		//		),
		//	),
		//).
		Complete(r)
}
