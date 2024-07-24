package analyzer

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type AnalyzeHostJsonCompare struct {
	hostAnalyzer *troubleshootv1beta2.JsonCompare
}

func (a *AnalyzeHostJsonCompare) Title() string {
	return jsonCompareTitle(a.hostAnalyzer)
}

func (a *AnalyzeHostJsonCompare) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostJsonCompare) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	result, err := analyzeJsonCompare(a.hostAnalyzer, getCollectedFileContents, a.Title())
	if err != nil {
		return nil, err
	}
	result.Strict = a.hostAnalyzer.Strict.BoolOrDefaultFalse()
	return []*AnalyzeResult{result}, nil
}
