package analyzer

import (
	"encoding/json"
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
)

func analyzeCustomResourceDefinition(analyzer *troubleshootv1beta2.CustomResourceDefinition, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	crdData, err := getCollectedFileContents(fmt.Sprintf("%s/%s.json", constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_CUSTOM_RESOURCE_DEFINITIONS))
	if err != nil {
		return nil, err
	}

	var crds apiextensionsv1beta1.CustomResourceDefinitionList
	if err := json.Unmarshal(crdData, &crds); err != nil {
		return nil, err
	}

	title := analyzer.CheckName
	if title == "" {
		title = fmt.Sprintf("Custom resource definition %s", analyzer.CustomResourceDefinitionName)
	}

	result := AnalyzeResult{
		Title:   title,
		IconKey: "kubernetes_custom_resource_definition",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/custom-resource-definition.svg?w=13&h=16",
	}

	for _, crd := range crds.Items {
		if crd.Name == analyzer.CustomResourceDefinitionName {
			result.IsPass = true
			for _, outcome := range analyzer.Outcomes {
				if outcome.Pass != nil {
					result.Message = outcome.Pass.Message
					result.URI = outcome.Pass.URI
				}
			}

			return &result, nil
		}
	}

	result.IsFail = true
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			result.Message = outcome.Fail.Message
			result.URI = outcome.Fail.URI
		}
	}

	return &result, nil
}
