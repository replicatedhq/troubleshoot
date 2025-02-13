package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostSubnetContainsIP struct {
	hostAnalyzer *troubleshootv1beta2.SubnetContainsIPAnalyze
}

func (a *AnalyzeHostSubnetContainsIP) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Subnet Contains IP")
}

func (a *AnalyzeHostSubnetContainsIP) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostSubnetContainsIP) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	collectorName := a.hostAnalyzer.CollectorName
	if collectorName == "" {
		collectorName = "result"
	}

	const nodeBaseDir = "host-collectors/subnetContainsIP"
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
		return nil, errors.Wrap(err, "failed to analyze Subnet Contains IP")
	}

	return results, nil
}

func (a *AnalyzeHostSubnetContainsIP) CheckCondition(when string, data []byte) (bool, error) {
	var result collect.SubnetContainsIPResult
	if err := json.Unmarshal(data, &result); err != nil {
		return false, errors.Wrap(err, "failed to unmarshal subnetContainsIP result")
	}

	switch when {
	case "true":
		return result.Contains, nil
	case "false":
		return !result.Contains, nil
	}

	return false, errors.Errorf("unknown condition: %q", when)
}
