package analyzer

import (
	"path"
	"regexp"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

func analyzeTextAnalyze(analyzer *troubleshootv1beta1.TextAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	fullPath := path.Join(analyzer.CollectorName, analyzer.FileName)
	collected, err := getCollectedFileContents(fullPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read collected file name: %s", fullPath)
	}

	re, err := regexp.Compile(analyzer.RegexPattern)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to compile regex: %s", analyzer.RegexPattern)
	}

	var failOutcome *troubleshootv1beta1.Outcome
	var passOutcome *troubleshootv1beta1.Outcome
	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			failOutcome = outcome
		} else if outcome.Pass != nil {
			passOutcome = outcome
		}
	}

	if re.MatchString(string(collected)) {
		return &AnalyzeResult{
			Title:   analyzer.CheckName,
			IsPass:  true,
			Message: passOutcome.Pass.Message,
			URI:     passOutcome.Pass.URI,
		}, nil
	}
	return &AnalyzeResult{
		Title:   analyzer.CheckName,
		IsFail:  true,
		Message: failOutcome.Fail.Message,
		URI:     failOutcome.Fail.URI,
	}, nil
}
