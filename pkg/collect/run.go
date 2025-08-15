package collect

import (
	"context"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectRun struct {
	Collector    *troubleshootv1beta2.Run
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectRun) Title() string {
	return getCollectorName(c)
}

func (c *CollectRun) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectRun) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectRun) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	pullPolicy := corev1.PullIfNotPresent
	if c.Collector.ImagePullPolicy != "" {
		pullPolicy = corev1.PullPolicy(c.Collector.ImagePullPolicy)
	}

	namespace := "default"
	if c.Collector.Namespace != "" {
		namespace = c.Collector.Namespace
	}

	serviceAccountName := "default"
	if c.Collector.ServiceAccountName != "" {
		serviceAccountName = c.Collector.ServiceAccountName
	}

	if err := checkForExistingServiceAccount(c.Context, c.Client, namespace, serviceAccountName); err != nil {
		return nil, err
	}

	runPodSpec := &troubleshootv1beta2.RunPod{
		CollectorMeta: troubleshootv1beta2.CollectorMeta{
			CollectorName: c.Collector.CollectorName,
		},
		Name:            c.Collector.Name,
		Namespace:       namespace,
		Timeout:         c.Collector.Timeout,
		ImagePullSecret: c.Collector.ImagePullSecret,
		PodSpec: corev1.PodSpec{
			RestartPolicy:      corev1.RestartPolicyNever,
			ServiceAccountName: serviceAccountName,
			Containers: []corev1.Container{
				{
					Image:           c.Collector.Image,
					ImagePullPolicy: pullPolicy,
					Name:            "collector",
					Command:         c.Collector.Command,
					Args:            c.Collector.Args,
				},
			},
		},
	}

	rbacErrors := c.GetRBACErrors()
	runPodCollector := &CollectRunPod{runPodSpec, c.BundlePath, c.Namespace, c.ClientConfig, c.Client, c.Context, rbacErrors}

	return runPodCollector.Collect(progressChan)
}
