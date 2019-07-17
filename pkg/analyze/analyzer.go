package analyzer

import (
	"errors"

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

func Analyze(analyzer *troubleshootv1beta1.Analyze, getCollectedFileContents func(string) ([]byte, error)) (*AnalyzeResult, error) {
	if analyzer.ClusterVersion != nil {
		return analyzeClusterVersion(analyzer.ClusterVersion, getCollectedFileContents)
	}

	return nil, errors.New("invalid analyer")
}
