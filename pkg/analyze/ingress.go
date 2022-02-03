package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
)

func analyzeIngress(analyzer *troubleshootv1beta2.Ingress, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	ingressData, err := getCollectedFileContents(filepath.Join("cluster-resources", "ingress", fmt.Sprintf("%s.json", analyzer.Namespace)))
	if err != nil {
		return nil, err
	}

	var ingresses extensionsv1beta1.IngressList
	if err := json.Unmarshal(ingressData, &ingresses); err != nil {
		return nil, err
	}

	title := analyzer.CheckName
	if title == "" {
		title = fmt.Sprintf("Ingress %s", analyzer.IngressName)
	}

	result := AnalyzeResult{
		Title:   title,
		IconKey: "kubernetes_ingress",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/ingress-controller.svg?w=20&h=13",
	}

	for _, ingress := range ingresses.Items {
		if ingress.Name == analyzer.IngressName {
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
