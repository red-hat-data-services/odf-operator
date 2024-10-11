package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"
)

var (
	csvInputFiles = flag.String("csv-input-files", "", "the space separated CSV input files of child operators")
)

func main() {
	flag.Parse()

	if *csvInputFiles == "" {
		log.Fatal("--csv-input-files is required")
	}

	odfCsvInputFile := "bundle/manifests/odf-operator.clusterserviceversion.yaml"
	odfCsvOutputFile := "bundle/manifests/odf-operator.clusterserviceversion.yaml"
	deploymentsOutputFile := "config/deployments/deployments.yaml"

	csvFiles := strings.Split(*csvInputFiles, " ")
	odfCsv := unmarshalCSV(odfCsvInputFile)

	deployments := []appsv1.Deployment{}
	apis := []string{}

	for _, csvFile := range csvFiles {
		csv := unmarshalCSV(csvFile)

		// whitelisting APIs
		for _, crd := range csv.Spec.CustomResourceDefinitions.Owned {
			apis = append(apis, crd.Name)
		}

		odfCsv.Spec.CustomResourceDefinitions.Owned = append(
			odfCsv.Spec.CustomResourceDefinitions.Owned, csv.Spec.CustomResourceDefinitions.Owned...)

		odfCsv.Spec.InstallStrategy.StrategySpec.ClusterPermissions = append(
			odfCsv.Spec.InstallStrategy.StrategySpec.ClusterPermissions, csv.Spec.InstallStrategy.StrategySpec.ClusterPermissions...)

		odfCsv.Spec.InstallStrategy.StrategySpec.Permissions = append(
			odfCsv.Spec.InstallStrategy.StrategySpec.Permissions, csv.Spec.InstallStrategy.StrategySpec.Permissions...)

		for _, deploymentSpec := range csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs {

			// Append the deployments to the odf CSV
			if deploymentSpec.Name == "" {
				odfCsv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs = append(
					odfCsv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs, csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs...)
				continue
			}

			// Append the deployments to the deployments
			deployment := &appsv1.Deployment{}
			deployment.ObjectMeta.Name = deploymentSpec.Name
			deployment.ObjectMeta.Labels = deploymentSpec.Label
			deployment.Spec = deploymentSpec.Spec

			deployments = append(deployments, *deployment)
		}
	}

	odfCsv.Annotations["operators.operatorframework.io/internal-objects"] = "[\"" + strings.Join(apis, "\",\"") + "\"]"

	writeObjectToFile(deployments, deploymentsOutputFile)
	writeObjectToFile(odfCsv, odfCsvOutputFile)
}

// unmarshalCSV reads the csv yaml from a given filePath and return a CSV structure
func unmarshalCSV(filePath string) *operatorsv1alpha1.ClusterServiceVersion {

	fmt.Printf("Reading file %s\n", filePath)
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	csv := &operatorsv1alpha1.ClusterServiceVersion{}
	err = yaml.Unmarshal(bytes, csv)
	if err != nil {
		panic(err)
	}

	return csv
}

// writeObjectToFile writes a k8s object to the given filePath into yaml format
func writeObjectToFile(object interface{}, filePath string) {

	bytes, err := yaml.Marshal(object)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Writing file %s\n", filePath)
	err = os.WriteFile(filePath, bytes, 0644)
	if err != nil {
		panic(err)
	}
}
