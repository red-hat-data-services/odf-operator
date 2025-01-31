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
	"os"
)

var (
	DefaultValMap = map[string]string{
		"OPERATOR_NAMESPACE": "openshift-storage",

		"ODF_DEPS_SUBSCRIPTION_NAME":                    "odf-dependencies",
		"ODF_DEPS_SUBSCRIPTION_PACKAGE":                 "odf-dependencies",
		"ODF_DEPS_SUBSCRIPTION_CHANNEL":                 "alpha",
		"ODF_DEPS_SUBSCRIPTION_STARTINGCSV":             "odf-dependencies.v4.18.0",
		"ODF_DEPS_SUBSCRIPTION_CATALOGSOURCE":           "odf-catalogsource",
		"ODF_DEPS_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE": "openshift-marketplace",

		"NOOBAA_SUBSCRIPTION_NAME":                    "noobaa-operator",
		"NOOBAA_SUBSCRIPTION_PACKAGE":                 "noobaa-operator",
		"NOOBAA_SUBSCRIPTION_CHANNEL":                 "alpha",
		"NOOBAA_SUBSCRIPTION_STARTINGCSV":             "noobaa-operator.v5.18.0",
		"NOOBAA_SUBSCRIPTION_CATALOGSOURCE":           "odf-catalogsource",
		"NOOBAA_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE": "openshift-marketplace",

		"OCS_SUBSCRIPTION_NAME":                    "ocs-operator",
		"OCS_SUBSCRIPTION_PACKAGE":                 "ocs-operator",
		"OCS_SUBSCRIPTION_CHANNEL":                 "alpha",
		"OCS_SUBSCRIPTION_STARTINGCSV":             "ocs-operator.v4.18.0",
		"OCS_SUBSCRIPTION_CATALOGSOURCE":           "odf-catalogsource",
		"OCS_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE": "openshift-marketplace",

		"OCS_CLIENT_SUBSCRIPTION_NAME":                    "ocs-client-operator",
		"OCS_CLIENT_SUBSCRIPTION_PACKAGE":                 "ocs-client-operator",
		"OCS_CLIENT_SUBSCRIPTION_CHANNEL":                 "alpha",
		"OCS_CLIENT_SUBSCRIPTION_STARTINGCSV":             "ocs-client-operator.v4.18.0",
		"OCS_CLIENT_SUBSCRIPTION_CATALOGSOURCE":           "odf-catalogsource",
		"OCS_CLIENT_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE": "openshift-marketplace",

		"CSIADDONS_SUBSCRIPTION_NAME":                    "csi-addons",
		"CSIADDONS_SUBSCRIPTION_PACKAGE":                 "csi-addons",
		"CSIADDONS_SUBSCRIPTION_CHANNEL":                 "alpha",
		"CSIADDONS_SUBSCRIPTION_STARTINGCSV":             "csi-addons.v0.10.0",
		"CSIADDONS_SUBSCRIPTION_CATALOGSOURCE":           "odf-catalogsource",
		"CSIADDONS_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE": "openshift-marketplace",

		"CEPHCSI_SUBSCRIPTION_NAME":                    "cephcsi-operator",
		"CEPHCSI_SUBSCRIPTION_PACKAGE":                 "cephcsi-operator",
		"CEPHCSI_SUBSCRIPTION_CHANNEL":                 "alpha",
		"CEPHCSI_SUBSCRIPTION_STARTINGCSV":             "cephcsi-operator.v4.18.0",
		"CEPHCSI_SUBSCRIPTION_CATALOGSOURCE":           "odf-catalogsource",
		"CEPHCSI_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE": "openshift-marketplace",

		"IBM_SUBSCRIPTION_NAME":                    "ibm-storage-odf-operator",
		"IBM_SUBSCRIPTION_PACKAGE":                 "ibm-storage-odf-operator",
		"IBM_SUBSCRIPTION_CHANNEL":                 "stable-v1.6",
		"IBM_SUBSCRIPTION_STARTINGCSV":             "ibm-storage-odf-operator.v1.6.0",
		"IBM_SUBSCRIPTION_CATALOGSOURCE":           "odf-catalogsource",
		"IBM_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE": "openshift-marketplace",

		"ROOK_SUBSCRIPTION_NAME":                    "rook-ceph-operator",
		"ROOK_SUBSCRIPTION_PACKAGE":                 "rook-ceph-operator",
		"ROOK_SUBSCRIPTION_CHANNEL":                 "alpha",
		"ROOK_SUBSCRIPTION_STARTINGCSV":             "rook-ceph-operator.v4.18.0",
		"ROOK_SUBSCRIPTION_CATALOGSOURCE":           "odf-catalogsource",
		"ROOK_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE": "openshift-marketplace",

		"PROMETHEUS_SUBSCRIPTION_NAME":                    "odf-prometheus-operator",
		"PROMETHEUS_SUBSCRIPTION_PACKAGE":                 "odf-prometheus-operator",
		"PROMETHEUS_SUBSCRIPTION_CHANNEL":                 "beta",
		"PROMETHEUS_SUBSCRIPTION_STARTINGCSV":             "odf-prometheus-operator.v4.18.0",
		"PROMETHEUS_SUBSCRIPTION_CATALOGSOURCE":           "odf-catalogsource",
		"PROMETHEUS_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE": "openshift-marketplace",

		"RECIPE_SUBSCRIPTION_NAME":                    "recipe",
		"RECIPE_SUBSCRIPTION_PACKAGE":                 "recipe",
		"RECIPE_SUBSCRIPTION_CHANNEL":                 "alpha",
		"RECIPE_SUBSCRIPTION_STARTINGCSV":             "recipe.v0.0.1",
		"RECIPE_SUBSCRIPTION_CATALOGSOURCE":           "odf-catalogsource",
		"RECIPE_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE": "openshift-marketplace",
	}

	OperatorNamespace = GetEnvOrDefault("OPERATOR_NAMESPACE")

	OdfDepsSubscriptionName                   = GetEnvOrDefault("ODF_DEPS_SUBSCRIPTION_NAME")
	OdfDepsSubscriptionPackage                = GetEnvOrDefault("ODF_DEPS_SUBSCRIPTION_PACKAGE")
	OdfDepsSubscriptionChannel                = GetEnvOrDefault("ODF_DEPS_SUBSCRIPTION_CHANNEL")
	OdfDepsSubscriptionStartingCSV            = GetEnvOrDefault("ODF_DEPS_SUBSCRIPTION_STARTINGCSV")
	OdfDepsSubscriptionCatalogSource          = GetEnvOrDefault("ODF_DEPS_SUBSCRIPTION_CATALOGSOURCE")
	OdfDepsSubscriptionCatalogSourceNamespace = GetEnvOrDefault("ODF_DEPS_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE")

	OcsSubscriptionName                   = GetEnvOrDefault("OCS_SUBSCRIPTION_NAME")
	OcsSubscriptionPackage                = GetEnvOrDefault("OCS_SUBSCRIPTION_PACKAGE")
	OcsSubscriptionChannel                = GetEnvOrDefault("OCS_SUBSCRIPTION_CHANNEL")
	OcsSubscriptionStartingCSV            = GetEnvOrDefault("OCS_SUBSCRIPTION_STARTINGCSV")
	OcsSubscriptionCatalogSource          = GetEnvOrDefault("OCS_SUBSCRIPTION_CATALOGSOURCE")
	OcsSubscriptionCatalogSourceNamespace = GetEnvOrDefault("OCS_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE")

	OcsClientSubscriptionName                   = GetEnvOrDefault("OCS_CLIENT_SUBSCRIPTION_NAME")
	OcsClientSubscriptionPackage                = GetEnvOrDefault("OCS_CLIENT_SUBSCRIPTION_PACKAGE")
	OcsClientSubscriptionChannel                = GetEnvOrDefault("OCS_CLIENT_SUBSCRIPTION_CHANNEL")
	OcsClientSubscriptionStartingCSV            = GetEnvOrDefault("OCS_CLIENT_SUBSCRIPTION_STARTINGCSV")
	OcsClientSubscriptionCatalogSource          = GetEnvOrDefault("OCS_CLIENT_SUBSCRIPTION_CATALOGSOURCE")
	OcsClientSubscriptionCatalogSourceNamespace = GetEnvOrDefault("OCS_CLIENT_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE")

	NoobaaSubscriptionName                   = GetEnvOrDefault("NOOBAA_SUBSCRIPTION_NAME")
	NoobaaSubscriptionPackage                = GetEnvOrDefault("NOOBAA_SUBSCRIPTION_PACKAGE")
	NoobaaSubscriptionChannel                = GetEnvOrDefault("NOOBAA_SUBSCRIPTION_CHANNEL")
	NoobaaSubscriptionStartingCSV            = GetEnvOrDefault("NOOBAA_SUBSCRIPTION_STARTINGCSV")
	NoobaaSubscriptionCatalogSource          = GetEnvOrDefault("NOOBAA_SUBSCRIPTION_CATALOGSOURCE")
	NoobaaSubscriptionCatalogSourceNamespace = GetEnvOrDefault("NOOBAA_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE")

	CSIAddonsSubscriptionName                   = GetEnvOrDefault("CSIADDONS_SUBSCRIPTION_NAME")
	CSIAddonsSubscriptionPackage                = GetEnvOrDefault("CSIADDONS_SUBSCRIPTION_PACKAGE")
	CSIAddonsSubscriptionChannel                = GetEnvOrDefault("CSIADDONS_SUBSCRIPTION_CHANNEL")
	CSIAddonsSubscriptionStartingCSV            = GetEnvOrDefault("CSIADDONS_SUBSCRIPTION_STARTINGCSV")
	CSIAddonsSubscriptionCatalogSource          = GetEnvOrDefault("CSIADDONS_SUBSCRIPTION_CATALOGSOURCE")
	CSIAddonsSubscriptionCatalogSourceNamespace = GetEnvOrDefault("CSIADDONS_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE")

	CephCSISubscriptionName                   = GetEnvOrDefault("CEPHCSI_SUBSCRIPTION_NAME")
	CephCSISubscriptionPackage                = GetEnvOrDefault("CEPHCSI_SUBSCRIPTION_PACKAGE")
	CephCSISubscriptionChannel                = GetEnvOrDefault("CEPHCSI_SUBSCRIPTION_CHANNEL")
	CephCSISubscriptionStartingCSV            = GetEnvOrDefault("CEPHCSI_SUBSCRIPTION_STARTINGCSV")
	CephCSISubscriptionCatalogSource          = GetEnvOrDefault("CEPHCSI_SUBSCRIPTION_CATALOGSOURCE")
	CephCSISubscriptionCatalogSourceNamespace = GetEnvOrDefault("CEPHCSI_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE")

	IbmSubscriptionName                   = GetEnvOrDefault("IBM_SUBSCRIPTION_NAME")
	IbmSubscriptionPackage                = GetEnvOrDefault("IBM_SUBSCRIPTION_PACKAGE")
	IbmSubscriptionChannel                = GetEnvOrDefault("IBM_SUBSCRIPTION_CHANNEL")
	IbmSubscriptionStartingCSV            = GetEnvOrDefault("IBM_SUBSCRIPTION_STARTINGCSV")
	IbmSubscriptionCatalogSource          = GetEnvOrDefault("IBM_SUBSCRIPTION_CATALOGSOURCE")
	IbmSubscriptionCatalogSourceNamespace = GetEnvOrDefault("IBM_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE")

	RookSubscriptionName                   = GetEnvOrDefault("ROOK_SUBSCRIPTION_NAME")
	RookSubscriptionPackage                = GetEnvOrDefault("ROOK_SUBSCRIPTION_PACKAGE")
	RookSubscriptionChannel                = GetEnvOrDefault("ROOK_SUBSCRIPTION_CHANNEL")
	RookSubscriptionStartingCSV            = GetEnvOrDefault("ROOK_SUBSCRIPTION_STARTINGCSV")
	RookSubscriptionCatalogSource          = GetEnvOrDefault("ROOK_SUBSCRIPTION_CATALOGSOURCE")
	RookSubscriptionCatalogSourceNamespace = GetEnvOrDefault("ROOK_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE")

	PrometheusSubscriptionName                   = GetEnvOrDefault("PROMETHEUS_SUBSCRIPTION_NAME")
	PrometheusSubscriptionPackage                = GetEnvOrDefault("PROMETHEUS_SUBSCRIPTION_PACKAGE")
	PrometheusSubscriptionChannel                = GetEnvOrDefault("PROMETHEUS_SUBSCRIPTION_CHANNEL")
	PrometheusSubscriptionStartingCSV            = GetEnvOrDefault("PROMETHEUS_SUBSCRIPTION_STARTINGCSV")
	PrometheusSubscriptionCatalogSource          = GetEnvOrDefault("PROMETHEUS_SUBSCRIPTION_CATALOGSOURCE")
	PrometheusSubscriptionCatalogSourceNamespace = GetEnvOrDefault("PROMETHEUS_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE")

	RecipeSubscriptionName                   = GetEnvOrDefault("RECIPE_SUBSCRIPTION_NAME")
	RecipeSubscriptionPackage                = GetEnvOrDefault("RECIPE_SUBSCRIPTION_PACKAGE")
	RecipeSubscriptionChannel                = GetEnvOrDefault("RECIPE_SUBSCRIPTION_CHANNEL")
	RecipeSubscriptionStartingCSV            = GetEnvOrDefault("RECIPE_SUBSCRIPTION_STARTINGCSV")
	RecipeSubscriptionCatalogSource          = GetEnvOrDefault("RECIPE_SUBSCRIPTION_CATALOGSOURCE")
	RecipeSubscriptionCatalogSourceNamespace = GetEnvOrDefault("RECIPE_SUBSCRIPTION_CATALOGSOURCE_NAMESPACE")
)

const (
	OdfSubscriptionPackage = "odf-operator"
)

var (
	EssentialCSVs = []string{
		OcsSubscriptionStartingCSV,
		RookSubscriptionStartingCSV,
		NoobaaSubscriptionStartingCSV,
	}
)

func GetEnvOrDefault(env string) string {
	if val := os.Getenv(env); val != "" {
		return val
	}

	return DefaultValMap[env]
}
