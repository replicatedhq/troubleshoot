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
	Context      context.Context
	RBACErrors
}

func (c *CollectCollectd) Title() string {
	return getCollectorName(c)
}

func (c *CollectCollectd) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectCollectd) SkipRedaction() bool {
	return c.Collector.SkipRedaction
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

	rbacErrors := c.GetRBACErrors()
	copyFromHostCollector := &CollectCopyFromHost{
		Collector:        copyFromHost,
		BundlePath:       c.BundlePath,
		Namespace:        c.Namespace,
		ClientConfig:     c.ClientConfig,
		Client:           c.Client,
		Context:          c.Context,
		RetryFailedMount: false,
		RBACErrors:       rbacErrors,
	}

	return copyFromHostCollector.Collect(progressChan)
}
