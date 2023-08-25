package specs

import (
	"context"

	specs "github.com/replicatedhq/troubleshoot/internal/specs"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// SplitTroubleshootSecretLabelSelector splits a label selector into two selectors, if applicable:
// 1. troubleshoot.io/kind=support-bundle and non-troubleshoot (if contains) labels selector.
// 2. troubleshoot.sh/kind=support-bundle and non-troubleshoot (if contains) labels selector.
// Deprecated: Remove in a future version (v1.0). Future loader functions will be created
func SplitTroubleshootSecretLabelSelector(client kubernetes.Interface, labelSelector labels.Selector) ([]string, error) {
	return specs.SplitTroubleshootSecretLabelSelector(context.TODO(), labelSelector)
}
