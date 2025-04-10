package collect

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type CollectLogs struct {
	Collector    *troubleshootv1beta2.Logs
	BundlePath   string
	Namespace    string // TODO: There is a Namespace parameter in troubleshootv1beta2.Logs. Should we remove this?
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	SinceTime    *time.Time
	RBACErrors
}

func (c *CollectLogs) Title() string {
	return getCollectorName(c)
}

func (c *CollectLogs) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectLogs) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectLogs) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	return c.CollectWithClient(progressChan, client)

}

// CollectWithClient is a helper function that allows passing in a kubernetes client
// It's a stopgap implementation before it's decided whether to either always use a single
// client for collectors or leave the implementation as is.
// Ref: https://github.com/replicatedhq/troubleshoot/pull/821#discussion_r1026258904
func (c *CollectLogs) CollectWithClient(progressChan chan<- interface{}, client kubernetes.Interface) (CollectorResult, error) {
	output := NewResult()

	ctx, cancel := context.WithTimeout(c.Context, constants.DEFAULT_LOGS_COLLECTOR_TIMEOUT)
	defer cancel()

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
				podLogs, err := savePodLogs(ctx, c.BundlePath, client, &pod, c.Collector.Name, containerName, c.Collector.Limits, false, true)
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						klog.Errorf("Pod logs timed out for pod %s and container %s: %v", pod.Name, containerName, err)
					}
					key := fmt.Sprintf("%s/%s-errors.json", c.Collector.Name, pod.Name)
					if containerName != "" {
						key = fmt.Sprintf("%s/%s/%s-errors.json", c.Collector.Name, pod.Name, containerName)
					}
					err := output.SaveResult(c.BundlePath, key, marshalErrors([]string{err.Error()}))
					if err != nil {
						klog.Errorf("Failed to save pod logs result for pod %s and container %s: %v", pod.Name, containerName, err)
					}
					continue
				}
				output.AddResult(podLogs)
			}
		} else {
			for _, containerName := range c.Collector.ContainerNames {
				containerLogs, err := savePodLogs(ctx, c.BundlePath, client, &pod, c.Collector.Name, containerName, c.Collector.Limits, false, true)
				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) {
						klog.Errorf("Pod logs timed out for pod %s and container %s: %v", pod.Name, containerName, err)
					}
					key := fmt.Sprintf("%s/%s/%s-errors.json", c.Collector.Name, pod.Name, containerName)
					err := output.SaveResult(c.BundlePath, key, marshalErrors([]string{err.Error()}))
					if err != nil {
						klog.Errorf("Failed to save pod logs result for pod %s and container %s: %v", pod.Name, containerName, err)
					}
					continue
				}
				output.AddResult(containerLogs)
			}
		}
	}

	return output, nil
}

func listPodsInSelectors(ctx context.Context, client kubernetes.Interface, namespace string, selector []string) ([]corev1.Pod, []string) {
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

func savePodLogs(
	ctx context.Context,
	bundlePath string,
	client kubernetes.Interface,
	pod *corev1.Pod,
	collectorName, container string,
	limits *troubleshootv1beta2.LogLimits,
	follow bool,
	createSymLinks bool,
) (CollectorResult, error) {
	podLogOpts := corev1.PodLogOptions{
		Follow:    follow,
		Container: container,
	}

	result := NewResult()

	// TODO: Abstract away hard coded directory structure paths
	// Maybe create a FS provider or something similar
	filePathPrefix := filepath.Join(
		constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_PODS_LOGS, pod.Namespace, pod.Name, pod.Spec.Containers[0].Name,
	)

	// TODO: If collectorName is empty, the path is stored with a leading slash
	// Retain this behavior otherwise analysers in the wild may break
	// Analysers that need to find a file in the root of the bundle should
	// prefix the path with a slash e.g /file.txt. This behavior should be
	// properly deprecated in the future.
	linkRelPathPrefix := fmt.Sprintf("%s/%s", collectorName, pod.Name)
	if container != "" {
		linkRelPathPrefix = fmt.Sprintf("%s/%s/%s", collectorName, pod.Name, container)
		filePathPrefix = filepath.Join(
			constants.CLUSTER_RESOURCES_DIR, constants.CLUSTER_RESOURCES_PODS_LOGS, pod.Namespace, pod.Name, container,
		)
	}

	setLogLimits(&podLogOpts, limits, convertMaxAgeToTime)

	req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get log stream")
	}
	defer podLogs.Close()

	logWriter, err := result.GetWriter(bundlePath, filePathPrefix+".log")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get log writer")
	}
	// NOTE: deferred calls are executed in LIFO order i.e called in reverse order
	if createSymLinks {
		defer result.SymLinkResult(bundlePath, linkRelPathPrefix+".log", filePathPrefix+".log")
	}
	defer result.CloseWriter(bundlePath, filePathPrefix+".log", logWriter)

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

	prevLogWriter, err := result.GetWriter(bundlePath, filePathPrefix+"-previous.log")
	if err != nil {
		return nil, errors.Wrap(err, "failed to get previous log writer")
	}
	// NOTE: deferred calls are executed in LIFO order i.e called in reverse order
	if createSymLinks {
		defer result.SymLinkResult(bundlePath, linkRelPathPrefix+"-previous.log", filePathPrefix+"-previous.log")
	}
	defer result.CloseWriter(bundlePath, filePathPrefix+"-previous.log", logWriter)

	_, err = io.Copy(prevLogWriter, podLogs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to copy previous log")
	}

	return result, nil
}

func convertMaxAgeToTime(maxAge string) *metav1.Time {
	parsedDuration, err := time.ParseDuration(maxAge)
	if err != nil {
		klog.Errorf("Failed to parse time duration %s", maxAge)
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

	defaultMaxBytes := int64(5000000)
	if limits.MaxBytes == 0 {
		podLogOpts.LimitBytes = &defaultMaxBytes
	} else {
		podLogOpts.LimitBytes = &limits.MaxBytes
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
