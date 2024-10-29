package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostSubnetAvailable struct {
	hostAnalyzer *troubleshootv1beta2.SubnetAvailableAnalyze
}

func (a *AnalyzeHostSubnetAvailable) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Subnet Available")
}

func (a *AnalyzeHostSubnetAvailable) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostSubnetAvailable) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "result"
	}

	localPath := fmt.Sprintf("host-collectors/subnetAvailable/%s.json", collectorName)
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

func (a *AnalyzeHostSubnetAvailable) CheckCondition(when string, data []byte) (bool, error) {
	isSubnetAvailable := &collect.SubnetAvailableResult{}
	if err := json.Unmarshal(data, isSubnetAvailable); err != nil {
		return false, errors.Wrap(err, "failed to unmarshal subnetAvailable result")
	}

	return string(isSubnetAvailable.Status) == when, nil
}
