package collect

import (
	"bytes"
	"context"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Sysctl(ctx context.Context, c *Collector, client kubernetes.Interface, collector *troubleshootv1beta2.Sysctl) (CollectorResult, error) {

	if collector.Timeout != "" {
		timeout, err := time.ParseDuration(collector.Timeout)
		if err != nil {
			return nil, errors.Wrap(err, "parse timeout")
		}
		if timeout == 0 {
			timeout = time.Minute
		}
		childCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		ctx = childCtx
	}

	runPodOptions := RunPodOptions{
		Image:           collector.Image,
		ImagePullPolicy: collector.ImagePullPolicy,
		Namespace:       collector.Namespace,
		HostNetwork:     true,
	}

	command := `
find /proc/sys/net/ipv4 -type f | while read f; do v=$(cat $f 2>/dev/null); echo "$f = $v"; done
find /proc/sys/net/bridge -type f | while read f; do v=$(cat $f 2>/dev/null); echo "$f = $v"; done
`
	runPodOptions.Command = []string{"sh", "-c", command}

	if collector.ImagePullSecret != nil {
		runPodOptions.ImagePullSecretName = collector.ImagePullSecret.Name

		if collector.ImagePullSecret.Data != nil {
			secretName, err := createSecret(ctx, client, collector.Namespace, collector.ImagePullSecret)
			if err != nil {
				return nil, errors.Wrap(err, "create image pull secret")
			}
			defer func() {
				err := client.CoreV1().Secrets(collector.Namespace).Delete(ctx, collector.ImagePullSecret.Name, metav1.DeleteOptions{})
				if err != nil && !kuberneteserrors.IsNotFound(err) {
					logger.Printf("Failed to delete secret %s: %v", collector.ImagePullSecret.Name, err)
				}
			}()

			runPodOptions.ImagePullSecretName = secretName
		}
	}

	results, err := RunPodsReadyNodes(ctx, client.CoreV1(), runPodOptions)
	if err != nil {
		return nil, err
	}

	output := NewResult()

	for k, v := range results {
		output.SaveResult(c.BundlePath, filepath.Join("sysctl", k), bytes.NewBuffer(v))
	}

	return output, nil
}
