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
		return analyzeClusterVersion(analyzer.ClusterVersion, getFile)
	}
	if analyzer.StorageClass != nil {
		return analyzeStorageClass(analyzer.StorageClass, getFile)
	}
	if analyzer.CustomResourceDefinition != nil {
		return analyzeCustomResourceDefinition(analyzer.CustomResourceDefinition, getFile)
	}
	if analyzer.Ingress != nil {
		return analyzeIngress(analyzer.Ingress, getFile)
	}
	if analyzer.Secret != nil {
		return analyzeSecret(analyzer.Secret, getFile)
	}
	if analyzer.ImagePullSecret != nil {
		return analyzeImagePullSecret(analyzer.ImagePullSecret, findFiles)
	}
	if analyzer.DeploymentStatus != nil {
		return deploymentStatus(analyzer.DeploymentStatus, getFile)
	}
	if analyzer.StatefulsetStatus != nil {
		return statefulsetStatus(analyzer.StatefulsetStatus, getFile)
	}

	return nil, errors.New("invalid analyzer")
}
