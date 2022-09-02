package collect

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type RBACError struct {
	DisplayName string
	Namespace   string
	Resource    string
	Verb        string
}

func (e RBACError) Error() string {
	if e.Namespace == "" {
		return fmt.Sprintf("cannot collect %s: action %q is not allowed on resource %q at the cluster scope", e.DisplayName, e.Verb, e.Resource)
	}
	return fmt.Sprintf("cannot collect %s: action %q is not allowed on resource %q in the %q namespace", e.DisplayName, e.Verb, e.Resource, e.Namespace)
}

func IsRBACError(err error) bool {
	_, ok := errors.Cause(err).(RBACError)
	return ok
}

func checkRBAC(ctx context.Context, clientConfig *rest.Config, namespace string, title string, collector *troubleshootv1beta2.Collect) ([]error, error) {
	client, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client from config")
	}

	forbidden := make([]error, 0)

	specs := collector.AccessReviewSpecs(namespace)
	for _, spec := range specs {
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: spec,
		}

		resp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "failed to run subject review")
		}

		if !resp.Status.Allowed { // all other fields of Status are empty...
			forbidden = append(forbidden, RBACError{
				DisplayName: title,
				Namespace:   spec.ResourceAttributes.Namespace,
				Resource:    spec.ResourceAttributes.Resource,
				Verb:        spec.ResourceAttributes.Verb,
			})
		}
	}

	return forbidden, nil
}
