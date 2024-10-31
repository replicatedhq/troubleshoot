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

	name := filepath.Join("host-collectors/system", "networkNamespaceConnectivity.json")
	if hostAnalyzer.CollectorName != "" {
		name = filepath.Join("host-collectors/system", hostAnalyzer.CollectorName+".json")
	}

	contents, err := getCollectedFileContents(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get collected file %s: %w", name, err)
	}

	var info collect.NetworkNamespaceConnectivityInfo
	if err := json.Unmarshal(contents, &info); err != nil {
		return nil, fmt.Errorf("failed to unmarshal disk usage info from %s: %w", name, err)
	}

	var results []*AnalyzeResult
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

	return results, nil
}
