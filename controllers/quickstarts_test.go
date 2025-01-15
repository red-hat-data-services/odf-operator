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
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	consolev1 "github.com/openshift/api/console/v1"
	odfv1alpha1 "github.com/red-hat-storage/odf-operator/api/v1alpha1"
)

var (
	cases = []struct {
		quickstartName string
	}{
		{
			quickstartName: "getting-started-odf",
		},
		{
			quickstartName: "odf-configuration",
		},
	}
)

func TestQuickStartYAMLs(t *testing.T) {
	for _, qs := range AllQuickStarts {
		cqs := consolev1.ConsoleQuickStart{}
		err := yaml.Unmarshal(qs, &cqs)
		assert.NoError(t, err)
	}
}

func TestEnsureQuickStarts(t *testing.T) {
	allExpectedQuickStarts := []consolev1.ConsoleQuickStart{}
	for _, qs := range AllQuickStarts {
		cqs := consolev1.ConsoleQuickStart{}
		err := yaml.Unmarshal(qs, &cqs)
		assert.NoError(t, err)
		allExpectedQuickStarts = append(allExpectedQuickStarts, cqs)
	}

	fakeReconciler := GetFakeStorageSystemReconciler(t)
	err := fakeReconciler.ensureQuickStarts(fakeLogger)
	assert.NoError(t, err)
	for _, c := range cases {
		qs := consolev1.ConsoleQuickStart{}
		err = fakeReconciler.Client.Get(context.TODO(), types.NamespacedName{
			Name: c.quickstartName,
		}, &qs)
		assert.NoError(t, err)
		found := consolev1.ConsoleQuickStart{}
		expected := consolev1.ConsoleQuickStart{}
		for _, cqs := range allExpectedQuickStarts {
			if qs.Name == cqs.Name {
				found = qs
				expected = cqs
				break
			}
		}
		assert.Equal(t, expected.Name, found.Name)
		assert.Equal(t, expected.Namespace, found.Namespace)
		assert.Equal(t, expected.Spec.DurationMinutes, found.Spec.DurationMinutes)
		assert.Equal(t, expected.Spec.Introduction, found.Spec.Introduction)
		assert.Equal(t, expected.Spec.DisplayName, found.Spec.DisplayName)
	}
	assert.Equal(t, len(allExpectedQuickStarts), len(getActualQuickStarts(t, cases, fakeReconciler)))
}

func getActualQuickStarts(t *testing.T, cases []struct {
	quickstartName string
}, reconciler *StorageSystemReconciler) []consolev1.ConsoleQuickStart {
	allActualQuickStarts := []consolev1.ConsoleQuickStart{}
	for _, c := range cases {
		qs := consolev1.ConsoleQuickStart{}
		err := reconciler.Client.Get(context.TODO(), types.NamespacedName{
			Name: c.quickstartName,
		}, &qs)
		if apierrors.IsNotFound(err) {
			continue
		}
		assert.NoError(t, err)
		allActualQuickStarts = append(allActualQuickStarts, qs)
	}
	return allActualQuickStarts
}

func TestDeleteQuickStarts(t *testing.T) {
	fss1 := generateFakeStorageSystem()
	fss2 := generateFakeStorageSystem()

	testCases := []struct {
		label                string
		createStorageSystems []odfv1alpha1.StorageSystem
		deleteStorageSystems []odfv1alpha1.StorageSystem
		expectDeleted        bool
	}{
		{
			label:                "having two storage systems but only deleting one does not delete quickstarts",
			createStorageSystems: []odfv1alpha1.StorageSystem{fss1, fss2},
			deleteStorageSystems: []odfv1alpha1.StorageSystem{fss1},
			expectDeleted:        false,
		},
		{
			label:                "having two storage systems and deleting both deletes the quickstarts",
			createStorageSystems: []odfv1alpha1.StorageSystem{fss1, fss2},
			deleteStorageSystems: []odfv1alpha1.StorageSystem{fss1, fss2},
			expectDeleted:        true,
		},

		{
			label:                "having one storage system and deleting it deletes the quickstarts",
			createStorageSystems: []odfv1alpha1.StorageSystem{fss1},
			deleteStorageSystems: []odfv1alpha1.StorageSystem{fss1},
			expectDeleted:        true,
		},
	}

	for i, tc := range testCases {
		t.Logf("Case %d: %s\n", i+1, tc.label)

		fakeReconciler := GetFakeStorageSystemReconciler(t)

		err := fakeReconciler.ensureQuickStarts(fakeLogger)
		assert.NoError(t, err)

		var quickstarts []consolev1.ConsoleQuickStart = getActualQuickStarts(t, cases, fakeReconciler)
		assert.Equal(t, 2, len(quickstarts))

		for i := range tc.createStorageSystems {
			err = fakeReconciler.Client.Create(context.TODO(), &tc.createStorageSystems[i])
			assert.NoError(t, err)
		}
		for i := range tc.deleteStorageSystems {
			err := fakeReconciler.Client.Delete(context.TODO(), &tc.deleteStorageSystems[i])
			assert.NoError(t, err)
			err = fakeReconciler.deleteResources(&tc.deleteStorageSystems[i], fakeLogger)
			assert.NoError(t, err)
		}
		quickstarts = getActualQuickStarts(t, cases, fakeReconciler)
		if tc.expectDeleted {
			assert.Equal(t, 0, len(quickstarts))
		} else {
			assert.Equal(t, 2, len(quickstarts))
		}
	}

}

// Generate a unique StorageSystem
func generateFakeStorageSystem() odfv1alpha1.StorageSystem {
	return odfv1alpha1.StorageSystem{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateShortRandomString(),
			Namespace: OperatorNamespace,
		},
		Spec: odfv1alpha1.StorageSystemSpec{
			Kind:      VendorStorageCluster(),
			Name:      generateShortRandomString(),
			Namespace: OperatorNamespace,
		},
	}

}

// Not meant to be collison-proof like UUID but good enough for small tests like this.
func generateShortRandomString() string {
	b := make([]byte, 4) //equals 8 characters
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
