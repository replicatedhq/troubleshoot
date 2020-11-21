package analyzer

import (
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

// blah
type AnalyzeResult struct {
	IsPass bool
	IsFail bool
	IsWarn bool

	Title   string
	Message string
	URI     string
	IconKey string
	IconURI string
}

type getCollectedFileContents func(string) ([]byte, error)
type getChildCollectedFileContents func(string) (map[string][]byte, error)

func isExcluded(excludeVal multitype.BoolOrString) (bool, error) {
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

// blah
func Analyze(analyzer *troubleshootv1beta2.Analyze, getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error) {
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
		result, err := analyzeDeploymentStatus(analyzer.DeploymentStatus, getFile)
		if err != nil {
			return nil, err
		}
		return []*AnalyzeResult{result}, nil
	}
	if analyzer.StatefulsetStatus != nil {
		isExcluded, err := isExcluded(analyzer.StatefulsetStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		result, err := analyzeStatefulsetStatus(analyzer.StatefulsetStatus, getFile)
		if err != nil {
			return nil, err
		}
		return []*AnalyzeResult{result}, nil
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
		multiResult, err := analyzeTextAnalyze(analyzer.TextAnalyze, findFiles)
		if err != nil {
			return nil, err
		}
		return multiResult, nil
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
		return []*AnalyzeResult{result}, nil
	}
	return nil, errors.New("invalid analyzer")

}
