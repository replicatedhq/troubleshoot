package collect

import (
	"bytes"
	"context"
	"path/filepath"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
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

func (c *CollectData) GetRBACErrors() []error {
	return c.RBACErrors
}

func (c *CollectData) HasRBACErrors() bool {
	return len(c.RBACErrors) > 0
}

func (c *CollectData) CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error {
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

func (c *CollectData) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	bundlePath := filepath.Join(c.Collector.Name, c.Collector.CollectorName)

	output := NewResult()
	output.SaveResult(c.BundlePath, bundlePath, bytes.NewBuffer([]byte(c.Collector.Data)))

	return output, nil
}
