package analyzer

import (
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	longhornv1beta1 "github.com/replicatedhq/troubleshoot/pkg/longhorn/apis/longhorn/v1beta1"
	longhorntypes "github.com/replicatedhq/troubleshoot/pkg/longhorn/types"
	"gopkg.in/yaml.v2"
)

type AnalyzeLonghorn struct {
	analyzer *troubleshootv1beta2.LonghornAnalyze
}

func (a *AnalyzeLonghorn) Title() string {
	return "Longhorn analyzer"
}

func (a *AnalyzeLonghorn) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

func (a *AnalyzeLonghorn) Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	results, err := longhorn(a.analyzer, getFile, findFiles)
	if err != nil {
		return nil, err
	}
	for i := range results {
		results[i].Strict = a.analyzer.Strict.BoolOrDefaultFalse()
	}
	return results, nil
}

func longhorn(analyzer *troubleshootv1beta2.LonghornAnalyze, getFileContents getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	ns := collect.DefaultLonghornNamespace
	if analyzer.Namespace != "" {
		ns = analyzer.Namespace
	}

	excludeFiles := []string{}
	// get nodes.longhorn.io
	nodesDir := collect.GetLonghornNodesDirectory(ns)
	nodesGlob := filepath.Join(nodesDir, "*")
	nodesYaml, err := findFiles(nodesGlob, excludeFiles)
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
	replicasYaml, err := findFiles(replicasGlob, excludeFiles)
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

	// get engines.longhorn.io
	enginesDir := collect.GetLonghornEnginesDirectory(ns)
	enginesGlob := filepath.Join(enginesDir, "*")
	enginesYaml, err := findFiles(enginesGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find longhorn engines files under %s", enginesDir)
	}
	engines := []*longhornv1beta1.Engine{}
	for key, engineYaml := range enginesYaml {
		engineYaml = stripRedactedLines(engineYaml)
		engine := &longhornv1beta1.Engine{}
		err := yaml.Unmarshal(engineYaml, engine)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal engine yaml from %s", key)
		}
		engines = append(engines, engine)
	}

	// get volumes.longhorn.io
	volumesDir := collect.GetLonghornVolumesDirectory(ns)
	volumesGlob := filepath.Join(volumesDir, "*.yaml")
	volumesYaml, err := findFiles(volumesGlob, excludeFiles)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to find longhorn volumes files under %s", volumesDir)
	}
	volumes := []*longhornv1beta1.Volume{}
	for key, volumeYaml := range volumesYaml {
		volumeYaml = stripRedactedLines(volumeYaml)
		volume := &longhornv1beta1.Volume{}
		err := yaml.Unmarshal(volumeYaml, volume)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to unmarshal volume yaml from %s", key)
		}
		volumes = append(volumes, volume)
	}

	results := []*AnalyzeResult{}

	for _, node := range nodes {
		results = append(results, analyzeLonghornNodeSchedulable(node))
	}

	for _, replica := range replicas {
		results = append(results, analyzeLonghornReplica(replica))
	}

	for _, engine := range engines {
		results = append(results, analyzeLonghornEngine(engine))
	}

	for _, volume := range volumes {
		// get replica checksums for each volume if provided
		checksumsGlob := filepath.Join(volumesDir, volume.Name, "replicachecksums", "*")
		checksumFiles, err := findFiles(checksumsGlob, excludeFiles)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to find longhorn replica checksums under %s", checksumsGlob)
		}

		checksums := []map[string]string{}
		for key, checksumTxt := range checksumFiles {
			checksum, err := collect.ParseReplicaChecksum(checksumTxt)
			if err != nil {
				return nil, errors.Wrapf(err, "Failed to parse %s", key)
			}
			checksums = append(checksums, checksum)
		}

		if len(checksums) > 1 {
			results = append(results, analyzeLonghornReplicaChecksums(volume.Name, checksums))
		}

		// Check Volume replicas
		if volume.Spec.NumberOfReplicas < 2 {
			result := &AnalyzeResult{
				Title:   "Longhorn volume with low replicas",
				IsWarn:  true,
				Message: fmt.Sprintf("Longhorn volume %s has less than two replicas, this could lead to issues with volume availability", volume.Name),
			}
			results = append(results, result)
		}

	}

	return simplifyLonghornResults(results), nil
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

func analyzeLonghornEngine(engine *longhornv1beta1.Engine) *AnalyzeResult {
	result := &AnalyzeResult{
		Title: fmt.Sprintf("Longhorn Engine: %s", engine.Name),
	}

	desired := engine.Spec.InstanceSpec.DesireState
	actual := engine.Status.InstanceStatus.CurrentState

	if desired != actual {
		result.IsWarn = true
		result.Message = fmt.Sprintf("Longhorn engine %s current status %q, should be %q", engine.Name, actual, desired)
		return result
	}

	result.IsPass = true
	result.Message = fmt.Sprintf("Engine is %s", actual)

	return result
}

func analyzeLonghornReplicaChecksums(volumeName string, checksums []map[string]string) *AnalyzeResult {
	result := &AnalyzeResult{
		Title: fmt.Sprintf("Longhorn Volume Replica Corruption: %s", volumeName),
	}

	for i, checksum := range checksums {
		if i == 0 {
			continue
		}
		prior := checksums[i-1]
		if !reflect.DeepEqual(prior, checksum) {
			result.IsWarn = true
			result.Message = "Replica corruption detected"
			return result
		}
	}

	result.IsPass = true
	result.Message = "No replica corruption detected"

	return result
}

// Keep warn/error results. Return a single pass result if there are no warn/errors.
func simplifyLonghornResults(results []*AnalyzeResult) []*AnalyzeResult {
	out := []*AnalyzeResult{}
	resultPass := false
	for _, result := range results {
		if result.IsPass {
			resultPass = true
			continue
		}
		out = append(out, result)
	}

	if resultPass && len(out) == 0 {
		out = append(out, &AnalyzeResult{
			Title:   "Longhorn Health Status",
			IsPass:  true,
			Message: "Longhorn is healthy",
		})
	}

	return out
}
