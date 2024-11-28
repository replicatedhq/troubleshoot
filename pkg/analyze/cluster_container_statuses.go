package analyzer

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type AnalyzeClusterContainerStatuses struct {
	analyzer *troubleshootv1beta2.ClusterContainerStatuses
}

func (a *AnalyzeClusterContainerStatuses) Title() string {
	if a.analyzer.CheckName != "" {
		return a.analyzer.CheckName
	}
	return "Cluster Container Status"
}

func (a *AnalyzeClusterContainerStatuses) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeClusterContainerStatuses) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult

	// TODO

	return results, nil
}
