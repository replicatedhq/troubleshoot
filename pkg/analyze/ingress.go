package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
)

type AnalyzeIngress struct {
	analyzer *troubleshootv1beta2.Ingress
}

func (a *AnalyzeIngress) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = fmt.Sprintf("Ingress %s", a.analyzer.IngressName)
	}

	return title
}

func (a *AnalyzeIngress) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeIngress) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeIngress(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeIngress) analyzeIngress(analyzer *troubleshootv1beta2.Ingress, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	ingressData, err := getCollectedFileContents(filepath.Join(constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_INGRESS, fmt.Sprintf("%s.json", analyzer.Namespace)))
	if err != nil {
		return nil, err
	}

	var ingresses extensionsv1beta1.IngressList
	if err := json.Unmarshal(ingressData, &ingresses); err != nil {
		return nil, err
	}

	result := AnalyzeResult{
		Title:   a.Title(),
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
