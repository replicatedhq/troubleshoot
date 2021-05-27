package analyzer

import (
	"bufio"
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	longhornv1beta1 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta1"
	longhorntypes "github.com/longhorn/longhorn-manager/types"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/redact"
	"gopkg.in/yaml.v2"
)

func longhorn(analyzer *troubleshootv1beta2.LonghornAnalyze, getCollectedFileContents func(string) ([]byte, error), findFiles func(string) (map[string][]byte, error)) ([]*AnalyzeResult, error) {
	ns := collect.DefaultLonghornNamespace
	if analyzer.Namespace != "" {
		ns = analyzer.Namespace
	}

	// get nodes.longhorn.io
	nodesDir := collect.GetLonghornNodesDirectory(ns)
	nodesGlob := filepath.Join(nodesDir, "*")
	nodesYaml, err := findFiles(nodesGlob)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find longhorn nodes files under %s", nodesDir)
	}
	nodes := []*longhornv1beta1.Node{}
	for key, nodeYaml := range nodesYaml {
		nodeYaml = stripRedactedLines(nodeYaml)
		node := &longhornv1beta1.Node{}
		err := yaml.Unmarshal(nodeYaml, node)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal node yaml from %s", key)
		}
		nodes = append(nodes, node)
	}

	// get replicas.longhorn.io
	replicasDir := collect.GetLonghornReplicasDirectory(ns)
	replicasGlob := filepath.Join(replicasDir, "*")
	replicasYaml, err := findFiles(replicasGlob)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find longhorn replicas files under %s", replicasDir)
	}
	replicas := []*longhornv1beta1.Replica{}
	for key, replicaYaml := range replicasYaml {
		replicaYaml = stripRedactedLines(replicaYaml)
		replica := &longhornv1beta1.Replica{}
		err := yaml.Unmarshal(replicaYaml, replica)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal replica yaml from %s", key)
		}
		replicas = append(replicas, replica)
	}

	results := []*AnalyzeResult{}

	for _, node := range nodes {
		results = append(results, analyzeLonghornNodeSchedulable(node))
	}

	for _, replica := range replicas {
		results = append(results, analyzeLonghornReplica(replica))
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

func analyzeLonghornReplica(replica *longhornv1beta1.Replica) *AnalyzeResult {
	result := &AnalyzeResult{
		Title: fmt.Sprintf("Longhorn Replica: %s", replica.Name),
	}

	if replica.Spec.FailedAt != "" {
		result.IsWarn = true
		result.Message = fmt.Sprintf("Longhorn replica %s failed at %s", replica.Name, replica.Spec.FailedAt)
		return result
	}

	desired := replica.Spec.InstanceSpec.DesireState
	actual := replica.Status.InstanceStatus.CurrentState

	if desired != actual {
		result.IsWarn = true
		result.Message = fmt.Sprintf("Longhorn replica %s current status %q, should be %q", replica.Name, actual, desired)
		return result
	}

	result.IsPass = true
	result.Message = fmt.Sprintf("Replica is %s", actual)

	return result
}

func stripRedactedLines(yaml []byte) []byte {
	buf := bytes.NewBuffer(yaml)
	scanner := bufio.NewScanner(buf)

	out := []byte{}

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), redact.MASK_TEXT) {
			continue
		}
		out = append(out, scanner.Bytes()...)
		out = append(out, '\n')
	}

	return out
}
