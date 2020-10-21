package collect

import (
	"bytes"
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
)

func Logs(c *Collector, logsCollector *troubleshootv1beta2.Logs) (map[string][]byte, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	logsOutput := map[string][]byte{}

	ctx := context.Background()

	pods, podsErrors := listPodsInSelectors(ctx, client, logsCollector.Namespace, logsCollector.Selector)
	if len(podsErrors) > 0 {
		errorBytes, err := marshalNonNil(podsErrors)
		if err != nil {
			return nil, errors.Wrap(err, "failed to list pods")
		}
		logsOutput[getLogsErrorsFileName(logsCollector)] = errorBytes
	}

	if len(pods) > 0 {
		for _, pod := range pods {
			if len(logsCollector.ContainerNames) == 0 {
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
					podLogs, err := getPodLogs(ctx, client, pod, logsCollector.Name, containerName, logsCollector.Limits, false)
					if err != nil {
						key := fmt.Sprintf("%s/%s-errors.json", logsCollector.Name, pod.Name)
						if containerName != "" {
							key = fmt.Sprintf("%s/%s/%s-errors.json", logsCollector.Name, pod.Name, containerName)
						}
						logsOutput[key], err = marshalNonNil([]string{err.Error()})
						if err != nil {
							return nil, err
						}
						continue
					}
					for k, v := range podLogs {
						logsOutput[k] = v
					}
				}
			} else {
				for _, container := range logsCollector.ContainerNames {
					containerLogs, err := getPodLogs(ctx, client, pod, logsCollector.Name, container, logsCollector.Limits, false)
					if err != nil {
						key := fmt.Sprintf("%s/%s/%s-errors.json", logsCollector.Name, pod.Name, container)
						logsOutput[key], err = marshalNonNil([]string{err.Error()})
						if err != nil {
							return nil, err
						}
						continue
					}
					for k, v := range containerLogs {
						logsOutput[k] = v
					}
				}
			}
		}
	}

	return logsOutput, nil
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

func getPodLogs(ctx context.Context, client *kubernetes.Clientset, pod corev1.Pod, name, container string, limits *troubleshootv1beta2.LogLimits, follow bool) (map[string][]byte, error) {
	podLogOpts := corev1.PodLogOptions{
		Follow:    follow,
		Container: container,
	}

	defaultMaxLines := int64(10000)
	if limits == nil || limits.MaxLines == 0 {
		podLogOpts.TailLines = &defaultMaxLines
	} else {
		podLogOpts.TailLines = &limits.MaxLines
	}

	if limits != nil && limits.SinceTime != "" {
		t, err := time.Parse(time.RFC3339, limits.SinceTime)
		if err != nil {
			logger.Printf("unable to parse --since-time=%s\n", limits.SinceTime)
		}

		sinceTime := metav1.NewTime(t)
		podLogOpts.SinceTime = &sinceTime

	} else if limits != nil && (limits.MaxAge != "" || limits.Since != "") {
		if limits.Since != "" {
			limits.MaxAge = limits.Since
		}
		parsedDuration, err := time.ParseDuration(limits.MaxAge)
		if err != nil {
			logger.Printf("unable to parse time duration %s\n", limits.MaxAge)
		} else {
			now := time.Now()
			then := now.Add(0 - parsedDuration)
			kthen := metav1.NewTime(then)

			podLogOpts.SinceTime = &kthen
		}
	}

	fileKey := fmt.Sprintf("%s/%s", name, pod.Name)
	if container != "" {
		fileKey = fmt.Sprintf("%s/%s/%s", name, pod.Name, container)
	}

	result := make(map[string][]byte)

	req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get log stream")
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to copy log")
	}
	result[fileKey+".log"] = buf.Bytes()

	podLogOpts.Previous = true
	req = client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err = req.Stream(ctx)
	if err != nil {
		// maybe fail on !kuberneteserrors.IsNotFound(err)?
		return result, nil
	}
	defer podLogs.Close()

	buf = new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to copy previous log")
	}
	result[fileKey+"-previous.log"] = buf.Bytes()

	return result, nil
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
