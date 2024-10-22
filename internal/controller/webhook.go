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
	admrv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	odfwebhook "github.com/red-hat-storage/odf-operator/internal/webhook"
)

const (
	WebhookServiceName = "odf-operator-webhook-service"
	WebhookName        = "csv.odf.openshift.io"
)

func (r *StorageSystemReconciler) reconcileSubscriptionValidatingWebhook() error {

	csvWebhook := admrv1.MutatingWebhook{
		Name: WebhookName,
		ClientConfig: admrv1.WebhookClientConfig{
			Service: &admrv1.ServiceReference{
				Name: WebhookServiceName,
				Path: ptr.To(odfwebhook.WebhookPath),
				Port: ptr.To(int32(443)),
			},
		},
		Rules: []admrv1.RuleWithOperations{
			{
				Rule: admrv1.Rule{
					APIGroups:   []string{"operators.coreos.com"},
					APIVersions: []string{"v1alpha1"},
					Resources:   []string{"clusterserviceversions"},
					Scope:       ptr.To(admrv1.NamespacedScope),
				},
				Operations: []admrv1.OperationType{admrv1.Create},
			},
		},
		SideEffects:             ptr.To(admrv1.SideEffectClassNone),
		TimeoutSeconds:          ptr.To(int32(30)),
		AdmissionReviewVersions: []string{"v1"},
		// fail the validation if webhook can't be reached
		FailurePolicy: ptr.To(admrv1.Fail),
	}

	whConfig := &admrv1.MutatingWebhookConfiguration{}
	whConfig.Name = csvWebhook.Name

	res, err := controllerutil.CreateOrUpdate(r.context, r.Client, whConfig, func() error {

		// openshift fills in the ca on finding this annotation
		whConfig.Annotations = map[string]string{
			"service.beta.openshift.io/inject-cabundle": "true",
		}

		var caBundle []byte
		if len(whConfig.Webhooks) == 0 {
			whConfig.Webhooks = make([]admrv1.MutatingWebhook, 1)
		} else {
			// do not mutate CA bundle that was injected by openshift
			caBundle = whConfig.Webhooks[0].ClientConfig.CABundle
		}

		// webhook desired state
		var wh *admrv1.MutatingWebhook = &whConfig.Webhooks[0]
		csvWebhook.DeepCopyInto(wh)

		wh.Name = whConfig.Name
		// only send requests received from own namespace
		wh.NamespaceSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"kubernetes.io/metadata.name": r.OperatorNamespace,
			},
		}
		// preserve the existing (injected) CA bundle if any
		wh.ClientConfig.CABundle = caBundle
		// send request to the service running in own namespace
		wh.ClientConfig.Service.Namespace = r.OperatorNamespace

		return nil
	})
	if err != nil {
		return err
	}

	r.logger.Info("successfully created or updated webhook", "operation", res, "name", whConfig.Name)
	return nil
}

func (r *StorageSystemReconciler) reconcileWebhookService() error {

	webhookService := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebhookServiceName,
			Namespace: r.OperatorNamespace,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Port:       443,
					Protocol:   corev1.ProtocolTCP,
					TargetPort: intstr.FromInt32(9443),
				},
			},
			Selector: map[string]string{
				"app.kubernetes.io/name": "odf-operator-webhook",
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	svc := &corev1.Service{
		ObjectMeta: webhookService.ObjectMeta,
	}
	res, err := controllerutil.CreateOrUpdate(r.context, r.Client, svc, func() error {
		if svc.Annotations == nil {
			svc.Annotations = map[string]string{}
		}
		svc.Annotations["service.beta.openshift.io/serving-cert-secret-name"] = "webhook-server-cert"
		webhookService.Spec.DeepCopyInto(&svc.Spec)
		return nil
	})
	if err != nil {
		return err
	}

	r.logger.Info("successfully created or updated webhook service", "operation", res, "name", svc.Name)
	return nil
}
