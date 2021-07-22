package collect

import (
	"context"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
)

func Collectd(ctx context.Context, namespace string, clientConfig *restclient.Config, client kubernetes.Interface, collector *troubleshootv1beta2.Collectd) (map[string][]byte, error) {
	return CopyFromHost(ctx, namespace, clientConfig, client, &troubleshootv1beta2.CopyFromHost{
		CollectorMeta:   collector.CollectorMeta,
		Name:            "collectd/rrd",
		Namespace:       collector.Namespace,
		Image:           collector.Image,
		ImagePullPolicy: collector.ImagePullPolicy,
		ImagePullSecret: collector.ImagePullSecret,
		Timeout:         collector.Timeout,
		HostPath:        collector.HostPath,
	})
}
