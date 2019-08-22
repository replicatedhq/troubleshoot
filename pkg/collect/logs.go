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

type LogsOutput struct {
	PodLogs map[string][]byte `json:"logs/,omitempty"`
	Errors  map[string][]byte `json:"logs-errors/,omitempty"`
}

func Logs(ctx *Context, logsCollector *troubleshootv1beta1.Logs) ([]byte, error) {
	client, err := kubernetes.NewForConfig(ctx.ClientConfig)
	if err != nil {
		return nil, err
	}

	logsOutput := &LogsOutput{
		PodLogs: make(map[string][]byte),
		Errors:  make(map[string][]byte),
	}

	pods, podsErrors := listPodsInSelectors(client, logsCollector.Namespace, logsCollector.Selector)
	if len(podsErrors) > 0 {
		errorBytes, err := marshalNonNil(podsErrors)
		if err != nil {
			return nil, err
		}
		logsOutput.Errors[getLogsErrorsFileName(logsCollector)] = errorBytes
	}

	if len(pods) > 0 {
		for _, pod := range pods {
			podLogs, err := getPodLogs(client, pod, logsCollector.Limits, false)
			if err != nil {
				key := fmt.Sprintf("%s/%s-errors.json", pod.Namespace, pod.Name)
				logsOutput.Errors[key], err = marshalNonNil([]string{err.Error()})
				if err != nil {
					return nil, err
				}
				continue
			}

			for k, v := range podLogs {
				logsOutput.PodLogs[k] = v
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

func getPodLogs(client *kubernetes.Clientset, pod corev1.Pod, limits *troubleshootv1beta1.LogLimits, follow bool) (map[string][]byte, error) {
	podLogOpts := corev1.PodLogOptions{
		Follow: follow,
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

	return map[string][]byte{
		fmt.Sprintf("%s/%s.txt", pod.Namespace, pod.Name): buf.Bytes(),
	}, nil
}

func (l *LogsOutput) Redact() (*LogsOutput, error) {
	podLogs, err := redactMap(l.PodLogs)
	if err != nil {
		return nil, err
	}

	return &LogsOutput{
		PodLogs: podLogs,
		Errors:  l.Errors,
	}, nil
}

func getLogsErrorsFileName(logsCollector *troubleshootv1beta1.Logs) string {
	if len(logsCollector.CollectorName) > 0 {
		return fmt.Sprintf("%s.json", logsCollector.CollectorName)
	}
	// TODO: random part
	return "errors.json"
}
