package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostTCPConnect struct {
	hostAnalyzer *troubleshootv1beta2.TCPConnectAnalyze
}

func (a *AnalyzeHostTCPConnect) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "TCP Connection Attempt")
}

func (a *AnalyzeHostTCPConnect) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostTCPConnect) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "connect"
	}

	const nodeBaseDir = "host-collectors/connect"
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
		return nil, errors.Wrap(err, "failed to analyze tcp connect")
	}

	return results, nil
}

func (a *AnalyzeHostTCPConnect) CheckCondition(when string, data []byte) (bool, error) {

	var tcpConnect collect.NetworkStatusResult
	if err := json.Unmarshal(data, &tcpConnect); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into NetworkStatusResult: %v", err)
	}

	return string(tcpConnect.Status) == when, nil
}
