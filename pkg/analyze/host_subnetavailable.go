package analyzer

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type AnalyzeHostSubnetAvailable struct {
	hostAnalyzer *troubleshootv1beta2.SubnetAvailableAnalyze
}

func (a *AnalyzeHostSubnetAvailable) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Subnet Available")
}

func (a *AnalyzeHostSubnetAvailable) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostSubnetAvailable) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	// TODO: implement

	var coll resultCollector

	return coll.get(a.Title()), nil
}
