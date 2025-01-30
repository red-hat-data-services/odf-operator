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
	"os"
	"slices"
	"strings"

	"go.uber.org/multierr"
	admv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	ocsv1 "github.com/red-hat-storage/ocs-operator/api/v4/v1"
	odfv1alpha1 "github.com/red-hat-storage/odf-operator/api/v1alpha1"
	"github.com/red-hat-storage/odf-operator/pkg/util"
)

// CheckForExistingSubscription looks for any existing Subscriptions that
// reference the given package. If one does exist, use its ObjectMeta for the
// desiredSubscription.
//
// NOTE(jarrpa): We can't use client.MatchingFields to limit the list results
// because fake.Client does not support them.
func CheckExistingSubscriptions(cli client.Client, desiredSubscription *operatorv1alpha1.Subscription) (*operatorv1alpha1.Subscription, error) {

	odfSub, err := GetOdfSubscription(cli)
	if err != nil {
		return nil, err
	}

	if odfSub.Spec.Config == nil {
		odfSub.Spec.Config = &operatorv1alpha1.SubscriptionConfig{}
	}

	var isProvider bool
	if desiredSubscription.Spec.Package == OcsClientSubscriptionPackage ||
		desiredSubscription.Spec.Package == CSIAddonsSubscriptionPackage ||
		desiredSubscription.Spec.Package == CephCSISubscriptionPackage {

		isProvider, err = isProviderMode(cli)
		if err != nil {
			return nil, err
		}
	}

	subsList := &operatorv1alpha1.SubscriptionList{}
	err = cli.List(context.TODO(), subsList, &client.ListOptions{Namespace: desiredSubscription.Namespace})
	if err != nil {
		return nil, err
	}

	var subExsist bool
	var actualSub *operatorv1alpha1.Subscription
	pkgName := desiredSubscription.Spec.Package
	for i, sub := range subsList.Items {
		if sub.Spec.Package == pkgName {
			subExsist = true
			if actualSub != nil {
				foundSubs := []string{actualSub.Name, sub.Name}
				return nil, fmt.Errorf("multiple Subscriptions found for package '%s': %v", pkgName, foundSubs)
			}
			actualSub = &subsList.Items[i]

			if !isProvider {
				actualSub.Spec.Channel = desiredSubscription.Spec.Channel
			}

			if actualSub.Spec.Config == nil && desiredSubscription.Spec.Config == nil {
				actualSub.Spec.Config = &operatorv1alpha1.SubscriptionConfig{}
				actualSub.Spec.Config.Tolerations = odfSub.Spec.Config.Tolerations
			} else if actualSub.Spec.Config == nil && desiredSubscription.Spec.Config != nil {
				actualSub.Spec.Config = desiredSubscription.Spec.Config
				actualSub.Spec.Config.Tolerations = getMergedTolerations(odfSub.Spec.Config.Tolerations, desiredSubscription.Spec.Config.Tolerations)
			} else if actualSub.Spec.Config != nil && desiredSubscription.Spec.Config == nil {
				actualSub.Spec.Config.Tolerations = odfSub.Spec.Config.Tolerations
			} else if actualSub.Spec.Config != nil && desiredSubscription.Spec.Config != nil {
				// Combines the environment variables from both subscriptions.
				actualSub.Spec.Config.Env = getMergedEnvVars(actualSub.Spec.Config.Env, desiredSubscription.Spec.Config.Env)
				// Combines the Tolerations from odf sub and desired sub.
				actualSub.Spec.Config.Tolerations = getMergedTolerations(odfSub.Spec.Config.Tolerations, desiredSubscription.Spec.Config.Tolerations)
			}

			desiredSubscription = actualSub
		}
	}

	if !subExsist {
		// Set the catalog source for the odf-dependencies subscription to match that of the odf-operator subscription
		// This ensures that the odf-dependencies subscription uses the same catalog source across all environments,
		// including offline and test environments where the catalog name may vary.
		if desiredSubscription.Spec.Package == OdfDepsSubscriptionPackage {
			desiredSubscription.Spec.CatalogSource = odfSub.Spec.CatalogSource
			desiredSubscription.Spec.CatalogSourceNamespace = odfSub.Spec.CatalogSourceNamespace
		}

		if desiredSubscription.Spec.Config == nil {
			desiredSubscription.Spec.Config = &operatorv1alpha1.SubscriptionConfig{
				Tolerations: odfSub.Spec.Config.Tolerations,
			}
		} else {
			desiredSubscription.Spec.Config.Tolerations = getMergedTolerations(odfSub.Spec.Config.Tolerations, desiredSubscription.Spec.Config.Tolerations)
		}
	}

	return desiredSubscription, nil
}

func isProviderMode(cli client.Client) (bool, error) {

	storageclusters := &ocsv1.StorageClusterList{}
	err := cli.List(context.TODO(), storageclusters)
	if err != nil {
		if meta.IsNoMatchError(err) {
			return false, nil
		}
		return false, err
	}

	for _, storagecluster := range storageclusters.Items {
		if storagecluster.Spec.AllowRemoteStorageConsumers {
			return true, nil
		}
	}

	return false, nil
}

func getMergedTolerations(tol1, tol2 []corev1.Toleration) []corev1.Toleration {

	if len(tol1) == 0 {
		return append([]corev1.Toleration{}, tol2...)
	} else if len(tol2) == 0 {
		return append([]corev1.Toleration{}, tol1...)
	}

	mergedTolerations := append([]corev1.Toleration{}, tol1...)

	for _, t2 := range tol2 {
		found := false
		for i, t1 := range tol1 {
			if t1.Key == t2.Key && t1.Operator == t2.Operator && t1.Value == t2.Value {
				found = true
				break
			}
			// If the toleration with the same key but different values is found,
			// update the existing toleration in tol1 with the new toleration from tol2.
			if t1.Key == t2.Key && t1.Operator == t2.Operator {
				tol1[i] = t2
				found = true
				break
			}
		}
		if !found {
			mergedTolerations = append(mergedTolerations, t2)
		}
	}

	return mergedTolerations
}

// getMergedEnvVars updates the value of env variables in the envList1 with that of envList2 and
// returns the updated list of env variables.
func getMergedEnvVars(envList1, envList2 []corev1.EnvVar) []corev1.EnvVar {
	envMap := make(map[string]string)

	for _, env := range envList1 {
		envMap[env.Name] = env.Value
	}

	for _, env := range envList2 {
		envMap[env.Name] = env.Value
	}

	// Convert the map back to a slice
	var updatedEnvVars []corev1.EnvVar
	for key, value := range envMap {
		updatedEnvVars = append(updatedEnvVars, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	return updatedEnvVars
}

func EnsureDesiredSubscription(cli client.Client, desiredSubscription *operatorv1alpha1.Subscription) error {

	var err error

	desiredSubscription, err = CheckExistingSubscriptions(cli, desiredSubscription)
	if err != nil {
		return err
	}

	// Skip creating (only update) subscriptions other than odf-dependencies
	// It will allow OLM to manage their creation via dependency resolution
	if desiredSubscription.Spec.Package != OdfDepsSubscriptionPackage && desiredSubscription.CreationTimestamp.IsZero() {
		return nil
	}

	// create/update subscription
	sub := &operatorv1alpha1.Subscription{}
	sub.ObjectMeta = desiredSubscription.ObjectMeta
	_, err = controllerutil.CreateOrUpdate(context.TODO(), cli, sub, func() error {
		sub.Spec = desiredSubscription.Spec
		return SetOdfSubControllerReference(cli, sub)
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	return nil
}

func SetOdfSubControllerReference(cli client.Client, obj client.Object) error {

	odfSub, err := GetOdfSubscription(cli)
	if err != nil {
		return err
	}

	err = controllerutil.SetControllerReference(odfSub, obj, cli.Scheme())
	if err != nil {
		return err
	}

	return nil
}

// GetOdfSubscription returns the subscription for odf-operator.
func GetOdfSubscription(cli client.Client) (*operatorv1alpha1.Subscription, error) {

	subsList := &operatorv1alpha1.SubscriptionList{}
	err := cli.List(context.TODO(), subsList, &client.ListOptions{Namespace: OperatorNamespace})
	if err != nil {
		return nil, err
	}

	for _, sub := range subsList.Items {
		if sub.Spec.Package == OdfSubscriptionPackage {
			return &sub, nil
		}
	}

	return nil, fmt.Errorf("odf-operator subscription not found")
}

func GetVendorCsvNames(cli client.Client, kind odfv1alpha1.StorageKind) ([]string, error) {

	var csvNames []string
	var err error
	var isProvider bool

	if kind == VendorFlashSystemCluster() {
		csvNames = []string{IbmSubscriptionStartingCSV}
	} else if kind == VendorStorageCluster() {
		csvNames = []string{OdfDepsSubscriptionStartingCSV, OcsSubscriptionStartingCSV, RookSubscriptionStartingCSV, NoobaaSubscriptionStartingCSV,
			PrometheusSubscriptionStartingCSV, RecipeSubscriptionStartingCSV}

		isProvider, err = isProviderMode(cli)
		if err != nil {
			return csvNames, err
		}

		if !isProvider {
			csvNames = append(csvNames, OcsClientSubscriptionStartingCSV, CSIAddonsSubscriptionStartingCSV, CephCSISubscriptionStartingCSV)
		}

		// In provider mode, upgrades of the ocs-client-operator and csiaddons are managed by the provider, not the odf-operator.
		// This can result in these operators lagging behind the odf-operator, with different CSV versions.
		// Therefore, we need to fetch the CSV name from the operator deployment.
		// We are only fetching the ocs-client-operator CSV name and ignoring the csiaddons
		// Because the ocs-client-operator is essential when enabling provider mode as we need to bring it up.

		// Fetch the CSV name from the client operator deployment and append it to the csv list
		if isProvider {
			// get ocs-client-operator deployment with label
			deployments := &appsv1.DeploymentList{}
			err = cli.List(context.TODO(), deployments, &client.ListOptions{
				Namespace:     OperatorNamespace,
				LabelSelector: labels.SelectorFromSet(map[string]string{"app": "ocs-client-operator"}),
			})

			if err != nil {
				return csvNames, err
			}

			if len(deployments.Items) == 0 {
				return csvNames, fmt.Errorf("ocs-client-operator deployment not found")
			}

			// get owner ref index from deployment
			ownerRefIndex := slices.IndexFunc(deployments.Items[0].OwnerReferences, func(o metav1.OwnerReference) bool {
				return o.Kind == "ClusterServiceVersion"
			})
			if ownerRefIndex == -1 {
				return csvNames, fmt.Errorf("ClusterServiceVersion owner reference not found in ocs-client-operator deployment")
			}

			ownerRef := deployments.Items[0].OwnerReferences[ownerRefIndex]

			// get csv name from owner ref
			csvNames = append(csvNames, ownerRef.Name)
		}
	}

	return csvNames, err
}

func EnsureVendorCsv(cli client.Client, csvName string) (*operatorv1alpha1.ClusterServiceVersion, error) {

	var err error

	csvObj := &operatorv1alpha1.ClusterServiceVersion{}
	err = cli.Get(context.TODO(), types.NamespacedName{
		Name: csvName, Namespace: OperatorNamespace}, csvObj)
	if err != nil {
		if errors.IsNotFound(err) {
			approvalErr := ApproveInstallPlanForCsv(cli, csvName)
			if approvalErr != nil {
				return nil, approvalErr
			}
		}
		return nil, err
	}
	_, err = controllerutil.CreateOrUpdate(context.TODO(), cli, csvObj, func() error {
		csvObj.OwnerReferences = []metav1.OwnerReference{}

		// Shut down the OCS client operator CSV pods in non provider mode
		if strings.HasPrefix(csvName, "ocs-client-operator") {
			isProvider, err := isProviderMode(cli)
			if err != nil {
				return err
			}

			var replicas int32 = 0
			if isProvider {
				replicas = 1
			}

			for i := range csvObj.Spec.InstallStrategy.StrategySpec.DeploymentSpecs {
				csvObj.Spec.InstallStrategy.StrategySpec.DeploymentSpecs[i].Spec.Replicas = &replicas
			}

			if replicas == 0 {
				// delete the subscription webhook created by the ocs-client-operator
				// we can not delete the webhook by the ocs-client-operator itself because the client operator is down
				webhook := &admv1.ValidatingWebhookConfiguration{}
				webhook.Name = "subscription.ocs.openshift.io"
				if err = cli.Delete(context.TODO(), webhook); err != nil && !errors.IsNotFound(err) {
					return err
				}
			}
		}

		return SetOdfSubControllerReference(cli, csvObj)
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}

	isReady := csvObj.Status.Phase == operatorv1alpha1.CSVPhaseSucceeded &&
		csvObj.Status.Reason == operatorv1alpha1.CSVReasonInstallSuccessful

	if !isReady {
		err = fmt.Errorf("CSV is not successfully installed")
		return nil, err
	}

	return csvObj, err
}

// ApproveInstallPlanForCsv approve the manual approval installPlan for the given CSV
// and returns an error if none found
func ApproveInstallPlanForCsv(cli client.Client, csvName string) error {

	var finalError error
	var foundInstallPlan bool

	installPlans := &operatorv1alpha1.InstallPlanList{}
	err := cli.List(context.TODO(), installPlans, &client.ListOptions{Namespace: OperatorNamespace})

	if err != nil {
		return err
	}

	for i, installPlan := range installPlans.Items {
		if util.FindInSlice(installPlan.Spec.ClusterServiceVersionNames, csvName) {
			foundInstallPlan = true
			if installPlan.Spec.Approval == operatorv1alpha1.ApprovalManual &&
				!installPlan.Spec.Approved {

				installPlans.Items[i].Spec.Approved = true
				err = cli.Update(context.TODO(), &installPlans.Items[i])
				if err != nil {
					multierr.AppendInto(&finalError, fmt.Errorf(
						"Failed to approve installplan %s", installPlan.Name))
					multierr.AppendInto(&finalError, err)
				}
			}
		}
	}

	if !foundInstallPlan {
		err = fmt.Errorf("InstallPlan not found for CSV %s", csvName)
		multierr.AppendInto(&finalError, err)
	}

	return finalError
}

// GetSubscriptions returns all required Subscriptions for the given StorageKind
func GetSubscriptions(k odfv1alpha1.StorageKind) []*operatorv1alpha1.Subscription {

	subscriptions := []*operatorv1alpha1.Subscription{}
	if k == StorageClusterKind {
		subscriptions = GetStorageClusterSubscriptions()
	} else if k == FlashSystemKind {
		subscriptions = GetFlashSystemClusterSubscriptions()
	}

	return subscriptions
}

// GetStorageClusterSubscription return subscription for StorageCluster
func GetStorageClusterSubscriptions() []*operatorv1alpha1.Subscription {

	odfDepsSubscription := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OdfDepsSubscriptionName,
			Namespace: OperatorNamespace,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			CatalogSource:          OdfDepsSubscriptionCatalogSource,
			CatalogSourceNamespace: OdfDepsSubscriptionCatalogSourceNamespace,
			Package:                OdfDepsSubscriptionPackage,
			Channel:                OdfDepsSubscriptionChannel,
			StartingCSV:            OdfDepsSubscriptionStartingCSV,
			InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
		},
	}

	noobaaSubscription := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      NoobaaSubscriptionName,
			Namespace: OperatorNamespace,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			CatalogSource:          NoobaaSubscriptionCatalogSource,
			CatalogSourceNamespace: NoobaaSubscriptionCatalogSourceNamespace,
			Package:                NoobaaSubscriptionPackage,
			Channel:                NoobaaSubscriptionChannel,
			StartingCSV:            NoobaaSubscriptionStartingCSV,
			InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
		},
	}

	roleARN := os.Getenv("ROLEARN")
	if roleARN != "" {
		noobaaSubscription.Spec.Config = &operatorv1alpha1.SubscriptionConfig{
			Env: []corev1.EnvVar{
				{
					Name:  "ROLEARN",
					Value: roleARN,
				},
			},
		}
	}

	ocsSubscription := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OcsSubscriptionName,
			Namespace: OperatorNamespace,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			CatalogSource:          OcsSubscriptionCatalogSource,
			CatalogSourceNamespace: OcsSubscriptionCatalogSourceNamespace,
			Package:                OcsSubscriptionPackage,
			Channel:                OcsSubscriptionChannel,
			StartingCSV:            OcsSubscriptionStartingCSV,
			InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
		},
	}

	ocsClientSubscription := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OcsClientSubscriptionName,
			Namespace: OperatorNamespace,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			CatalogSource:          OcsClientSubscriptionCatalogSource,
			CatalogSourceNamespace: OcsClientSubscriptionCatalogSourceNamespace,
			Package:                OcsClientSubscriptionPackage,
			Channel:                OcsClientSubscriptionChannel,
			StartingCSV:            OcsClientSubscriptionStartingCSV,
			InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
		},
	}

	csiAddonsSubscription := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CSIAddonsSubscriptionName,
			Namespace: OperatorNamespace,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			CatalogSource:          CSIAddonsSubscriptionCatalogSource,
			CatalogSourceNamespace: CSIAddonsSubscriptionCatalogSourceNamespace,
			Package:                CSIAddonsSubscriptionPackage,
			Channel:                CSIAddonsSubscriptionChannel,
			StartingCSV:            CSIAddonsSubscriptionStartingCSV,
			InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
			Config: &operatorv1alpha1.SubscriptionConfig{
				Tolerations: []corev1.Toleration{
					{
						Key:      "node.ocs.openshift.io/storage",
						Operator: "Equal",
						Value:    "true",
						Effect:   "NoSchedule",
					},
				},
			},
		},
	}

	cephCsiSubscription := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      CephCSISubscriptionName,
			Namespace: OperatorNamespace,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			CatalogSource:          CephCSISubscriptionCatalogSource,
			CatalogSourceNamespace: CephCSISubscriptionCatalogSourceNamespace,
			Package:                CephCSISubscriptionPackage,
			Channel:                CephCSISubscriptionChannel,
			StartingCSV:            CephCSISubscriptionStartingCSV,
			InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
			Config: &operatorv1alpha1.SubscriptionConfig{
				Tolerations: []corev1.Toleration{
					{
						Key:      "node.ocs.openshift.io/storage",
						Operator: "Equal",
						Value:    "true",
						Effect:   "NoSchedule",
					},
				},
			},
		},
	}

	rookSubscription := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RookSubscriptionName,
			Namespace: OperatorNamespace,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			CatalogSource:          RookSubscriptionCatalogSource,
			CatalogSourceNamespace: RookSubscriptionCatalogSourceNamespace,
			Package:                RookSubscriptionPackage,
			Channel:                RookSubscriptionChannel,
			StartingCSV:            RookSubscriptionStartingCSV,
			InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
		},
	}

	prometheusSubscription := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PrometheusSubscriptionName,
			Namespace: OperatorNamespace,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			CatalogSource:          PrometheusSubscriptionCatalogSource,
			CatalogSourceNamespace: PrometheusSubscriptionCatalogSourceNamespace,
			Package:                PrometheusSubscriptionPackage,
			Channel:                PrometheusSubscriptionChannel,
			StartingCSV:            PrometheusSubscriptionStartingCSV,
			InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
		},
	}

	recipeSubscription := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RecipeSubscriptionName,
			Namespace: OperatorNamespace,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			CatalogSource:          RecipeSubscriptionCatalogSource,
			CatalogSourceNamespace: RecipeSubscriptionCatalogSourceNamespace,
			Package:                RecipeSubscriptionPackage,
			Channel:                RecipeSubscriptionChannel,
			StartingCSV:            RecipeSubscriptionStartingCSV,
			InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
		},
	}

	// Do not change the inxex of odfDepsSubscription. The 0 index is being used to create this subscription
	// while starting in the subscription controller in SetupWithManager.
	return []*operatorv1alpha1.Subscription{odfDepsSubscription, ocsSubscription, rookSubscription, noobaaSubscription,
		csiAddonsSubscription, cephCsiSubscription, ocsClientSubscription, prometheusSubscription, recipeSubscription}
}

// GetFlashSystemClusterSubscription return subscription for FlashSystemCluster
func GetFlashSystemClusterSubscriptions() []*operatorv1alpha1.Subscription {
	ibmSubscription := &operatorv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      IbmSubscriptionName,
			Namespace: OperatorNamespace,
		},
		Spec: &operatorv1alpha1.SubscriptionSpec{
			CatalogSource:          IbmSubscriptionCatalogSource,
			CatalogSourceNamespace: IbmSubscriptionCatalogSourceNamespace,
			Package:                IbmSubscriptionPackage,
			Channel:                IbmSubscriptionChannel,
			StartingCSV:            IbmSubscriptionStartingCSV,
			InstallPlanApproval:    operatorv1alpha1.ApprovalAutomatic,
		},
	}

	return []*operatorv1alpha1.Subscription{ibmSubscription}
}
