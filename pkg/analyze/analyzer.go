package analyzer

import (
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

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

	parsed, err := strconv.ParseBool(excludeVal.StrVal)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse bool string")
	}

	return parsed, nil
}

func Analyze(analyzer *troubleshootv1beta1.Analyze, getFile getCollectedFileContents, findFiles getChildCollectedFileContents) (*AnalyzeResult, error) {
	if analyzer.ClusterVersion != nil {
		isExcluded, err := isExcluded(analyzer.ClusterVersion.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeClusterVersion(analyzer.ClusterVersion, getFile)
	}
	if analyzer.StorageClass != nil {
		isExcluded, err := isExcluded(analyzer.StorageClass.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeStorageClass(analyzer.StorageClass, getFile)
	}
	if analyzer.CustomResourceDefinition != nil {
		isExcluded, err := isExcluded(analyzer.CustomResourceDefinition.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeCustomResourceDefinition(analyzer.CustomResourceDefinition, getFile)
	}
	if analyzer.Ingress != nil {
		isExcluded, err := isExcluded(analyzer.Ingress.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeIngress(analyzer.Ingress, getFile)
	}
	if analyzer.Secret != nil {
		isExcluded, err := isExcluded(analyzer.Secret.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeSecret(analyzer.Secret, getFile)
	}
	if analyzer.ImagePullSecret != nil {
		isExcluded, err := isExcluded(analyzer.ImagePullSecret.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeImagePullSecret(analyzer.ImagePullSecret, findFiles)
	}
	if analyzer.DeploymentStatus != nil {
		isExcluded, err := isExcluded(analyzer.DeploymentStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeDeploymentStatus(analyzer.DeploymentStatus, getFile)
	}
	if analyzer.StatefulsetStatus != nil {
		isExcluded, err := isExcluded(analyzer.StatefulsetStatus.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeStatefulsetStatus(analyzer.StatefulsetStatus, getFile)
	}
	if analyzer.ContainerRuntime != nil {
		isExcluded, err := isExcluded(analyzer.ContainerRuntime.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeContainerRuntime(analyzer.ContainerRuntime, getFile)
	}
	if analyzer.Distribution != nil {
		isExcluded, err := isExcluded(analyzer.Distribution.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeDistribution(analyzer.Distribution, getFile)
	}
	if analyzer.NodeResources != nil {
		isExcluded, err := isExcluded(analyzer.NodeResources.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeNodeResources(analyzer.NodeResources, getFile)
	}
	if analyzer.TextAnalyze != nil {
		isExcluded, err := isExcluded(analyzer.TextAnalyze.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzeTextAnalyze(analyzer.TextAnalyze, getFile)
	}
	if analyzer.PostgresAnalyze != nil {
		isExcluded, err := isExcluded(analyzer.PostgresAnalyze.Exclude)
		if err != nil {
			return nil, err
		}
		if isExcluded {
			return nil, nil
		}
		return analyzePostgresAnalyze(analyzer.PostgresAnalyze, getFile)
	}

	return nil, errors.New("invalid analyzer")
}
