package collect

import (
	"context"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectEtcd struct {
	Collector    *troubleshootv1beta2.Etcd
	BundlePath   string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectEtcd) Title() string {
	return getCollectorName(c)
}

func (c *CollectEtcd) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectEtcd) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	// TODO
	return nil, nil
}
