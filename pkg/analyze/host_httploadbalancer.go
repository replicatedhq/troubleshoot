package analyzer

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostHTTPLoadBalancer struct {
	hostAnalyzer *troubleshootv1beta2.HTTPLoadBalancerAnalyze
}

func (a *AnalyzeHostHTTPLoadBalancer) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "HTTP Load Balancer")
}

func (a *AnalyzeHostHTTPLoadBalancer) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostHTTPLoadBalancer) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "httpLoadBalancer"
	}

	localPath := fmt.Sprintf("host-collectors/httpLoadBalancer/%s.json", collectorName)
	fileName := fmt.Sprintf("%s.json", collectorName)

	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		localPath,
		collect.NodeInfoBaseDir,
		fileName,
	)
	if err != nil {
		return []*AnalyzeResult{{Title: a.Title()}}, err
	}

	results, err := analyzeHostCollectorResults(collectedContents, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze HTTP Load Balancer")
	}

	return results, nil
}

func (a *AnalyzeHostHTTPLoadBalancer) CheckCondition(when string, data collectorData) (bool, error) {
	rawData, ok := data.([]byte)
	if !ok {
		return false, fmt.Errorf("expected data to be []uint8 (raw bytes), got: %v", reflect.TypeOf(data))
	}

	var httpLoadBalancer collect.NetworkStatusResult
	if err := json.Unmarshal(rawData, &httpLoadBalancer); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into NetworkStatusResult: %v", err)
	}

	if string(httpLoadBalancer.Status) == when {
		return true, nil
	}

	return false, nil
}
