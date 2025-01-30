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
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	ibmv1alpha1 "github.com/IBM/ibm-storage-odf-operator/api/v1alpha1"
	ocsv1 "github.com/red-hat-storage/ocs-operator/api/v4/v1"
	odfv1alpha1 "github.com/red-hat-storage/odf-operator/api/v1alpha1"
)

func TestDeleteResources(t *testing.T) {

	var err error

	testCases := []struct {
		label         string
		kind          odfv1alpha1.StorageKind
		resourceExist bool
		expectedError bool
	}{
		{
			label:         "delete StorageCluster",
			kind:          VendorStorageCluster(),
			resourceExist: true,
			expectedError: true,
		},
		{
			label:         "delete FlashSystemCluster",
			kind:          VendorFlashSystemCluster(),
			resourceExist: true,
			expectedError: true,
		},
		{
			label:         "StorageCluster does not exist",
			kind:          VendorStorageCluster(),
			resourceExist: false,
			expectedError: false,
		},
		{
			label:         "FlashSystemCluster does not exist",
			kind:          VendorFlashSystemCluster(),
			resourceExist: false,
			expectedError: false,
		},
	}

	for i, tc := range testCases {
		t.Logf("Case %d: %s\n", i+1, tc.label)

		fakeStorageSystem := GetFakeStorageSystem(tc.kind)
		fakeReconciler := GetFakeStorageSystemReconciler(t, fakeStorageSystem)

		if tc.resourceExist {
			// create resource
			var vendorSystem client.Object
			if tc.kind == VendorStorageCluster() {
				vendorSystem = GetFakeStorageCluster()
			} else if tc.kind == VendorFlashSystemCluster() {
				vendorSystem = GetFakeFlashSystemCluster()
			}

			fakeStorageSystem.Spec.Name = vendorSystem.GetName()
			fakeStorageSystem.Spec.Namespace = vendorSystem.GetNamespace()
			err = fakeReconciler.Client.Create(context.TODO(), vendorSystem)
			assert.NoError(t, err)
		}

		err = fakeReconciler.deleteResources(fakeStorageSystem, fakeLogger)
		assert.True(t, (tc.expectedError == (err != nil)))

		// verify resource does not exist
		if tc.kind == VendorStorageCluster() {
			storageCluster := &ocsv1.StorageCluster{}
			err = fakeReconciler.Client.Get(context.TODO(), types.NamespacedName{
				Name: fakeStorageSystem.Spec.Name, Namespace: fakeStorageSystem.Namespace},
				storageCluster)
			assert.True(t, errors.IsNotFound(err))

		} else if tc.kind == VendorFlashSystemCluster() {
			flashSystemCluster := &ibmv1alpha1.FlashSystemCluster{}
			err = fakeReconciler.Client.Get(context.TODO(), types.NamespacedName{
				Name: fakeStorageSystem.Spec.Name, Namespace: fakeStorageSystem.Namespace},
				flashSystemCluster)
			assert.True(t, errors.IsNotFound(err))

		}
	}
}
