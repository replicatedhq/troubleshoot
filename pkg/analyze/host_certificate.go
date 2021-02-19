package analyzer

import (
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func analyzeHostCertificate(hostAnalyzer *troubleshootv1beta2.CertificateAnalyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	collectorName := hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "certificate"
	}
	name := filepath.Join("certificate", collectorName+".json")
	contents, err := getCollectedFileContents(name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}
	status := string(contents)

	result := AnalyzeResult{}

	title := hostAnalyzer.CheckName
	if title == "" {
		title = "Certificate Key Pair"
	}
	result.Title = title

	for _, outcome := range hostAnalyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" || outcome.Fail.When == status {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return &result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" || outcome.Warn.When == status {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return &result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" || outcome.Pass.When == status {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return &result, nil
			}
		}
	}

	return &result, nil
}
