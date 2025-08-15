package collect

import (
	"bytes"
	"context"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type CollectSysctl struct {
	Collector    *troubleshootv1beta2.Sysctl
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectSysctl) Title() string {
	return getCollectorName(c)
}

func (c *CollectSysctl) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectSysctl) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectSysctl) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	if c.Collector.Timeout != "" {
		timeout, err := time.ParseDuration(c.Collector.Timeout)
		if err != nil {
			return nil, errors.Wrap(err, "parse timeout")
		}
		if timeout == 0 {
			timeout = time.Minute
		}
		childCtx, cancel := context.WithTimeout(c.Context, timeout)
		defer cancel()
		c.Context = childCtx
	}

	if c.Collector.Namespace == "" {
		c.Collector.Namespace = c.Namespace
	}
	if c.Collector.Namespace == "" {
		kubeconfig := k8sutil.GetKubeconfig()
		namespace, _, _ := kubeconfig.Namespace()
		c.Collector.Namespace = namespace
	}

	runPodOptions := RunPodOptions{
		Image:           c.Collector.Image,
		ImagePullPolicy: c.Collector.ImagePullPolicy,
		Namespace:       c.Collector.Namespace,
		HostNetwork:     true,
	}

	command := `
find /proc/sys/net/ipv4 -type f | while read f; do v=$(cat $f 2>/dev/null); echo "$f = $v"; done
find /proc/sys/net/bridge -type f | while read f; do v=$(cat $f 2>/dev/null); echo "$f = $v"; done
find /proc/sys/vm -type f | while read f; do v=$(cat $f 2>/dev/null); echo "$f = $v"; done
`
	runPodOptions.Command = []string{"sh", "-c", command}

	if c.Collector.ImagePullSecret != nil {
		runPodOptions.ImagePullSecretName = c.Collector.ImagePullSecret.Name

		if c.Collector.ImagePullSecret.Data != nil {
			secretName, err := createSecret(c.Context, c.Client, c.Collector.Namespace, c.Collector.ImagePullSecret)
			if err != nil {
				return nil, errors.Wrap(err, "create image pull secret")
			}
			defer func() {
				err := c.Client.CoreV1().Secrets(c.Collector.Namespace).Delete(context.Background(), c.Collector.ImagePullSecret.Name, metav1.DeleteOptions{})
				if err != nil && !kuberneteserrors.IsNotFound(err) {
					klog.Errorf("Failed to delete secret %s: %v", c.Collector.ImagePullSecret.Name, err)
				}
			}()

			runPodOptions.ImagePullSecretName = secretName
		}
	}

	results, err := RunPodsReadyNodes(c.Context, c.Client.CoreV1(), runPodOptions)
	if err != nil {
		return nil, err
	}

	output := NewResult()

	for k, v := range results {
		output.SaveResult(c.BundlePath, filepath.Join("sysctl", k), bytes.NewBuffer(v))
	}

	return output, nil
}
