package preflight

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// Analyze runs the analyze phase of preflight checks
func (c ClusterCollectResult) Analyze() []*analyze.AnalyzeResult {
	return doAnalyze(c.AllCollectedData, c.Spec.Spec.Analyzers, nil, "")
}

// Analyze runs the analysze phase of host preflight checks
func (c HostCollectResult) Analyze() []*analyze.AnalyzeResult {
	return doAnalyze(c.AllCollectedData, nil, c.Spec.Spec.Analyzers, "")
}

// Analyze runs the analysze phase of host preflight checks.
//
// Runs the analysis for each node and aggregates the results.
func (c RemoteCollectResult) Analyze() []*analyze.AnalyzeResult {
	var results []*analyze.AnalyzeResult
	for nodeName, nodeResult := range c.AllCollectedData {
		var strResult = make(map[string]string)
		if err := json.Unmarshal(nodeResult, &strResult); err != nil {
			analyzeResult := &analyze.AnalyzeResult{
				IsFail:  true,
				Title:   "Remote Result Parser Failed",
				Message: err.Error(),
			}
			results = append(results, analyzeResult)
			continue
		}

		var byteResult = make(map[string][]byte)
		for k, v := range strResult {
			byteResult[k] = []byte(v)

		}
		results = append(results, doAnalyze(byteResult, nil, c.Spec.Spec.Analyzers, nodeName)...)
	}
	return results
}

func doAnalyze(allCollectedData map[string][]byte, analyzers []*troubleshootv1beta2.Analyze, hostAnalyzers []*troubleshootv1beta2.HostAnalyze, nodeName string) []*analyze.AnalyzeResult {
	getCollectedFileContents := func(fileName string) ([]byte, error) {
		contents, ok := allCollectedData[fileName]
		if !ok {
			return nil, fmt.Errorf("file %s was not collected", fileName)
		}

		return contents, nil
	}
	getChildCollectedFileContents := func(prefix string) (map[string][]byte, error) {
		matching := make(map[string][]byte)
		for k, v := range allCollectedData {
			if strings.HasPrefix(k, prefix) {
				matching[k] = v
			}
		}

		for k, v := range allCollectedData {
			if ok, _ := filepath.Match(prefix, k); ok {
				matching[k] = v
			}
		}

		return matching, nil
	}

	analyzeResults := []*analyze.AnalyzeResult{}
	for _, analyzer := range analyzers {
		analyzeResult, err := analyze.Analyze(analyzer, getCollectedFileContents, getChildCollectedFileContents)
		if err != nil {
			analyzeResult = []*analyze.AnalyzeResult{
				{
					IsFail:  true,
					Title:   "Analyzer Failed",
					Message: err.Error(),
				},
			}
		}

		if analyzeResult != nil {
			analyzeResults = append(analyzeResults, analyzeResult...)
		}
	}

	for _, hostAnalyzer := range hostAnalyzers {
		analyzeResult := analyze.HostAnalyze(hostAnalyzer, getCollectedFileContents, getChildCollectedFileContents)
		analyzeResults = append(analyzeResults, analyzeResult...)
	}

	// Add the nodename to the result title if provided.
	if nodeName != "" {
		for _, result := range analyzeResults {
			result.Title = fmt.Sprintf("%s (%s)", result.Title, nodeName)
		}
	}
	return analyzeResults
}
