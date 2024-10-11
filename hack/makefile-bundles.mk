CEPHCSI_BUNDLE_IMG ?= quay.io/ocs-dev/cephcsi-operator-bundle:release-4.17-fb535da
CSIADDONS_BUNDLE_IMG ?= quay.io/csiaddons/k8s-bundle:v0.9.1
NOOBAA_BUNDLE_IMG ?= quay.io/noobaa/noobaa-operator-bundle:master-20240829
OCS_BUNDLE_IMG ?= quay.io/ocs-dev/ocs-operator-bundle:release-4.17-772eb9c
OCS_CLIENT_BUNDLE_IMG ?= quay.io/ocs-dev/ocs-client-operator-bundle:main-5595a28
PROMETHEUS_BUNDLE_IMG ?= quay.io/ocs-dev/odf-prometheus-operator-bundle:main-d82716f
RECIPE_BUNDLE_IMG ?= quay.io/ramendr/recipe-bundle:latest
ROOK_BUNDLE_IMG ?= quay.io/ocs-dev/rook-ceph-operator-bundle:release-4.17-91cc5780b


extract-maifests:

	rm -rf config/child/*

	$(CONTAINER_TOOL) create --name cephcsi $(CEPHCSI_BUNDLE_IMG) /bin/true
	$(CONTAINER_TOOL) cp cephcsi:/manifests config/child/cephcsi
	$(CONTAINER_TOOL) rm cephcsi

	$(CONTAINER_TOOL) create --name csiaddons $(CSIADDONS_BUNDLE_IMG) /bin/true
	$(CONTAINER_TOOL) cp csiaddons:/manifests config/child/csiaddons
	$(CONTAINER_TOOL) rm csiaddons

	$(CONTAINER_TOOL) create --name noobaa $(NOOBAA_BUNDLE_IMG) /bin/true
	$(CONTAINER_TOOL) cp noobaa:/manifests config/child/noobaa
	$(CONTAINER_TOOL) rm noobaa

	$(CONTAINER_TOOL) create --name ocs $(OCS_BUNDLE_IMG) /bin/true
	$(CONTAINER_TOOL) cp ocs:/manifests config/child/ocs
	$(CONTAINER_TOOL) rm ocs

	$(CONTAINER_TOOL) create --name client $(OCS_CLIENT_BUNDLE_IMG) /bin/true
	$(CONTAINER_TOOL) cp client:/manifests config/child/client
	$(CONTAINER_TOOL) rm client

	$(CONTAINER_TOOL) create --name prometheus $(PROMETHEUS_BUNDLE_IMG) /bin/true
	$(CONTAINER_TOOL) cp prometheus:/manifests config/child/prometheus
	$(CONTAINER_TOOL) rm prometheus

	$(CONTAINER_TOOL) create --name recipe $(RECIPE_BUNDLE_IMG) /bin/true
	$(CONTAINER_TOOL) cp recipe:/manifests config/child/recipe
	$(CONTAINER_TOOL) rm recipe

	$(CONTAINER_TOOL) create --name rook $(ROOK_BUNDLE_IMG) /bin/true
	$(CONTAINER_TOOL) cp rook:/manifests config/child/rook
	$(CONTAINER_TOOL) rm rook

render-bundles: opm
	$(OPM) render $(CEPHCSI_BUNDLE_IMG) > catalog/cephcsi.json
	$(OPM) render $(CSIADDONS_BUNDLE_IMG) > catalog/csiaddons.json
	$(OPM) render $(NOOBAA_BUNDLE_IMG) > catalog/noobaa.json
	$(OPM) render $(OCS_BUNDLE_IMG) > catalog/ocs.json
	$(OPM) render $(OCS_CLIENT_BUNDLE_IMG) > catalog/client.json
	$(OPM) render $(PROMETHEUS_BUNDLE_IMG) > catalog/prometheus.json
	$(OPM) render $(RECIPE_BUNDLE_IMG) > catalog/recipe.json
	$(OPM) render $(ROOK_BUNDLE_IMG) > catalog/rook.json
