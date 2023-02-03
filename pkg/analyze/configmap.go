package analyzer

import (
	"encoding/json"
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeConfigMap struct {
	analyzer *troubleshootv1beta2.AnalyzeConfigMap
}

func (a *AnalyzeConfigMap) Title() string {
	title := a.analyzer.CheckName
	if title == "" {
		title = fmt.Sprintf("ConfigMap %s", a.analyzer.ConfigMapName)
	}

	return title
}

func (a *AnalyzeConfigMap) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeConfigMap) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	result, err := a.analyzeConfigMap(a.analyzer, getFile)
	if err != nil {
		return nil, err
	}
	result.Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}

func (a *AnalyzeConfigMap) analyzeConfigMap(analyzer *troubleshootv1beta2.AnalyzeConfigMap, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	filename := collect.GetConfigMapFileName(
		&troubleshootv1beta2.ConfigMap{
			Namespace: analyzer.Namespace,
			Name:      analyzer.ConfigMapName,
			Key:       analyzer.Key,
		},
		analyzer.ConfigMapName,
	)

	configMapData, err := getCollectedFileContents(filename)
	if err != nil {
		return nil, err
	}

	var foundConfigMap collect.ConfigMapOutput
	if err := json.Unmarshal(configMapData, &foundConfigMap); err != nil {
		return nil, err
	}

	result := AnalyzeResult{
		Title:   a.Title(),
		IconKey: "kubernetes_analyze_secret", // TODO: icon
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/secret.svg?w=13&h=16",
	}

	var failOutcome *troubleshootv1beta2.Outcome
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			failOutcome = outcome
		}
	}

	if !foundConfigMap.ConfigMapExists {
		result.IsFail = true
		result.Message = failOutcome.Fail.Message
		result.URI = failOutcome.Fail.URI

		return &result, nil
	}

	if analyzer.Key != "" {
		if foundConfigMap.Key != analyzer.Key || !foundConfigMap.KeyExists {
			result.IsFail = true
			result.Message = failOutcome.Fail.Message
			result.URI = failOutcome.Fail.URI

			return &result, nil
		}
	}

	result.IsPass = true
	for _, outcome := range analyzer.Outcomes {
		if outcome.Pass != nil {
			result.Message = outcome.Pass.Message
			result.URI = outcome.Pass.URI
		}
	}

	return &result, nil
}
