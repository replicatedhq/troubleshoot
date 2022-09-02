package collect

import (
	"bytes"
	"context"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectData struct {
	Collector    *troubleshootv1beta2.Data
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	ctx          context.Context
	RBACErrors   []error
}

func (c *CollectData) Title() string {
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "Data")
}

func (c *CollectData) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectData) CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error {
	exclude, err := c.IsExcluded()
	if err != nil || exclude != true {
		return nil
	}

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create client from config")
	}

	forbidden := make([]error, 0)

	specs := collector.AccessReviewSpecs(c.Namespace)
	for _, spec := range specs {
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: spec,
		}

		resp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to run subject review")
		}

		if !resp.Status.Allowed { // all other fields of Status are empty...
			forbidden = append(forbidden, RBACError{
				DisplayName: c.Title(),
				Namespace:   spec.ResourceAttributes.Namespace,
				Resource:    spec.ResourceAttributes.Resource,
				Verb:        spec.ResourceAttributes.Verb,
			})
		}
	}
	c.RBACErrors = forbidden

	return nil
}

func (c *CollectData) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	bundlePath := filepath.Join(c.Collector.Name, c.Collector.CollectorName)

	output := NewResult()
	output.SaveResult(c.BundlePath, bundlePath, bytes.NewBuffer([]byte(c.Collector.Data)))

	return output, nil
}
