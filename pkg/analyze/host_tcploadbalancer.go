package analyzer

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostTCPLoadBalancer struct {
	hostAnalyzer *troubleshootv1beta2.TCPLoadBalancerAnalyze
}

func (a *AnalyzeHostTCPLoadBalancer) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "TCP Load Balancer")
}

func (a *AnalyzeHostTCPLoadBalancer) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostTCPLoadBalancer) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {

	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "tcpLoadBalancer"
	}

	const nodeBaseDir = "host-collectors/tcpLoadBalancer"
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
		return nil, errors.Wrap(err, "failed to analyze TCP Load Balancer")
	}

	return results, nil

}

func (a *AnalyzeHostTCPLoadBalancer) CheckCondition(when string, data collectorData) (bool, error) {
	rawData, ok := data.([]byte)
	if !ok {
		return false, fmt.Errorf("expected data to be []uint8 (raw bytes), got: %v", reflect.TypeOf(data))
	}

	var tcpLoadBalancer collect.NetworkStatusResult
	if err := json.Unmarshal(rawData, &tcpLoadBalancer); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into NetworkStatusResult: %v", err)
	}

	if string(tcpLoadBalancer.Status) == when {
		return true, nil
	}

	return false, nil
}
