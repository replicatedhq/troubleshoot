package collect

import (
	"bytes"
	"context"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectInstallers struct {
	Collector    *troubleshootv1beta2.Installers
	BundlePath   string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectInstallers) Title() string {
	return getCollectorName(c)
}

func (c *CollectInstallers) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectInstallers) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create kubernetes client")
	}

	apiResourcesBytes, _ := client.RESTClient().Get().AbsPath("/apis/cluster.kurl.sh/v1beta1/installers/").
		DoRaw(context.TODO())

	collectorName := c.Collector.CollectorName
	if collectorName == "" {
		collectorName = "Installers"
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, fmt.Sprintf("installers/%s.json", collectorName), bytes.NewBuffer(apiResourcesBytes))

	return output, nil
}
