package analyzer

import (
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type AnalyzeHTTPAnalyze struct {
	analyzer *troubleshootv1beta2.HTTPAnalyze
}

func (a *AnalyzeHTTPAnalyze) Title() string {
	checkName := a.analyzer.CheckName
	if checkName == "" {
		checkName = a.analyzer.CollectorName
	}

	return checkName
}

func (a *AnalyzeHTTPAnalyze) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeHTTPAnalyze) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	fileName := "result.json"
	if a.analyzer.CollectorName != "" {
		fileName = a.analyzer.CollectorName + ".json"
	}
	return analyzeHTTPResult(a.analyzer, fileName, getFile, a.Title())
}
