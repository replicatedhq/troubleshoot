package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

// AnalyzeHostTCPPortStatus is an analyzer that will return only one matching result, or a warning if nothing matches. The first
// match that is encountered is the one that is returned.
type AnalyzeHostTCPPortStatus struct {
	hostAnalyzer *troubleshootv1beta2.TCPPortStatusAnalyze
}

func (a *AnalyzeHostTCPPortStatus) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "TCP Port Status")
}

func (a *AnalyzeHostTCPPortStatus) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostTCPPortStatus) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "tcpPortStatus"
	}

	const nodeBaseDir = "host-collectors/tcpPortStatus"
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
		return nil, errors.Wrap(err, "failed to analyze tcp port status")
	}

	return results, nil

}

func (a *AnalyzeHostTCPPortStatus) CheckCondition(when string, data []byte) (bool, error) {

	var tcpPortStatus collect.NetworkStatusResult
	if err := json.Unmarshal(data, &tcpPortStatus); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into NetworkStatusResult: %v", err)
	}

	return string(tcpPortStatus.Status) == when, nil
}
