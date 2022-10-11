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
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "CollectD")
}

func (c *CollectCollectd) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
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
	copyFromHostCollector := &CollectCopyFromHost{copyFromHost, c.BundlePath, c.Namespace, c.ClientConfig, c.Client, c.Context, rbacErrors}

	return copyFromHostCollector.Collect(progressChan)
}
