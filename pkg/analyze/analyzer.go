package analyzer

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	corev1 "k8s.io/api/core/v1"
)

type AnalyzeResult struct {
	IsPass bool
	IsFail bool
	IsWarn bool
	Strict bool

	Title   string
	Message string
	URI     string
	IconKey string
	IconURI string

	InvolvedObject *corev1.ObjectReference
}

type getCollectedFileContents func(string) ([]byte, error)
type getChildCollectedFileContents func(string) (map[string][]byte, error)

func isExcluded(excludeVal *multitype.BoolOrString) (bool, error) {
	if excludeVal == nil {
		return false, nil
	}

	if excludeVal.Type == multitype.Bool {
		return excludeVal.BoolVal, nil
	}

	if excludeVal.StrVal == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(excludeVal.StrVal)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse bool string")
	}

	return parsed, nil
}

func HostAnalyze(hostAnalyzer *troubleshootv1beta2.HostAnalyze, getFile getCollectedFileContents, findFiles getChildCollectedFileContents) []*AnalyzeResult {
	analyzer, ok := GetHostAnalyzer(hostAnalyzer)
	if !ok {
		return NewAnalyzeResultError(analyzer, errors.New("invalid host analyzer"))
	}

	isExcluded, _ := analyzer.IsExcluded()
	if isExcluded {
		return nil
	}

	result, err := analyzer.Analyze(getFile)
	if err != nil {
		return NewAnalyzeResultError(analyzer, errors.Wrap(err, "analyze"))
	}
	return result
}

func NewAnalyzeResultError(analyzer HostAnalyzer, err error) []*AnalyzeResult {
	if analyzer != nil {
		return []*AnalyzeResult{{
			IsFail:  true,
			Title:   analyzer.Title(),
			Message: fmt.Sprintf("Analyzer Failed: %v", err),
		}}
	}
	return []*AnalyzeResult{{
		IsFail:  true,
		Title:   "nil analyzer",
		Message: fmt.Sprintf("Analyzer Failed: %v", err),
	}}
}

func Analyze(analyzer *troubleshootv1beta2.Analyze, getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
	if analyzer == nil {
		return nil, errors.New("nil analyzer")
	}

	if analyzer.ClusterVersion != nil {
		isExcluded, err := isExcluded(analyzer.ClusterVersion.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeClusterVersion(analyzer.ClusterVersion, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.ClusterVersion.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.StorageClass != nil {
		isExcluded, err := isExcluded(analyzer.StorageClass.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeStorageClass(analyzer.StorageClass, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.StorageClass.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.CustomResourceDefinition != nil {
		isExcluded, err := isExcluded(analyzer.CustomResourceDefinition.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeCustomResourceDefinition(analyzer.CustomResourceDefinition, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.CustomResourceDefinition.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Ingress != nil {
		isExcluded, err := isExcluded(analyzer.Ingress.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeIngress(analyzer.Ingress, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Ingress.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Secret != nil {
		isExcluded, err := isExcluded(analyzer.Secret.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeSecret(analyzer.Secret, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Secret.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.ConfigMap != nil {
		isExcluded, err := isExcluded(analyzer.ConfigMap.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeConfigMap(analyzer.ConfigMap, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.ConfigMap.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.ImagePullSecret != nil {
		isExcluded, err := isExcluded(analyzer.ImagePullSecret.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeImagePullSecret(analyzer.ImagePullSecret, findFiles)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.ImagePullSecret.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.DeploymentStatus != nil {
		isExcluded, err := isExcluded(analyzer.DeploymentStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := analyzeDeploymentStatus(analyzer.DeploymentStatus, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.DeploymentStatus.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.StatefulsetStatus != nil {
		isExcluded, err := isExcluded(analyzer.StatefulsetStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := analyzeStatefulsetStatus(analyzer.StatefulsetStatus, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.StatefulsetStatus.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.JobStatus != nil {
		isExcluded, err := isExcluded(analyzer.JobStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := analyzeJobStatus(analyzer.JobStatus, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.JobStatus.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.ReplicaSetStatus != nil {
		isExcluded, err := isExcluded(analyzer.ReplicaSetStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := analyzeReplicaSetStatus(analyzer.ReplicaSetStatus, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.ReplicaSetStatus.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.ClusterPodStatuses != nil {
		isExcluded, err := isExcluded(analyzer.ClusterPodStatuses.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := clusterPodStatuses(analyzer.ClusterPodStatuses, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.ClusterPodStatuses.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.ContainerRuntime != nil {
		isExcluded, err := isExcluded(analyzer.ContainerRuntime.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeContainerRuntime(analyzer.ContainerRuntime, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.ContainerRuntime.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Distribution != nil {
		isExcluded, err := isExcluded(analyzer.Distribution.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeDistribution(analyzer.Distribution, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Distribution.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.NodeResources != nil {
		isExcluded, err := isExcluded(analyzer.NodeResources.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeNodeResources(analyzer.NodeResources, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.NodeResources.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.TextAnalyze != nil {
		isExcluded, err := isExcluded(analyzer.TextAnalyze.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := analyzeTextAnalyze(analyzer.TextAnalyze, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.TextAnalyze.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}
	if analyzer.Postgres != nil {
		isExcluded, err := isExcluded(analyzer.Postgres.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzePostgres(analyzer.Postgres, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Postgres.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Mysql != nil {
		isExcluded, err := isExcluded(analyzer.Mysql.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeMysql(analyzer.Mysql, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Mysql.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Redis != nil {
		isExcluded, err := isExcluded(analyzer.Redis.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeRedis(analyzer.Redis, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.Redis.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.CephStatus != nil {
		isExcluded, err := isExcluded(analyzer.CephStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := cephStatus(analyzer.CephStatus, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.CephStatus.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.Longhorn != nil {
		isExcluded, err := isExcluded(analyzer.Longhorn.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := longhorn(analyzer.Longhorn, getFile, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.Longhorn.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}

	if analyzer.RegistryImages != nil {
		isExcluded, err := isExcluded(analyzer.RegistryImages.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeRegistry(analyzer.RegistryImages, getFile)
		if err != nil {
			return nil, err
		}
		result.Strict = analyzer.RegistryImages.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}

	if analyzer.WeaveReport != nil {
		isExcluded, err := isExcluded(analyzer.WeaveReport.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		results, err := analyzeWeaveReport(analyzer.WeaveReport, findFiles)
		if err != nil {
			return nil, err
		}
		for i := range results {
			results[i].Strict = analyzer.WeaveReport.Strict.BoolOrDefaultFalse()
		}
		return results, nil
	}

	if analyzer.Sysctl != nil {
		isExcluded, err := isExcluded(analyzer.Sysctl.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeSysctl(analyzer.Sysctl, findFiles)
		if err != nil {
			return nil, err
		}
		if result == nil {
			return []*AnalyzeResult{}, nil
		}
		result.Strict = analyzer.Sysctl.Strict.BoolOrDefaultFalse()
		return []*AnalyzeResult{result}, nil
	}

	return nil, errors.New("invalid analyzer")
}
