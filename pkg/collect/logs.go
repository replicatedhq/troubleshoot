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
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type CollectLogs struct {
	Collector    *troubleshootv1beta2.Logs
	BundlePath   string
	Namespace    string // There is a Namespace parameter in troubleshootv1beta2.Logs. Should we remove this?
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

func (c *CollectLogs) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	output := NewResult()

	ctx := context.Background()

	const timeout = 60 //timeout in seconds used for context timeout value

	// timeout context
	ctxTimeout, cancel := context.WithTimeout(context.Background(), timeout*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	resultCh := make(chan CollectorResult, 1)

	//wrapped code go func for context timeout solution
	go func() {

		output := NewResult()

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
						podLogs, err := savePodLogs(ctx, c.BundlePath, client, &pod, c.Collector.Name, containerName, c.Collector.Limits, false, true)
						if err != nil {
							key := fmt.Sprintf("%s/%s-errors.json", c.Collector.Name, pod.Name)
							if containerName != "" {
								key = fmt.Sprintf("%s/%s/%s-errors.json", c.Collector.Name, pod.Name, containerName)
							}
							err := output.SaveResult(c.BundlePath, key, marshalErrors([]string{err.Error()}))
							if err != nil {
								errCh <- err
							}
							continue
						}
						output.AddResult(podLogs)

						resultCh <- output
					}
				} else {
					for _, container := range c.Collector.ContainerNames {
						containerLogs, err := savePodLogs(ctx, c.BundlePath, client, &pod, c.Collector.Name, container, c.Collector.Limits, false, true)
						if err != nil {
							key := fmt.Sprintf("%s/%s/%s-errors.json", c.Collector.Name, pod.Name, container)
							err := output.SaveResult(c.BundlePath, key, marshalErrors([]string{err.Error()}))
							if err != nil {
								errCh <- err
							}
							continue
						}
						output.AddResult(containerLogs)
						resultCh <- output
					}
				}
			}
		} else {
			resultCh <- output
		}
	}()

	select {
	case <-ctxTimeout.Done():
		return nil, fmt.Errorf("%s (%s) collector timeout exceeded", c.Title(), c.Collector.CollectorName)
	case o := <-resultCh:
		output = o
	case err := <-errCh:
		return nil, err
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
	client *kubernetes.Clientset,
	pod *corev1.Pod,
	collectorName, container string,
	limits *troubleshootv1beta2.LogLimits,
	follow bool,
	createSymLinks bool,
) (CollectorResult, error) {
	return savePodLogsWithInterface(ctx, bundlePath, client, pod, collectorName, container, limits, follow, createSymLinks)
}

func savePodLogsWithInterface(
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
		"cluster-resources", "pods", "logs", pod.Namespace, pod.Name, pod.Spec.Containers[0].Name,
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
			"cluster-resources", "pods", "logs", pod.Namespace, pod.Name, container,
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

	if limits.MaxBytes == 0 {
		podLogOpts.LimitBytes = &limits.MaxBytes
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
