package analyzer

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// AnalyzeHostTextAnalyze implements HostAnalyzer interface
type AnalyzeHostTextAnalyze struct {
	hostAnalyzer *troubleshootv1beta2.TextAnalyze
}

func (a *AnalyzeHostTextAnalyze) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Regular Expression")
}

func (a *AnalyzeHostTextAnalyze) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostTextAnalyze) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	return analyzeTextAnalyze(a.hostAnalyzer, findFiles, a.Title())
}
