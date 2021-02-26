package preflight

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

// Analyze runs the analyze phase of preflight checks
func (c ClusterCollectResult) Analyze() []*analyze.AnalyzeResult {
	return doAnalyze(c.AllCollectedData, c.Spec.Spec.Analyzers, nil)
}

// Analyze runs the analysze phase of host preflight checks
func (c HostCollectResult) Analyze() []*analyze.AnalyzeResult {
	return doAnalyze(c.AllCollectedData, nil, c.Spec.Spec.Analyzers)
}

func doAnalyze(allCollectedData map[string][]byte, analyzers []*troubleshootv1beta2.Analyze, hostAnalyzers []*troubleshootv1beta2.HostAnalyze) []*analyze.AnalyzeResult {
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
		if excluded, _ := isExcluded(hostAnalyzer.Exclude); excluded {
			continue
		}
		analyzeResult, err := analyze.HostAnalyze(hostAnalyzer, getCollectedFileContents, getChildCollectedFileContents)
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

func isExcluded(excludeVal multitype.BoolOrString) (bool, error) {
	if excludeVal.Type == multitype.Bool {
		return excludeVal.BoolVal, nil
	}

	if excludeVal.StrVal == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(excludeVal.StrVal)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse bool string")
	}

	return parsed, nil
}
