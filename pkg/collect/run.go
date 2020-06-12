package collect

import (
	"context"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Run(c *Collector, runCollector *troubleshootv1beta1.Run) (map[string][]byte, error) {
	ctx := context.Background()

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client from config")
	}

	pod, err := runPod(ctx, client, runCollector, c.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run pod")
	}

	defer func() {
		if err := client.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
			logger.Printf("Failed to delete pod %s: %v\n", pod.Name, err)
		}
	}()

	if runCollector.Timeout == "" {
		return runWithoutTimeout(ctx, c, pod, runCollector)
	}

	timeout, err := time.ParseDuration(runCollector.Timeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse timeout")
	}

	errCh := make(chan error, 1)
	resultCh := make(chan map[string][]byte, 1)
	go func() {
		b, err := runWithoutTimeout(ctx, c, pod, runCollector)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- b
		}
	}()

	select {
	case <-time.After(timeout):
		return nil, errors.New("timeout")
	case result := <-resultCh:
		return result, nil
	case err := <-errCh:
		return nil, err
	}
}

func runWithoutTimeout(ctx context.Context, c *Collector, pod *corev1.Pod, runCollector *troubleshootv1beta1.Run) (map[string][]byte, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "failed create client from config")
	}

	for {
		status, err := client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return nil, errors.Wrap(err, "failed to get pod")
		}
		if status.Status.Phase == corev1.PodRunning ||
			status.Status.Phase == corev1.PodFailed ||
			status.Status.Phase == corev1.PodSucceeded {
			break
		}
		time.Sleep(time.Second * 1)
	}

	runOutput := map[string][]byte{}

	limits := troubleshootv1beta1.LogLimits{
		MaxLines: 10000,
	}
	podLogs, err := getPodLogs(ctx, client, *pod, runCollector.Name, "", &limits, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pod logs")
	}

	for k, v := range podLogs {
		runOutput[k] = v
	}

	return runOutput, nil
}

func runPod(ctx context.Context, client *kubernetes.Clientset, runCollector *troubleshootv1beta1.Run, namespace string) (*corev1.Pod, error) {
	podLabels := make(map[string]string)
	podLabels["troubleshoot-role"] = "run-collector"

	pullPolicy := corev1.PullIfNotPresent
	if runCollector.ImagePullPolicy != "" {
		pullPolicy = corev1.PullPolicy(runCollector.ImagePullPolicy)
	}

	if namespace == "" {
		namespace = runCollector.Namespace
	}
	if namespace == "" {
		namespace = "default"
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      runCollector.CollectorName,
			Namespace: namespace,
			Labels:    podLabels,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Image:           runCollector.Image,
					ImagePullPolicy: pullPolicy,
					Name:            "collector",
					Command:         runCollector.Command,
					Args:            runCollector.Args,
				},
			},
		},
	}

	created, err := client.CoreV1().Pods(namespace).Create(ctx, &pod, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create pod")
	}

	return created, nil
}
