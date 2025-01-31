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
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/red-hat-storage/odf-operator/console"
	"github.com/red-hat-storage/odf-operator/pkg/util"
)

// ClusterVersionReconciler reconciles a ClusterVersion object
type ClusterVersionReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	ConsolePort       int
	OperatorNamespace string
}

//+kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions/finalizers,verbs=update
//+kubebuilder:rbac:groups="apps",resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="apps",resources=deployments/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=console.openshift.io,resources=consoleplugins,verbs=get;create;update
//+kubebuilder:rbac:groups=console.openshift.io,resources=consoleclidownloads,verbs=get;create;update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *ClusterVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	instance := configv1.ClusterVersion{}
	if err := r.Client.Get(context.TODO(), req.NamespacedName, &instance); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.ensureOdfConsoleConfigMapAndService(); err != nil {
		logger.Error(err, "Could not ensure configmap and service for odf-console deployment")
		return ctrl.Result{}, err
	}

	csvList, err := util.GetNamespaceCSVs(ctx, r.Client, r.OperatorNamespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Check if it is upgrade from 4.17 to 4.18
	// The new CSVs won't exists while upgrading
	// They will exists only after new operator has created a new subscription
	if !util.AreMultipleOdfOperatorCsvsPresent(csvList) {
		if err := util.ValidateCSVsPresent(csvList, EssentialCSVs...); err != nil {
			logger.Error(err, "Could not ensure CSVs presence")
			return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 2}, nil
		}
	}

	if err := r.ensureConsolePluginAndCLIDownload(instance.Status.Desired.Version); err != nil {
		logger.Error(err, "Could not ensure compatibility for ODF consolePlugin")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	err := mgr.Add(manager.RunnableFunc(func(context.Context) error {
		return r.ensureOdfConsoleConfigMapAndService()
	}))
	if err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&configv1.ClusterVersion{}).
		Complete(r)
}

func (r *ClusterVersionReconciler) ensureOdfConsoleConfigMapAndService() error {
	logger := log.FromContext(context.TODO())
	nginxConf := console.NginxConf

	// Get ODF console Deployment
	odfConsoleDeployment := console.GetDeployment(OperatorNamespace)
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      odfConsoleDeployment.Name,
		Namespace: odfConsoleDeployment.Namespace,
	}, odfConsoleDeployment)
	if err != nil {
		return err
	}

	// Create/Update ODF console ConfigMap (nginx configuration)
	odfConsoleConfigMap := console.GetNginxConfConfigMap(OperatorNamespace)
	_, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, odfConsoleConfigMap, func() error {
		if odfConsoleConfigMapData := odfConsoleConfigMap.Data["nginx.conf"]; odfConsoleConfigMapData != nginxConf {
			logger.Info(fmt.Sprintf("Set the ConfigMap odf-console-nginx-conf data as '%s'", nginxConf))
			odfConsoleConfigMap.Data["nginx.conf"] = nginxConf
		}
		return controllerutil.SetControllerReference(odfConsoleDeployment, odfConsoleConfigMap, r.Scheme)
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Create/Update ODF console Service
	odfConsoleService := console.GetService(r.ConsolePort, OperatorNamespace)
	_, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, odfConsoleService, func() error {
		return controllerutil.SetControllerReference(odfConsoleDeployment, odfConsoleService, r.Scheme)
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func (r *ClusterVersionReconciler) ensureConsolePluginAndCLIDownload(clusterVersion string) error {
	logger := log.FromContext(context.TODO())
	// The base path to where the request are sent
	basePath := console.GetBasePath(clusterVersion)
	// Customer portal link (CLI Tool download)
	portalLink := console.CUSTOMER_PORTAL_LINK

	// Create/Update ODF console ConsolePlugin
	odfConsolePlugin := console.GetConsolePluginCR(r.ConsolePort, OperatorNamespace)
	_, err := controllerutil.CreateOrUpdate(context.TODO(), r.Client, odfConsolePlugin, func() error {
		if odfConsolePlugin.Spec.Backend.Service != nil {
			if currentBasePath := odfConsolePlugin.Spec.Backend.Service.BasePath; currentBasePath != basePath {
				logger.Info(fmt.Sprintf("Set the BasePath for odf-console plugin as '%s'", basePath))
				odfConsolePlugin.Spec.Backend.Service.BasePath = basePath
			}
		}
		if odfConsolePlugin.Spec.Proxy == nil {
			odfConsolePlugin.Spec.Proxy = console.GetConsolePluginProxy(OperatorNamespace)
		}
		return nil
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// Create/Update ConsoleCLIDownload (CLI Tool download)
	consoleCLIDownload := console.GetConsoleCLIDownloadCR()
	_, err = controllerutil.CreateOrUpdate(context.TODO(), r.Client, consoleCLIDownload, func() error {
		if currentPortalLink := consoleCLIDownload.Spec.Links[0].Href; currentPortalLink != portalLink {
			logger.Info(fmt.Sprintf("Set the customer portal link for CLI Tool '%s'", portalLink))
			consoleCLIDownload.Spec.Links[0].Href = portalLink
		}
		if len(consoleCLIDownload.Spec.Links) != 1 {
			consoleCLIDownload.Spec.Links = console.GetConsoleCLIDownloadLinks()
		}
		return nil
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}
