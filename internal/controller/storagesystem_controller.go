/*
Copyright 2024 Red Hat OpenShift Data Foundation.

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

package controller

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	odfv1alpha1 "github.com/red-hat-storage/odf-operator/api/v1alpha1"
)

// StorageSystemReconciler reconciles a StorageSystem object
type StorageSystemReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	context           context.Context
	logger            logr.Logger
	OperatorNamespace string
}

//+kubebuilder:rbac:groups=odf.openshift.io,resources=storagesystems,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=odf.openshift.io,resources=storagesystems/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=odf.openshift.io,resources=storagesystems/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations,verbs=get;list;watch;create;update;patch;delete

func (r *StorageSystemReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.context = ctx
	r.logger = log.FromContext(r.context)

	r.logger.Info("Starting reconcile")

	if err := r.reconcileWebhookService(); err != nil {
		r.logger.Error(err, "unable to reconcile webhook service")
		return ctrl.Result{}, err
	}

	if err := r.reconcileSubscriptionValidatingWebhook(); err != nil {
		r.logger.Error(err, "unable to register subscription validating webhook")
		return ctrl.Result{}, err
	}

	r.logger.Info("Successfully completed reconcile")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *StorageSystemReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&odfv1alpha1.StorageSystem{}).
		Complete(r)
}
