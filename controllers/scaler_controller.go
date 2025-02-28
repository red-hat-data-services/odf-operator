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
	"time"

	"github.com/go-logr/logr"
	"github.com/red-hat-storage/odf-operator/pkg/util"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type ScalerReconciler struct {
	ctx               context.Context
	logger            logr.Logger
	Client            client.Client
	Scheme            *runtime.Scheme
	OperatorNamespace string

	foundKinds []string
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

var (
	kinds = []metav1.PartialObjectMetadata{
		*kindStorageCluster,
		*kindCephCluster,
		*kindFlashSystemCluster,
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

	r.logger.Info("starting reconcile")

	/*
		for _, kind := range kinds {
			objects := &metav1.PartialObjectMetadataList{}
			objects.TypeMeta = kind.TypeMeta

			r.logger.Info("nigoyal listing", "kind", kind.TypeMeta.Kind)
			err := r.Client.List(ctx, objects)
			if err != nil {
				if meta.IsNoMatchError(err) {
					r.logger.Info("nigoyal continue")
					continue
				}

				r.logger.Error(err, "nigoyal failed to list objects")
			}

			for _, item := range objects.Items {
				r.logger.Info("nigoyal list", "name", item.GetName())
			}
		}
	*/

	return ctrl.Result{}, nil
}

// watchStorageCluster watches for the All CR and writes events when it exists
func (r *ScalerReconciler) watchObjects(events chan<- event.GenericEvent) {
	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		<-ticker.C // Wait for the next tick

		ctx := context.Background()
		logger := log.FromContext(ctx)

		var currKinds []string

		for _, kind := range kinds {
			objects := &metav1.PartialObjectMetadataList{}
			objects.TypeMeta = kind.TypeMeta

			logger.Info("watchObjects: listing objects", "kind", kind.TypeMeta.Kind)
			err := r.Client.List(ctx, objects)

			if err != nil {
				if meta.IsNoMatchError(err) {
					logger.Info("watchObjects: kind is not registered in the cluster", "kind", kind.TypeMeta.Kind)
					continue
				}

				if errors.IsForbidden(err) {
					logger.Info("watchObjects: required to add permissions to list the objects", "kind", kind.TypeMeta.Kind)
					continue
				}

				logger.Error(err, "watchObjects: failed to list objects", "kind", kind.TypeMeta.Kind)
			}

			if len(objects.Items) > 0 {
				currKinds = append(currKinds, kind.TypeMeta.Kind)

				if !util.FindInSlice(r.foundKinds, kind.TypeMeta.Kind) {
					logger.Info("watchObjects: send signal to channel for enqueue", "kind", kind.TypeMeta.Kind)
					events <- event.GenericEvent{
						Object: &objects.Items[0],
					}
				}
			}
		}
		r.foundKinds = currKinds
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {

	events := make(chan event.GenericEvent)

	go r.watchObjects(events)

	return ctrl.NewControllerManagedBy(mgr).
		Named("scaler").
		WatchesRawSource(
			source.Channel(
				events,
				handler.EnqueueRequestsFromMapFunc(
					func(ctx context.Context, obj client.Object) []reconcile.Request {
						return []reconcile.Request{{}}
					},
				),
			),
		).Complete(r)
}
