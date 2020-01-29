package analyzer

import (
	"encoding/json"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func analyzeNoderesources(analyzer *troubleshootv1beta1.NodeResources, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	collected, err := getCollectedFileContents("cluster-info/nods.json")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get contents of nodes.json")
	}

	var nodes []corev1.Node
	if err := json.Unmarshal(collected, &nodes); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal node list")
	}

	matchingNodeCount := 0

	for _, node := range nodes {
		matches, err := nodeMatchesFilters(node, analyzer.Filters)
		if err != nil {
			return nil, errors.Wrap(err, "failed to check if node matches filter")
		}
	}
	return analyzeClusterVersionResult(k8sVersion, analyzer.Outcomes, analyzer.CheckName)
}

func nodeMatchesFilters(node *corev1.Node, filters *troubleshootv1beta1.NodeResourceFilters) (bool, error) {
	if filters == nil {
		return true, nil
	}

	return false, nil
}
