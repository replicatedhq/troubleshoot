package preflightjob

import (
	// "context"
	// "fmt"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	// "gopkg.in/yaml.v2"
	// corev1 "k8s.io/api/core/v1"
	// kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/types"
	// "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ReconcilePreflightJob) reconcilePreflightAnalyzers(instance *troubleshootv1beta1.PreflightJob, preflight *troubleshootv1beta1.Preflight) error {
	for _, analyzer := range preflight.Spec.Analyzers {
		if err := r.reconcileOnePreflightAnalyzer(instance, analyzer); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReconcilePreflightJob) reconcileOnePreflightAnalyzer(instance *troubleshootv1beta1.PreflightJob, analyze *troubleshootv1beta1.Analyze) error {
	if contains(instance.Status.AnalyzersRunning, idForAnalyzer(analyze)) {

		return nil
	}

	return nil
}

func idForAnalyzer(analyzer *troubleshootv1beta1.Analyze) string {
	return ""
}
