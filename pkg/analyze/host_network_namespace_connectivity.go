package analyzer

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

type AnalyzeHostNetworkNamespaceConnectivity struct {
	hostAnalyzer *troubleshootv1beta2.NetworkNamespaceConnectivityAnalyze
}

func (a *AnalyzeHostNetworkNamespaceConnectivity) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Network Namespace Connectivity")
}

func (a *AnalyzeHostNetworkNamespaceConnectivity) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostNetworkNamespaceConnectivity) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	collectedPath := filepath.Join("host-collectors/system", "networkNamespaceConnectivity.json")
	fileName := "networkNamespaceConnectivity.json"
	if hostAnalyzer.CollectorName != "" {
		collectedPath = filepath.Join("host-collectors/system", hostAnalyzer.CollectorName+".json")
		fileName = hostAnalyzer.CollectorName + ".json"
	}

	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		collectedPath,
		collect.NodeInfoBaseDir,
		fileName,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve collected contents: %w", err)
	}

	var results []*AnalyzeResult
	for _, collected := range collectedContents {
		var info collect.NetworkNamespaceConnectivityInfo
		if err := json.Unmarshal(collected.Data, &info); err != nil {
			return nil, fmt.Errorf("failed to unmarshal disk usage info: %w", err)
		}

		for _, outcome := range hostAnalyzer.Outcomes {
			result := &AnalyzeResult{Title: a.Title()}

			if outcome.Pass != nil && info.Success {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				results = append(results, result)
				break
			}

			if outcome.Fail != nil && !info.Success {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				results = append(results, result)
				break
			}
		}
	}

	return results, nil
}
