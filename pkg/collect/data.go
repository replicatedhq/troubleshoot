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
	Context      context.Context
	RBACErrors
}

func (c *CollectData) Title() string {
	return getCollectorName(c)
}

func (c *CollectData) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectData) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectData) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	bundlePath := filepath.Join(c.Collector.Name, c.Collector.CollectorName)

	output := NewResult()
	output.SaveResult(c.BundlePath, bundlePath, bytes.NewBuffer([]byte(c.Collector.Data)))

	return output, nil
}
