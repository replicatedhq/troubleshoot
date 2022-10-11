package collect

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectLogs struct {
	Collector    *troubleshootv1beta2.Logs
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	SinceTime    *time.Time
	RBACErrors
}

func (c *CollectLogs) Title() string {
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "Logs")
}

func (c *CollectLogs) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectLogs) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	output := NewResult()

	ctx := context.Background()

	if c.SinceTime != nil {
		if c.Collector.Limits == nil {
			c.Collector.Limits = new(troubleshootv1beta2.LogLimits)
		}
		c.Collector.Limits.SinceTime = metav1.NewTime(*c.SinceTime)
	}

	pods, podsErrors := listPodsInSelectors(ctx, client, c.Collector.Namespace, c.Collector.Selector)
	if len(podsErrors) > 0 {
		output.SaveResult(c.BundlePath, getLogsErrorsFileName(c.Collector), marshalErrors(podsErrors))
	}

	if len(pods) > 0 {
		for _, pod := range pods {
			if len(c.Collector.ContainerNames) == 0 {
				// make a list of all the containers in the pod, so that we can get logs from all of them
				containerNames := []string{}
				for _, container := range pod.Spec.Containers {
					containerNames = append(containerNames, container.Name)
				}
				for _, container := range pod.Spec.InitContainers {
					containerNames = append(containerNames, container.Name)
				}

				for _, containerName := range containerNames {
					if len(containerNames) == 1 {
						containerName = "" // if there was only one container, use the old behavior of not including the container name in the path
					}
					podLogs, err := savePodLogs(ctx, c.BundlePath, client, pod, c.Collector.Name, containerName, c.Collector.Limits, false)
					if err != nil {
						key := fmt.Sprintf("%s/%s-errors.json", c.Collector.Name, pod.Name)
						if containerName != "" {
							key = fmt.Sprintf("%s/%s/%s-errors.json", c.Collector.Name, pod.Name, containerName)
						}
						err := output.SaveResult(c.BundlePath, key, marshalErrors([]string{err.Error()}))
						if err != nil {
							return nil, err
						}
						continue
					}
					for k, v := range podLogs {
						output[k] = v
					}
				}
			} else {
				for _, container := range c.Collector.ContainerNames {
					containerLogs, err := savePodLogs(ctx, c.BundlePath, client, pod, c.Collector.Name, container, c.Collector.Limits, false)
					if err != nil {
						key := fmt.Sprintf("%s/%s/%s-errors.json", c.Collector.Name, pod.Name, container)
						err := output.SaveResult(c.BundlePath, key, marshalErrors([]string{err.Error()}))
						if err != nil {
							return nil, err
						}
						continue
					}
					for k, v := range containerLogs {
						output[k] = v
					}
				}
			}
		}
	}

	return output, nil
}

func listPodsInSelectors(ctx context.Context, client *kubernetes.Clientset, namespace string, selector []string) ([]corev1.Pod, []string) {
	serializedLabelSelector := strings.Join(selector, ",")

	listOptions := metav1.ListOptions{
		LabelSelector: serializedLabelSelector,
	}

	pods, err := client.CoreV1().Pods(namespace).List(ctx, listOptions)
	if err != nil {
		return nil, []string{err.Error()}
	}

	return pods.Items, nil
}

func savePodLogs(ctx context.Context, bundlePath string, client *kubernetes.Clientset, pod corev1.Pod, name, container string, limits *troubleshootv1beta2.LogLimits, follow bool) (CollectorResult, error) {
	podLogOpts := corev1.PodLogOptions{
		Follow:    follow,
		Container: container,
	}

	setLogLimits(&podLogOpts, limits, convertMaxAgeToTime)

	fileKey := fmt.Sprintf("%s/%s", name, pod.Name)
	if container != "" {
		fileKey = fmt.Sprintf("%s/%s/%s", name, pod.Name, container)
	}

	result := NewResult()

	req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get log stream")
	}
	defer podLogs.Close()

	logWriter, err := result.GetWriter(bundlePath, fileKey+".log")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get log writer")
	}
	defer result.CloseWriter(bundlePath, fileKey+".log", logWriter)

	_, err = io.Copy(logWriter, podLogs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to copy log")
	}

	podLogOpts.Previous = true
	req = client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err = req.Stream(ctx)
	if err != nil {
		// maybe fail on !kuberneteserrors.IsNotFound(err)?
		return result, nil
	}
	defer podLogs.Close()

	prevLogWriter, err := result.GetWriter(bundlePath, fileKey+"-previous.log")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get previous log writer")
	}
	defer result.CloseWriter(bundlePath, fileKey+"-previous.log", logWriter)

	_, err = io.Copy(prevLogWriter, podLogs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to copy previous log")
	}

	return result, nil
}

func convertMaxAgeToTime(maxAge string) *metav1.Time {
	parsedDuration, err := time.ParseDuration(maxAge)
	if err != nil {
		logger.Printf("Failed to parse time duration %s", maxAge)
		return nil
	}

	now := time.Now()
	then := now.Add(0 - parsedDuration)
	kthen := metav1.NewTime(then)

	return &kthen
}

func setLogLimits(podLogOpts *corev1.PodLogOptions, limits *troubleshootv1beta2.LogLimits, maxAgeParser func(maxAge string) *metav1.Time) {
	if podLogOpts == nil {
		return
	}

	defaultMaxLines := int64(10000)
	if limits == nil {
		podLogOpts.TailLines = &defaultMaxLines
		return
	}

	if !limits.SinceTime.IsZero() {
		podLogOpts.SinceTime = &limits.SinceTime
		return
	}

	if limits.MaxAge != "" {
		podLogOpts.SinceTime = maxAgeParser(limits.MaxAge)
		return
	}

	if limits.MaxLines == 0 {
		podLogOpts.TailLines = &defaultMaxLines
	} else {
		podLogOpts.TailLines = &limits.MaxLines
	}
}

func getLogsErrorsFileName(logsCollector *troubleshootv1beta2.Logs) string {
	if len(logsCollector.Name) > 0 {
		return fmt.Sprintf("%s/errors.json", logsCollector.Name)
	} else if len(logsCollector.CollectorName) > 0 {
		return fmt.Sprintf("%s/errors.json", logsCollector.CollectorName)
	}
	// TODO: random part
	return "errors.json"
}
