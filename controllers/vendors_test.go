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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	conditionsv1 "github.com/openshift/custom-resource-status/conditions/v1"
	ocsv1 "github.com/red-hat-storage/ocs-operator/api/v4/v1"
	odfv1alpha1 "github.com/red-hat-storage/odf-operator/api/v1alpha1"
)

func TestIsVendorSystemPresent(t *testing.T) {

	testCases := []struct {
		label             string
		hasBackEndStorage bool
		expectedRequeue   bool
		expectedError     bool
	}{

		{
			label:             "ensure ResourcePresent condition is true",
			hasBackEndStorage: true,
			expectedError:     false,
		},
		{
			label:             "ensure ResourcePresent condition is false",
			hasBackEndStorage: false,
			expectedError:     true,
		},
	}

	for i, tc := range testCases {
		t.Logf("Case %d: %s\n", i+1, tc.label)

		fakeStorageSystem := GetFakeStorageSystem(StorageClusterKind)
		fakeReconciler := GetFakeStorageSystemReconciler(t, fakeStorageSystem)

		if tc.hasBackEndStorage {
			storageCluster := &ocsv1.StorageCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fakeStorageSystem.Spec.Name,
					Namespace: fakeStorageSystem.Spec.Namespace,
				},
			}
			err := fakeReconciler.Client.Create(context.TODO(), storageCluster)
			assert.NoError(t, err)
		}

		err := fakeReconciler.isVendorSystemPresent(fakeStorageSystem, fakeLogger)

		assert.Equal(t, tc.hasBackEndStorage,
			conditionsv1.IsStatusConditionTrue(
				fakeStorageSystem.Status.Conditions, odfv1alpha1.ConditionVendorSystemPresent))

		if tc.expectedError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
		}
	}
}
