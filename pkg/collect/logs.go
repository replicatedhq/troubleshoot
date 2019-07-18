package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type LogsOutput struct {
	PodLogs map[string][]byte `json:"logs/,omitempty"`
}

func Logs(logsCollector *troubleshootv1beta1.Logs, redact bool) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	pods, err := listPodsInSelectors(client, logsCollector.Namespace, logsCollector.Selector)
	if err != nil {
		return err
	}

	logsOutput := &LogsOutput{
		PodLogs: make(map[string][]byte),
	}
	for _, pod := range pods {
		podLogs, err := getPodLogs(client, pod, logsCollector.Limits)
		if err != nil {
			return err
		}

		for k, v := range podLogs {
			logsOutput.PodLogs[k] = v
		}
	}

	if redact {
		logsOutput, err = logsOutput.Redact()
		if err != nil {
			return err
		}
	}

	b, err := json.MarshalIndent(logsOutput, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", b)

	return nil
}

func listPodsInSelectors(client *kubernetes.Clientset, namespace string, selector []string) ([]corev1.Pod, error) {
	serializedLabelSelector := strings.Join(selector, ",")

	listOptions := metav1.ListOptions{
		LabelSelector: serializedLabelSelector,
	}

	pods, err := client.CoreV1().Pods(namespace).List(listOptions)
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

func getPodLogs(client *kubernetes.Clientset, pod corev1.Pod, limits *troubleshootv1beta1.LogLimits) (map[string][]byte, error) {
	podLogOpts := corev1.PodLogOptions{}

	defaultMaxLines := int64(10000)
	if limits == nil || limits.MaxLines == 0 {
		podLogOpts.TailLines = &defaultMaxLines
	} else {
		podLogOpts.TailLines = &limits.MaxLines
	}

	if limits != nil && limits.MaxAge != "" {
		parsedDuration, err := time.ParseDuration(limits.MaxAge)
		if err != nil {
			fmt.Printf("unable to parse time duration %s\n", limits.MaxAge)
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
	}, nil
}
