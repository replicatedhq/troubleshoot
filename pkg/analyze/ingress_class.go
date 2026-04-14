package analyzer

import (
	"encoding/json"
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	networkingv1 "k8s.io/api/networking/v1"
)

type AnalyzeIngressClass struct {
	analyzer *troubleshootv1beta2.IngressClass
}

func (a *AnalyzeIngressClass) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		if a.analyzer.IngressClassName != "" {
			title = fmt.Sprintf("Ingress class %s", a.analyzer.IngressClassName)
		} else {
			title = "Default Ingress Class"
		}
	}
	return title
}

func (a *AnalyzeIngressClass) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeIngressClass) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeIngressClass(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeIngressClass) analyzeIngressClass(analyzer *troubleshootv1beta2.IngressClass, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	ingressClassesData, err := getCollectedFileContents(fmt.Sprintf("%s/%s.json", constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_INGRESS_CLASS))
	if err != nil {
		return nil, err
	}

	var ingressClasses networkingv1.IngressClassList
	if err := json.Unmarshal(ingressClassesData, &ingressClasses); err != nil {
		return nil, err
	}

	result := AnalyzeResult{
		Title:   a.Title(),
		IconKey: "kubernetes_ingress_class",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/ingress-class.svg?w=12&h=12",
	}

	for _, ingressClass := range ingressClasses.Items {
		val := ingressClass.Annotations["ingressclass.kubernetes.io/is-default-class"]
		if (ingressClass.Name == analyzer.IngressClassName) || (analyzer.IngressClassName == "" && val == "true") {
			result.IsPass = true
			for _, outcome := range analyzer.Outcomes {
				if outcome.Pass != nil {
					result.Message = outcome.Pass.Message
					result.URI = outcome.Pass.URI
				}
			}
			if analyzer.IngressClassName == "" && result.Message == "" {
				result.Message = "Default Ingress Class found"
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
	if analyzer.IngressClassName == "" && result.Message == "" {
		result.Message = "No Default Ingress Class found"
	}

	return &result, nil
}
