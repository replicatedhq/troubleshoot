package k8sutil

import (
	"context"

	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func IsNamespacedScopeRBAC(client kubernetes.Interface) (bool, error) {
	ctx := context.Background()

	sar := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace: "",
				Verb:      "list",
				Resource:  "secrets,configmaps",
			},
		},
	}

	resp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return false, err
	}

	if resp.Status.Allowed {
		return true, nil
	} else {
		return false, nil
	}
}
