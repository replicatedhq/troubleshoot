package collect

import (
	"context"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
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

func (c *CollectCollectd) GetRBACErrors() []error {
	return c.RBACErrors
}

func (c *CollectCollectd) HasRBACErrors() bool {
	return len(c.RBACErrors) > 0
}

func (c *CollectCollectd) CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error {
	exclude, err := c.IsExcluded()
	if err != nil || exclude != true {
		return nil
	}

	rbacErrors, err := checkRBAC(ctx, c.ClientConfig, c.Namespace, c.Title(), collector)
	if err != nil {
		return err
	}

	c.RBACErrors = rbacErrors

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
