package analyzer

import (
	"fmt"
	"path/filepath"

	longhornv1beta1 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta1"
	longhorntypes "github.com/longhorn/longhorn-manager/types"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"gopkg.in/yaml.v2"
)

func longhorn(analyzer *troubleshootv1beta2.LonghornAnalyze, getCollectedFileContents func(string) ([]byte, error), findFiles func(string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	ns := collect.DefaultLonghornNamespace
	if analyzer.Namespace != "" {
		ns = analyzer.Namespace
	}
	nodesDir := collect.GetLonghornNodesDirectory(ns)
	glob := filepath.Join(nodesDir, "*")
	nodesYaml, err := findFiles(glob)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find longhorn nodes files under %s", nodesDir)
	}
	nodes := []*longhornv1beta1.Node{}
	for key, nodeYaml := range nodesYaml {
		node := &longhornv1beta1.Node{}
		err := yaml.Unmarshal(nodeYaml, node)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal node yaml from %s", key)
		}
		nodes = append(nodes, node)
	}

	results := []*AnalyzeResult{}

	for _, node := range nodes {
		results = append(results, analyzeLonghornNodeSchedulable(node))
	}

	return results, nil
}

func analyzeLonghornNodeSchedulable(node *longhornv1beta1.Node) *AnalyzeResult {
	result := &AnalyzeResult{
		Title: fmt.Sprintf("Longhorn Node Schedulable: %s", node.Name),
	}

	for key, condition := range node.Status.Conditions {
		if key != longhorntypes.NodeConditionTypeSchedulable {
			continue
		}
		if condition.Status == longhorntypes.ConditionStatusFalse {
			result.IsWarn = true
			result.Message = "Longhorn node is not schedulable"
			return result
		} else if condition.Status == longhorntypes.ConditionStatusTrue {
			result.IsPass = true
			result.Message = "Longhorn node is schedulable"
			return result
		}
	}

	result.IsWarn = true
	result.Message = "Node schedulable status not found"

	return result
}
