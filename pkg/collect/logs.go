package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type LogsOutput map[string][]byte

func Logs(ctx *Context, logsCollector *troubleshootv1beta1.Logs) ([]byte, error) {
	client, err := kubernetes.NewForConfig(ctx.ClientConfig)
	if err != nil {
		return nil, err
	}

	logsOutput := LogsOutput{}

	pods, podsErrors := listPodsInSelectors(client, logsCollector.Namespace, logsCollector.Selector)
	if len(podsErrors) > 0 {
		errorBytes, err := marshalNonNil(podsErrors)
		if err != nil {
			return nil, err
		}
		logsOutput[getLogsErrorsFileName(logsCollector)] = errorBytes
	}

	if len(pods) > 0 {
		for _, pod := range pods {
			if len(logsCollector.Containers) == 0 {
				podLogs, err := getPodLogs(client, pod, logsCollector.Name, "", logsCollector.Limits, false)
				if err != nil {
					key := fmt.Sprintf("%s/%s-errors.json", logsCollector.Name, pod.Name)
					logsOutput[key], err = marshalNonNil([]string{err.Error()})
					if err != nil {
						return nil, err
					}
					continue
				}
				for k, v := range podLogs {
					logsOutput[k] = v
				}
			} else {
				for _, container := range logsCollector.Containers {
					containerLogs, err := getPodLogs(client, pod, logsCollector.Name, container, logsCollector.Limits, false)
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

		if ctx.Redact {
			logsOutput, err = logsOutput.Redact()
			if err != nil {
				return nil, err
			}
		}
	}

	b, err := json.MarshalIndent(logsOutput, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, nil
}

func listPodsInSelectors(client *kubernetes.Clientset, namespace string, selector []string) ([]corev1.Pod, []string) {
	serializedLabelSelector := strings.Join(selector, ",")

	listOptions := metav1.ListOptions{
		LabelSelector: serializedLabelSelector,
	}

	pods, err := client.CoreV1().Pods(namespace).List(listOptions)
	if err != nil {
		return nil, []string{err.Error()}
	}

	return pods.Items, nil
}

func getPodLogs(client *kubernetes.Clientset, pod corev1.Pod, name, container string, limits *troubleshootv1beta1.LogLimits, follow bool) (map[string][]byte, error) {
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

	if limits != nil && limits.MaxAge != "" {
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

	req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOpts)
	podLogs, err := req.Stream()
	if err != nil {
		return nil, err
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return nil, err
	}

	fileKey := fmt.Sprintf("%s/%s.txt", name, pod.Name)
	if container != "" {
		fileKey = fmt.Sprintf("%s/%s/%s.txt", name, pod.Name, container)
	}

	return map[string][]byte{
		fileKey: buf.Bytes(),
	}, nil
}

func (l LogsOutput) Redact() (LogsOutput, error) {
	podLogs, err := redactMap(l)
	if err != nil {
		return nil, err
	}

	return podLogs, nil
}

func getLogsErrorsFileName(logsCollector *troubleshootv1beta1.Logs) string {
	if len(logsCollector.Name) > 0 {
		return fmt.Sprintf("%s/errors.json", logsCollector.Name)
	} else if len(logsCollector.CollectorName) > 0 {
		return fmt.Sprintf("%s/errors.json", logsCollector.CollectorName)
	}
	// TODO: random part
	return "errors.json"
}
