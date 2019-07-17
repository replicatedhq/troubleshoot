package analyzer

import (
	"encoding/json"
	"fmt"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
)

func analyzeIngress(analyzer *troubleshootv1beta1.Ingress, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	ingressData, err := getCollectedFileContents("cluster-resources/storage-classes.json")
	if err != nil {
		return nil, err
	}

	var ingresses []extensionsv1beta1.Ingress
	if err := json.Unmarshal(ingressData, &ingresses); err != nil {
		return nil, err
	}

	title := analyzer.CheckName
	if title == "" {
		title = fmt.Sprintf("Ingress %s", analyzer.IngressName)
	}

	result := AnalyzeResult{
		Title: title,
	}

	for _, ingress := range ingresses {
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
