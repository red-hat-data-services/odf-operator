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

package main

import (
	"flag"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorv2 "github.com/operator-framework/api/pkg/operators/v2"

	ibmv1alpha1 "github.com/IBM/ibm-storage-odf-operator/api/v1alpha1"
	ocsv1 "github.com/red-hat-storage/ocs-operator/api/v4/v1"

	odfv1alpha1 "github.com/red-hat-storage/odf-operator/api/v1alpha1"
	"github.com/red-hat-storage/odf-operator/controllers"
	"github.com/red-hat-storage/odf-operator/pkg/util"

	//+kubebuilder:scaffold:imports
	configv1 "github.com/openshift/api/config/v1"
	consolev1 "github.com/openshift/api/console/v1"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metrics "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(odfv1alpha1.AddToScheme(scheme))
	utilruntime.Must(ocsv1.AddToScheme(scheme))
	utilruntime.Must(ibmv1alpha1.AddToScheme(scheme))

	utilruntime.Must(operatorv1alpha1.AddToScheme(scheme))
	utilruntime.Must(operatorv2.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	utilruntime.Must(consolev1.AddToScheme(scheme))
	utilruntime.Must(extv1.AddToScheme(scheme))
	utilruntime.Must(configv1.AddToScheme(scheme))

}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var odfConsolePort int
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8085", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8082", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.IntVar(&odfConsolePort, "odf-console-port", 9001, "The port where the ODF console server will be serving it's payload")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	operatorNamespace, err := util.GetOperatorNamespace()
	if err != nil {
		setupLog.Error(err, "unable to get operator namespace")
		os.Exit(1)
	}

	defaultNamespaces := map[string]cache.Config{
		operatorNamespace:            {},
		"openshift-storage-extended": {},
	}

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metrics.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          enableLeaderElection,
		LeaderElectionID:        "4fd470de.openshift.io",
		LeaderElectionNamespace: operatorNamespace,
		Cache:                   cache.Options{DefaultNamespaces: defaultNamespaces},
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	storageSystemReconciler := &controllers.StorageSystemReconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName("controllers").WithName("StorageSystem"),
		Scheme:   mgr.GetScheme(),
		Recorder: controllers.NewEventReporter(mgr.GetEventRecorderFor("StorageSystem controller")),
	}
	if err = storageSystemReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "StorageSystem")
		os.Exit(1)
	}

	conditionName, err := util.GetConditionName(mgr.GetClient())
	if err != nil {
		setupLog.Error(err, "unable to get condition name")
		os.Exit(1)
	}
	condition, err := util.NewUpgradeableCondition(mgr.GetClient())
	if err != nil {
		setupLog.Error(err, "unable to get OperatorCondition")
		os.Exit(1)
	}
	subscriptionReconciler := &controllers.SubscriptionReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		ConditionName:     conditionName,
		OperatorCondition: condition,
	}
	if err = subscriptionReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Subscription")
		os.Exit(1)
	}

	storageClusterReconciler := &controllers.StorageClusterReconciler{
		Client:   mgr.GetClient(),
		Scheme:   mgr.GetScheme(),
		Recorder: controllers.NewEventReporter(mgr.GetEventRecorderFor("StorageCluster controller")),
	}
	if err = storageClusterReconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "StorageCluster")
		os.Exit(1)
	}

	if err = (&controllers.ClusterVersionReconciler{
		Client:      mgr.GetClient(),
		Scheme:      mgr.GetScheme(),
		ConsolePort: int32(odfConsolePort), //nolint:gosec
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ClusterVersion")
		os.Exit(1)
	}

	if err = (&controllers.ScalerReconciler{
		Client:            mgr.GetClient(),
		Scheme:            mgr.GetScheme(),
		OperatorNamespace: operatorNamespace,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Scaler")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}

	// console plugin is dependent on CRDs that are part of these CSVs and we
	// can't check for only odf-dependencies CSV as OLM doesn't guarantee
	// (based on observations) existence of all CRDs brought by OLM dependency
	// mechanism.
	csvsToBeSuceeded := []string{
		controllers.OcsSubscriptionStartingCSV,
		controllers.RookSubscriptionStartingCSV,
		controllers.NoobaaSubscriptionStartingCSV,
	}
	if err := mgr.AddReadyzCheck(
		"readyz",
		util.CheckCSVPhase(
			mgr.GetClient(),
			operatorNamespace,
			csvsToBeSuceeded...,
		),
	); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
