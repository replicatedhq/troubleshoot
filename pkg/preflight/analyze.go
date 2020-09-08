package preflight

import (
	"fmt"
	"path/filepath"
	"strings"

	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

// Analyze runs the analyze phase of preflight checks
func (c CollectResult) Analyze() []*analyze.AnalyzeResult {
	getCollectedFileContents := func(fileName string) ([]byte, error) {
		contents, ok := c.AllCollectedData[fileName]
		if !ok {
			return nil, fmt.Errorf("file %s was not collected", fileName)
		}

		return contents, nil
	}
	getChildCollectedFileContents := func(prefix string) (map[string][]byte, error) {
		matching := make(map[string][]byte)
		for k, v := range c.AllCollectedData {
			if strings.HasPrefix(k, prefix) {
				matching[k] = v
			}
		}

		for k, v := range c.AllCollectedData {
			if ok, _ := filepath.Match(prefix, k); ok {
				matching[k] = v
			}
		}

		return matching, nil
	}

	analyzeResults := []*analyze.AnalyzeResult{}
	for _, analyzer := range c.Spec.Spec.Analyzers {
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

	return analyzeResults
}
