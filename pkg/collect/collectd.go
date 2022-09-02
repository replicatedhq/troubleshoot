package collect

import (
	"context"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectCollectd struct {
	Collector    *troubleshootv1beta2.Collectd
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	ctx          context.Context
	RBACErrors   []error
}

func (c *CollectCollectd) Title() string {
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "CollectD")
}

func (c *CollectCollectd) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectCollectd) CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error {
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

func (c *CollectCollectd) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	copyFromHost := &troubleshootv1beta2.CopyFromHost{
		CollectorMeta:   c.Collector.CollectorMeta,
		Name:            "collectd/rrd",
		Namespace:       c.Collector.Namespace,
		Image:           c.Collector.Image,
		ImagePullPolicy: c.Collector.ImagePullPolicy,
		ImagePullSecret: c.Collector.ImagePullSecret,
		Timeout:         c.Collector.Timeout,
		HostPath:        c.Collector.HostPath,
	}

	copyFromHostCollector := &CollectCopyFromHost{copyFromHost, c.BundlePath, c.Namespace, c.ClientConfig, c.Client, c.ctx, c.RBACErrors}

	return copyFromHostCollector.Collect(progressChan)
}
