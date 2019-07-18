package collect

import (
	// "bytes"
	// "encoding/json"
	// "fmt"
	// "io"
	// "strings"
	// "time"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	// corev1 "k8s.io/api/core/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/client-go/kubernetes"
	// "sigs.k8s.io/controller-runtime/pkg/client/config"
)

type RunOutput struct {
	PodLogs map[string][]byte `json:"run/,omitempty"`
}

func Run(runCollector *troubleshootv1beta1.Run, redact bool) error {
	// cfg, err := config.GetConfig()
	// if err != nil {
	// 	return err
	// }

	// client, err := kubernetes.NewForConfig(cfg)
	// if err != nil {
	// 	return err
	// }

	// pods, err := listPodsInSelectors(client, logsCollector.Namespace, logsCollector.Selector)
	// if err != nil {
	// 	return err
	// }

	// logsOutput := LogsOutput{
	// 	PodLogs: make(map[string][]byte),
	// }
	// for _, pod := range pods {
	// 	podLogs, err := getPodLogs(client, pod, logsCollector.Limits)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	for k, v := range podLogs {
	// 		logsOutput.PodLogs[k] = v
	// 	}
	// }

	// b, err := json.MarshalIndent(logsOutput, "", "  ")
	// if err != nil {
	// 	return err
	// }

	// fmt.Printf("%s\n", b)

	return nil
}
