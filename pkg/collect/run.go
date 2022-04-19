package collect

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func Run(c *Collector, runCollector *troubleshootv1beta2.Run) (CollectorResult, error) {
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
			logger.Printf("Failed to delete pod %s: %v", pod.Name, err)
		}
	}()
	if runCollector.ImagePullSecret != nil && runCollector.ImagePullSecret.Data != nil {
		defer func() {
			for _, k := range pod.Spec.ImagePullSecrets {
				if err := client.CoreV1().Secrets(pod.Namespace).Delete(ctx, k.Name, metav1.DeleteOptions{}); err != nil {
					logger.Printf("Failed to delete secret %s: %v", k.Name, err)
				}
			}
		}()
	}
	if runCollector.Timeout == "" {
		return runWithoutTimeout(ctx, c, pod, runCollector)
	}

	timeout, err := time.ParseDuration(runCollector.Timeout)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse timeout")
	}

	errCh := make(chan error, 1)
	resultCh := make(chan CollectorResult, 1)

	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	go func() {
		b, err := runWithoutTimeout(timeoutCtx, c, pod, runCollector)
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

func runWithoutTimeout(ctx context.Context, c *Collector, pod *corev1.Pod, runCollector *troubleshootv1beta2.Run) (CollectorResult, error) {
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
		if status.Status.Phase == corev1.PodPending {
			for _, v := range status.Status.ContainerStatuses {
				if v.State.Waiting != nil && v.State.Waiting.Reason == "ImagePullBackOff" {
					return nil, errors.Errorf("run pod aborted after getting pod status 'ImagePullBackOff'")
				}
			}
		}
		time.Sleep(time.Second * 1)
	}

	output := NewResult()

	limits := troubleshootv1beta2.LogLimits{
		MaxLines: 10000,
	}
	podLogs, err := savePodLogs(ctx, c.BundlePath, client, *pod, runCollector.Name, "", &limits, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pod logs")
	}

	for k, v := range podLogs {
		output[k] = v
	}

	return output, nil
}

func runPod(ctx context.Context, client *kubernetes.Clientset, runCollector *troubleshootv1beta2.Run, namespace string) (*corev1.Pod, error) {
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

	serviceAccountName := "default"
	if runCollector.ServiceAccountName != "" {
		serviceAccountName = runCollector.ServiceAccountName
	}

	pod := corev1.Pod{}
	if runCollector.PodSpec.Containers != nil {
		pod = corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      runCollector.CollectorName,
				Namespace: namespace,
				Labels:    podLabels,
			},
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			Spec: runCollector.PodSpec,
		}
	} else {
		pod = corev1.Pod{
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
				RestartPolicy:      corev1.RestartPolicyNever,
				ServiceAccountName: serviceAccountName,
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

		if runCollector.ImagePullSecret != nil && runCollector.ImagePullSecret.Data != nil {
			secretName, err := createSecret(ctx, client, pod.Namespace, runCollector.ImagePullSecret)
			if err != nil {
				return nil, errors.Wrap(err, "failed to create secret")
			}
			pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: secretName})
		}
	}

	created, err := client.CoreV1().Pods(namespace).Create(ctx, &pod, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create pod")
	}

	return created, nil
}

func createSecret(ctx context.Context, client kubernetes.Interface, namespace string, imagePullSecret *troubleshootv1beta2.ImagePullSecrets) (string, error) {
	if imagePullSecret.Data == nil {
		return "", nil
	}

	var out bytes.Buffer
	data := make(map[string][]byte)
	if imagePullSecret.SecretType != "kubernetes.io/dockerconfigjson" {
		return "", errors.Errorf("ImagePullSecret must be of type: kubernetes.io/dockerconfigjson")
	}

	// Check if required field in data exists
	v, found := imagePullSecret.Data[".dockerconfigjson"]
	if !found {
		return "", errors.Errorf("Secret type kubernetes.io/dockerconfigjson requires argument \".dockerconfigjson\"")
	}
	if len(imagePullSecret.Data) > 1 {
		return "", errors.Errorf("Secret type kubernetes.io/dockerconfigjson accepts only one argument \".dockerconfigjson\"")
	}
	// K8s client accepts only Json formated files as data, provided data must be decoded and indented
	parsedConfig, err := base64.StdEncoding.DecodeString(v)
	if err != nil {
		return "", errors.Wrap(err, "Unable to decode data.")
	}
	err = json.Indent(&out, parsedConfig, "", "\t")
	if err != nil {
		return "", errors.Wrap(err, "Unable to parse encoded data.")
	}
	data[".dockerconfigjson"] = out.Bytes()

	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:         imagePullSecret.Name,
			GenerateName: "troubleshoot",
			Namespace:    namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "troubleshoot.sh",
			},
		},
		Data: data,
		Type: corev1.SecretType(imagePullSecret.SecretType),
	}

	created, err := client.CoreV1().Secrets(namespace).Create(ctx, &secret, metav1.CreateOptions{})
	if err != nil {
		return "", errors.Wrap(err, "failed to create secret")
	}

	return created.Name, nil
}
