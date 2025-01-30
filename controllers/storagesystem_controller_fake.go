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
	"testing"

	"github.com/stretchr/testify/assert"
	extv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	ibmv1alpha1 "github.com/IBM/ibm-storage-odf-operator/api/v1alpha1"
	consolev1 "github.com/openshift/api/console/v1"
	operatorv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	ocsv1 "github.com/red-hat-storage/ocs-operator/api/v4/v1"
	odfv1alpha1 "github.com/red-hat-storage/odf-operator/api/v1alpha1"
)

var (
	fakeLogger = ctrl.Log.WithName("test-controllers").WithName("StorageSystem")
)

func GetFakeStorageSystem(kind odfv1alpha1.StorageKind) *odfv1alpha1.StorageSystem {
	return &odfv1alpha1.StorageSystem{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-storage-system",
			Namespace: OperatorNamespace,
		},
		Spec: odfv1alpha1.StorageSystemSpec{
			Kind:      kind,
			Name:      "fake-vendor-system",
			Namespace: OperatorNamespace,
		},
	}
}

func GetFakeStorageSystemReconciler(t *testing.T, objs ...runtime.Object) *StorageSystemReconciler {

	scheme := createFakeScheme(t)
	fakeStorageSystemReconciler := &StorageSystemReconciler{
		Client:   fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build(),
		Scheme:   scheme,
		Recorder: NewEventReporter(record.NewFakeRecorder(1024)),
	}

	return fakeStorageSystemReconciler
}

func createFakeScheme(t *testing.T) *runtime.Scheme {

	scheme, err := odfv1alpha1.SchemeBuilder.Build()
	if err != nil {
		assert.Fail(t, "unable to build scheme")
	}

	err = consolev1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add consolev1 scheme")
	}

	err = extv1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add extv1 scheme")
	}

	err = operatorv1alpha1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add operatorv1alpha1 scheme")
	}

	err = ocsv1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add ocsv1 scheme")
	}

	err = ibmv1alpha1.AddToScheme(scheme)
	if err != nil {
		assert.Fail(t, "failed to add ibmv1alpha1 scheme")
	}

	return scheme
}

func GetFakeFlashSystemCluster() *ibmv1alpha1.FlashSystemCluster {
	return &ibmv1alpha1.FlashSystemCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-vendor-system",
			Namespace: OperatorNamespace,
		},
		Spec: ibmv1alpha1.FlashSystemClusterSpec{},
	}
}
