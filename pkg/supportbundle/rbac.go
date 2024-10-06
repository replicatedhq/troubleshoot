package supportbundle

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Custom error type for RBAC permission errors
type RBACPermissionError struct {
	Forbidden []error
}

func (e *RBACPermissionError) Error() string {
	return fmt.Sprintf("insufficient permissions: %v", e.Forbidden)
}

func (e *RBACPermissionError) HasErrors() bool {
	return len(e.Forbidden) > 0
}

// checkRBAC checks if the current user has the necessary permissions to run the collectors
func checkRemoteCollectorRBAC(ctx context.Context, clientConfig *rest.Config, title string, namespace string) error {
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create client from config")
	}

	var forbidden []error

	spec := authorizationv1.SelfSubjectAccessReviewSpec{
		ResourceAttributes: &authorizationv1.ResourceAttributes{
			Namespace:   namespace,
			Verb:        "create,delete",
			Group:       "",
			Version:     "",
			Resource:    "pods,configmap",
			Subresource: "",
			Name:        "",
		},
		NonResourceAttributes: nil,
	}

	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: spec,
	}
	resp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to run subject review")
	}

	if !resp.Status.Allowed {
		forbidden = append(forbidden, collect.RBACError{
			DisplayName: title,
			Namespace:   spec.ResourceAttributes.Namespace,
			Resource:    spec.ResourceAttributes.Resource,
			Verb:        spec.ResourceAttributes.Verb,
		})
	}

	if len(forbidden) > 0 {
		return &RBACPermissionError{Forbidden: forbidden}
	}

	return nil
}
