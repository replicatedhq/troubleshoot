package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/types"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type AnalyzeHostTLSCertificate struct {
	hostAnalyzer *troubleshootv1beta2.TLSAnalyze
}

func (a *AnalyzeHostTLSCertificate) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "TLSCertificate")
}

func (a *AnalyzeHostTLSCertificate) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostTLSCertificate) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {

	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		return nil, fmt.Errorf("collector name is required")
	}

	const nodeBaseDir = "host-collectors/tls-certificate"
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
		return nil, errors.Wrap(err, "failed to analyze http request")
	}

	return results, nil
}

func (a *AnalyzeHostTLSCertificate) CheckCondition(when string, data []byte) (bool, error) {
	var tlsInfo types.TLSInfo
	if err := json.Unmarshal(data, &tlsInfo); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into tlsInfo: %v", err)
	}
	return compareHostTLSResult(when, &tlsInfo)
}

// currently this supports only checks like "issuer == foo"
func compareHostTLSResult(when string, tlsInfo *types.TLSInfo) (bool, error) {
	parts := strings.Split(when, " ")

	checkType := parts[0]
	switch checkType {
	case "issuer":
		// check that the issuer matches the provided expected value
		if len(parts) < 3 {
			return false, fmt.Errorf("invalid when clause: %s", when)
		}

		expected := strings.Join(parts[2:], " ")

		for _, cert := range tlsInfo.PeerCertificates {
			if cert.Issuer == expected {
				return true, nil
			}
		}
		return false, nil
	case "matchesExpected":
		// check that the certificate's information matches what the server returned inside the response body
		if tlsInfo.ExpectedCerts == nil {
			return false, fmt.Errorf("expected certs not found in response")
		}

		// only check the issuer of the first expected cert today
		expected := tlsInfo.ExpectedCerts[0]
		comparison := tlsInfo.PeerCertificates[0]

		return expected.Issuer == comparison.Issuer, nil
	default:
		return false, fmt.Errorf("invalid check type: %s", checkType)
	}
}
