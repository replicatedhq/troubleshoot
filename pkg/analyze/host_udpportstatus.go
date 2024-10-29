package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

// AnalyzeHostUDPPortStatus is an analyzer that will return only one matching result, or a warning if nothing matches. The first
// match that is encountered is the one that is returned.
type AnalyzeHostUDPPortStatus struct {
	hostAnalyzer *troubleshootv1beta2.UDPPortStatusAnalyze
}

func (a *AnalyzeHostUDPPortStatus) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "UDP Port Status")
}

func (a *AnalyzeHostUDPPortStatus) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostUDPPortStatus) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "udpPortStatus"
	}

	const nodeBaseDir = "host-collectors/udpPortStatus"
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
		return nil, errors.Wrap(err, "failed to analyze udp port status")
	}

	return results, nil

}

func (a *AnalyzeHostUDPPortStatus) CheckCondition(when string, data []byte) (bool, error) {

	var udpPort collect.NetworkStatusResult
	if err := json.Unmarshal(data, &udpPort); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into NetworkStatusResult: %v", err)
	}

	return string(udpPort.Status) == when, nil
}
