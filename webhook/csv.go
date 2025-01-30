package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-logr/logr"
	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	WebhookPath = "/mutate-operators-coreos-com-v1alpha1-clusterserviceversion"
)

var (
	ManagedPkgNames = []string{
		// base csvs
		"ocs-operator",
		"rook-ceph-operator",
		"ocs-client-operator",
		"mcg-operator",
		"noobaa-operator",
		"odf-csi-addons-operator",
		"csi-addons",
		"cephcsi-operator",
		"odf-prometheus-operator",
		"recipe",
	}
)

type ClusterServiceVersionDefaulter struct {
	client.Client
	Decoder           admission.Decoder
	context           context.Context
	logger            logr.Logger
	OperatorNamespace string
}

// +kubebuilder:rbac:groups=operators.coreos.com,resources=clusterserviceversions,verbs=create;update;patch

func (r *ClusterServiceVersionDefaulter) Handle(ctx context.Context, req admission.Request) admission.Response {
	r.context = ctx
	r.logger = log.FromContext(r.context)
	r.logger.Info("Request received for ClusterServiceVersion review")

	instance := &opv1a1.ClusterServiceVersion{}
	err := r.Decoder.Decode(req, instance)
	if err != nil {
		r.logger.Error(err, "failed to decode admission review as csv")
		return admission.Errored(http.StatusBadRequest, fmt.Errorf("failed to decode admission review as csv: %v", err))
	}

	r.Default(instance)

	marshaledInstance, err := json.Marshal(instance)
	if err != nil {
		r.logger.Error(err, "failed to marshal csv")
		return admission.Errored(http.StatusInternalServerError, fmt.Errorf("failed to marshal csv: %v", err))
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledInstance)
}

func (r *ClusterServiceVersionDefaulter) Default(instance *opv1a1.ClusterServiceVersion) {

	ok := r.isOdfManagedCSV(instance)
	if !ok {
		r.logger.Info("Ignoring requested ClusterServiceVersion as it is not relevant")
		return
	}

	r.logger.Info("Mutating requested ClusterServiceVersion")
	for i := range instance.Spec.InstallStrategy.StrategySpec.DeploymentSpecs {
		instance.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[i].Spec.Replicas = ptr.To(int32(0))
	}
}

func (r *ClusterServiceVersionDefaulter) isOdfManagedCSV(instance *opv1a1.ClusterServiceVersion) bool {

	if instance.Namespace != r.OperatorNamespace {
		return false
	}

	for _, pkgName := range ManagedPkgNames {
		if strings.HasPrefix(instance.ObjectMeta.Name, pkgName) {
			return true
		}
	}

	return false
}

func (r *ClusterServiceVersionDefaulter) SetupWebhookWithManager(mgr ctrl.Manager) error {

	mgr.GetWebhookServer().Register(WebhookPath, &webhook.Admission{Handler: r})

	return nil
}
