package collect

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectSupportBundleMetadata struct {
	Collector    *troubleshootv1beta2.SupportBundleMetadata
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectSupportBundleMetadata) Title() string {
	return getCollectorName(c)
}

func (c *CollectSupportBundleMetadata) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectSupportBundleMetadata) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	output := NewResult()

	secret, err := c.Client.CoreV1().Secrets(c.Collector.Namespace).Get(c.Context, c.Collector.SecretName, metav1.GetOptions{})
	if err != nil {
		return output, errors.Wrapf(err, "failed to get secret %s/%s", c.Collector.Namespace, c.Collector.SecretName)
	}

	metadata := make(map[string]string)
	for k, v := range secret.Data {
		metadata[k] = string(v)
	}

	b, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return output, errors.Wrap(err, "failed to marshal metadata")
	}

	output.SaveResult(c.BundlePath, "metadata/cluster.json", bytes.NewBuffer(b))
	return output, nil
}
