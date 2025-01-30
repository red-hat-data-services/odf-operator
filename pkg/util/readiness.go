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

package util

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
)

func CheckCSVPhase(cli client.Client, namespace string, csvNames ...string) healthz.Checker {

	return func(r *http.Request) error {

		csvList, err := GetNamespaceCSVs(r.Context(), cli, namespace)
		if err != nil {
			return err
		}

		// Check if it is upgrade from 4.17 to 4.18
		// The new CSVs won't exists while upgrading
		// They will exists only after new operator has created a new subscription
		if AreMultipleOdfOperatorCsvsPresent(csvList) {
			return nil
		}

		return validateCSVsSucceeded(csvList, csvNames...)
	}
}

func validateCSVsSucceeded(csvList *opv1a1.ClusterServiceVersionList, csvsToBeSucceeded ...string) error {

	for _, csvName := range csvsToBeSucceeded {
		csv, found := getCSVByName(csvList, csvName)
		if !found {
			return fmt.Errorf("CSV %q not found in the list", csvName)
		}

		if csv.Status.Phase != opv1a1.CSVPhaseSucceeded {
			return fmt.Errorf("CSV %q is not in the Succeeded phase; current phase: %s", csv.Name, csv.Status.Phase)
		}
	}
	return nil
}

func ValidateCSVsPresent(csvList *opv1a1.ClusterServiceVersionList, csvsToBePresent ...string) error {

	for _, csvName := range csvsToBePresent {
		_, found := getCSVByName(csvList, csvName)
		if !found {
			return fmt.Errorf("CSV %q not found in the list", csvName)
		}
	}
	return nil
}

func getCSVByName(csvList *opv1a1.ClusterServiceVersionList, name string) (*opv1a1.ClusterServiceVersion, bool) {

	for i := range csvList.Items {
		if csvList.Items[i].Name == name {
			return &csvList.Items[i], true
		}
	}
	return nil, false
}

func GetNamespaceCSVs(ctx context.Context, cli client.Client, namespace string) (*opv1a1.ClusterServiceVersionList, error) {

	csvList := &opv1a1.ClusterServiceVersionList{}
	err := cli.List(ctx, csvList, client.InNamespace(namespace))
	return csvList, err
}

func AreMultipleOdfOperatorCsvsPresent(csvs *opv1a1.ClusterServiceVersionList) bool {

	count := 0

	for _, csv := range csvs.Items {
		if strings.HasPrefix(csv.Name, "odf-operator") {
			count += 1
		}
	}

	return count > 1
}
