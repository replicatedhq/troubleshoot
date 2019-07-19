package collect

import (
	"encoding/json"
	"fmt"
	"time"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type RunOutput struct {
	PodLogs map[string][]byte `json:"run/,omitempty"`
}

func Run(runCollector *troubleshootv1beta1.Run, redact bool) error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	pod, err := runPod(client, runCollector)
	if err != nil {
		return err
	}

	runOutput := &RunOutput{
		PodLogs: make(map[string][]byte),
	}

	now := time.Now()
	then := now.Add(time.Duration(20 * time.Second))

	if runCollector.Timeout != "" {
		parsedDuration, err := time.ParseDuration(runCollector.Timeout)
		if err != nil {
			fmt.Printf("unable to parse time duration %s\n", runCollector.Timeout)
		} else {
			then = now.Add(parsedDuration)
		}
	}

	for {
		if time.Now().After(then) {
			break
		}

		time.Sleep(time.Second)
	}

	limits := troubleshootv1beta1.LogLimits{
		MaxLines: 10000,
	}
	podLogs, err := getPodLogs(client, *pod, &limits)
	if err != nil {
		return err
	}

	for k, v := range podLogs {
		runOutput.PodLogs[k] = v
	}

	if err := client.CoreV1().Pods(pod.Namespace).Delete(pod.Name, &metav1.DeleteOptions{}); err != nil {
		return err
	}

	if redact {
		runOutput, err = runOutput.Redact()
		if err != nil {
			return err
		}
	}

	b, err := json.MarshalIndent(runOutput, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", b)

	return nil
}

func runPod(client *kubernetes.Clientset, runCollector *troubleshootv1beta1.Run) (*corev1.Pod, error) {
	podLabels := make(map[string]string)
	podLabels["troubleshoot-role"] = "run-collector"

	pullPolicy := corev1.PullIfNotPresent
	if runCollector.ImagePullPolicy != "" {
		pullPolicy = corev1.PullPolicy(runCollector.ImagePullPolicy)
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      runCollector.Name,
			Namespace: runCollector.Namespace,
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

	created, err := client.CoreV1().Pods(runCollector.Namespace).Create(&pod)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *RunOutput) Redact() (*RunOutput, error) {
	podLogs, err := redactMap(r.PodLogs)
	if err != nil {
		return nil, err
	}

	return &RunOutput{
		PodLogs: podLogs,
	}, nil
}
