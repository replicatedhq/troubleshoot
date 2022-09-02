package collect

import (
	"context"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectRun struct {
	Collector    *troubleshootv1beta2.Run
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	ctx          context.Context
	RBACErrors   []error
}

func (c *CollectRun) Title() string {
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "Run")
}

func (c *CollectRun) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectRun) CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error {
	exclude, err := c.IsExcluded()
	if err != nil || exclude != true {
		return nil
	}

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create client from config")
	}

	forbidden := make([]error, 0)

	specs := collector.AccessReviewSpecs(c.Namespace)
	for _, spec := range specs {
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: spec,
		}

		resp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to run subject review")
		}

		if !resp.Status.Allowed { // all other fields of Status are empty...
			forbidden = append(forbidden, RBACError{
				DisplayName: c.Title(),
				Namespace:   spec.ResourceAttributes.Namespace,
				Resource:    spec.ResourceAttributes.Resource,
				Verb:        spec.ResourceAttributes.Verb,
			})
		}
	}
	c.RBACErrors = forbidden

	return nil
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

	runPodCollector := &CollectRunPod{runPodSpec, c.BundlePath, c.Namespace, c.ClientConfig, c.Client, c.ctx, c.RBACErrors}

	return runPodCollector.Collect(progressChan)
}
