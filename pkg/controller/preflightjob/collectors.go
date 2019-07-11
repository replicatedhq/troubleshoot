package preflightjob

import (
	"fmt"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	// troubleshootclientv1beta1 "github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/typed/troubleshoot/v1beta1"
	// kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "sigs.k8s.io/controller-runtime/pkg/client/config"
)

func (r *ReconcilePreflightJob) reconcilePreflightCollectors(instance *troubleshootv1beta1.PreflightJob, preflight *troubleshootv1beta1.Preflight) error {
	for _, collector := range preflight.Spec.Collectors {
		fmt.Printf("collector = %#v\n", collector)
	}

	return nil
}
