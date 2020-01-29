package analyzer

import (
	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

type AnalyzeResult struct {
	IsPass bool
	IsFail bool
	IsWarn bool

	Title   string
	Message string
	URI     string
}

type getCollectedFileContents func(string) ([]byte, error)
type getChildCollectedFileContents func(string) (map[string][]byte, error)

func Analyze(analyzer *troubleshootv1beta1.Analyze, getFile getCollectedFileContents, findFiles getChildCollectedFileContents) (*AnalyzeResult, error) {
	if analyzer.ClusterVersion != nil {
		if analyzer.ClusterVersion.Exclude {
			return nil, nil
		}
		return analyzeClusterVersion(analyzer.ClusterVersion, getFile)
	}
	if analyzer.StorageClass != nil {
		if analyzer.StorageClass.Exclude {
			return nil, nil
		}
		return analyzeStorageClass(analyzer.StorageClass, getFile)
	}
	if analyzer.CustomResourceDefinition != nil {
		if analyzer.CustomResourceDefinition.Exclude {
			return nil, nil
		}
		return analyzeCustomResourceDefinition(analyzer.CustomResourceDefinition, getFile)
	}
	if analyzer.Ingress != nil {
		if analyzer.Ingress.Exclude {
			return nil, nil
		}
		return analyzeIngress(analyzer.Ingress, getFile)
	}
	if analyzer.Secret != nil {
		if analyzer.Secret.Exclude {
			return nil, nil
		}
		return analyzeSecret(analyzer.Secret, getFile)
	}
	if analyzer.ImagePullSecret != nil {
		if analyzer.ImagePullSecret.Exclude {
			return nil, nil
		}
		return analyzeImagePullSecret(analyzer.ImagePullSecret, findFiles)
	}
	if analyzer.DeploymentStatus != nil {
		if analyzer.DeploymentStatus.Exclude {
			return nil, nil
		}
		return analyzeDeploymentStatus(analyzer.DeploymentStatus, getFile)
	}
	if analyzer.StatefulsetStatus != nil {
		if analyzer.StatefulsetStatus.Exclude {
			return nil, nil
		}
		return analyzeStatefulsetStatus(analyzer.StatefulsetStatus, getFile)
	}
	if analyzer.ContainerRuntime != nil {
		if analyzer.ContainerRuntime.Exclude {
			return nil, nil
		}
		return analyzeContainerRuntime(analyzer.ContainerRuntime, getFile)
	}
	if analyzer.Distribution != nil {
		if analyzer.Distribution.Exclude {
			return nil, nil
		}
		return analyzeDistribution(analyzer.Distribution, getFile)
	}
	if analyzer.NodeResources != nil {
		if analyzer.NodeResources.Exclude {
			return nil, nil
		}
		return analyzeNodeResources(analyzer.NodeResources, getFile)
	}
	if analyzer.TextAnalyze != nil {
		return analyzeTextAnalyze(analyzer.TextAnalyze, getFile)
	}

	return nil, errors.New("invalid analyzer")
}
