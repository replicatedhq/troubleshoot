package analyzer

import (
	"fmt"
	"strings"

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

func (a *AnalyzeHostCertificate) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {

	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "certificate"
	}

	const nodeBaseDir = "host-collectors/certificate"
	localPath := fmt.Sprintf("%s/%s.json", nodeBaseDir, collectorName)
	fileName := fmt.Sprintf("%s.json", collectorName)

	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		localPath,
		nodeBaseDir,
		fileName,
	)
	if err != nil {
		return []*AnalyzeResult{{Title: a.Title()}}, err
	}

	results, err := analyzeHostCollectorResults(collectedContents, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze certificate")
	}

	return results, nil
}

func (a *AnalyzeHostCertificate) CheckCondition(when string, data []byte) (bool, error) {
	return strings.TrimSpace(string(data)) == when, nil
}
