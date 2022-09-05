package analyzer

import (
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type AnalyzeHostCertificate struct {
	hostAnalyzer *troubleshootv1beta2.CertificateAnalyze
}

func (a *AnalyzeHostCertificate) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Certificate Key Pair")
}

func (a *AnalyzeHostCertificate) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostCertificate) Analyze(getCollectedFileContents func(string) ([]byte, error)) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	collectorName := hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "certificate"
	}
	name := filepath.Join("host-collectors/certificate", collectorName+".json")
	contents, err := getCollectedFileContents(name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}
	status := string(contents)

	var coll resultCollector

	for _, outcome := range hostAnalyzer.Outcomes {
		result := &AnalyzeResult{Title: a.Title()}

		if outcome.Fail != nil {
			if outcome.Fail.When == "" || outcome.Fail.When == status {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				coll.push(result)
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" || outcome.Warn.When == status {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				coll.push(result)
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" || outcome.Pass.When == status {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				coll.push(result)
			}
		}
	}

	return coll.get(a.Title()), nil
}
