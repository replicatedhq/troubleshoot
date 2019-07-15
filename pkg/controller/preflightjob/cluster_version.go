package preflightjob

import (
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

func (r *ReconcilePreflightJob) analyzeClusterVersion(instance *troubleshootv1beta1.PreflightJob, clusterVersion *troubleshootv1beta1.ClusterVersion) (*AnalysisResult, error) {
	return &AnalysisResult{
		Success: true,
	}, nil
}
