package analyzer

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/analyze/types"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type AnalyzeHostTLS struct {
	hostAnalyzer *troubleshootv1beta2.TLSAnalyze
}

func (a *AnalyzeHostTLS) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "TLS")
}

func (a *AnalyzeHostTLS) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostTLS) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {

	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "result"
	}

	const nodeBaseDir = "host-collectors/tls"
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

func (a *AnalyzeHostTLS) CheckCondition(when string, data []byte) (bool, error) {

	var tlsInfo types.TLSInfo
	if err := json.Unmarshal(data, &tlsInfo); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into tlsInfo: %v", err)
	}
	return compareHostTLSResult(when, &tlsInfo)
}

// currently this supports only checks like "issuer == foo"
func compareHostTLSResult(when string, tlsInfo *types.TLSInfo) (bool, error) {
	parts := strings.SplitN(when, " ", 3)
	if len(parts) < 3 {
		return false, fmt.Errorf("invalid when clause: %s", when)
	}

	checkType := parts[0]
	if checkType != "issuer" {
		return false, fmt.Errorf("invalid check type: %s", checkType)
	}

	issuer := parts[2]

	for _, cert := range tlsInfo.PeerCertificates {
		if cert.Issuer == issuer {
			return true, nil
		}
	}

	return false, nil
}
