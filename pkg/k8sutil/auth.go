package k8sutil

import (
	"context"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CanIListAndGetAllSecretsAndConfigMaps checks if the current user can list and get secrets and configmaps
// from all namespaces
func CanIListAndGetAllSecretsAndConfigMaps(ctx context.Context, client kubernetes.Interface) (bool, error) {
	canis := []struct{ ns, verb, resource string }{
		{"", "get", "secrets"},
		{"", "get", "configmaps"},
		{"", "list", "secrets"},
		{"", "list", "configmaps"},
	}

	for _, cani := range canis {
		ican, err := authCanI(ctx, client, cani.ns, cani.verb, cani.resource)
		if err != nil {
			return false, err
		}

		if !ican {
			return false, nil
		}
	}

	return true, nil
}

func authCanI(ctx context.Context, client kubernetes.Interface, ns, verb, resource string) (bool, error) {
	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: ns,
				Verb:      verb,
				Resource:  resource,
			},
		},
	}

	resp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}

	return resp.Status.Allowed, nil
}
