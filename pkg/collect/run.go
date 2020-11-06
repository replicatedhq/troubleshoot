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

func Run(c *Collector, runCollector *troubleshootv1beta2.Run) (map[string][]byte, error) {
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
			logger.Printf("Failed to delete pod %s: %v\n", pod.Name, err)
		}
	}()
	if runCollector.ImagePullSecret != nil && runCollector.ImagePullSecret.Data != nil {
		defer func() {
			for _, k := range pod.Spec.ImagePullSecrets {
				if err := client.CoreV1().Secrets(pod.Namespace).Delete(ctx, k.Name, metav1.DeleteOptions{}); err != nil {
					logger.Printf("Failed to delete secret %s: %v\n", k.Name, err)
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
	resultCh := make(chan map[string][]byte, 1)
	go func() {
		b, err := runWithoutTimeout(ctx, c, pod, runCollector)
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

func runWithoutTimeout(ctx context.Context, c *Collector, pod *corev1.Pod, runCollector *troubleshootv1beta2.Run) (map[string][]byte, error) {
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

	runOutput := map[string][]byte{}

	limits := troubleshootv1beta2.LogLimits{
		MaxLines: 10000,
	}
	podLogs, err := getPodLogs(ctx, client, *pod, runCollector.Name, "", &limits, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pod logs")
	}

	for k, v := range podLogs {
		runOutput[k] = v
	}

	return runOutput, nil
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

	pod := corev1.Pod{
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

	if runCollector.ImagePullSecret != nil {
		err := createSecret(ctx, client, runCollector.ImagePullSecret, &pod)
		if err != nil {
			return nil, errors.Wrap(err, "failed to create secret")
		}
	}
	created, err := client.CoreV1().Pods(namespace).Create(ctx, &pod, metav1.CreateOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create pod")
	}

	return created, nil
}
func createSecret(ctx context.Context, client *kubernetes.Clientset, imagePullSecret *troubleshootv1beta2.ImagePullSecrets, pod *corev1.Pod) error {
	//In case a new secret needs to be created
	if imagePullSecret.Data != nil {
		var out bytes.Buffer
		data := make(map[string][]byte)
		if imagePullSecret.SecretType == "kubernetes.io/dockerconfigjson" {
			//Check if required field in data exists
			v, found := imagePullSecret.Data[".dockerconfigjson"]
			if !found {
				return errors.Errorf("Secret type kubernetes.io/dockerconfigjson requires argument \".dockerconfigjson\"")
			}
			if len(imagePullSecret.Data) > 1 {
				return errors.Errorf("Secret type kubernetes.io/dockerconfigjson accepts only one argument \".dockerconfigjson\"")
			}
			//K8s client accepts only Json formated files as data, provided data must be decoded and indented
			parsedConfig, err := base64.StdEncoding.DecodeString(v)
			if err != nil {
				return errors.Wrap(err, "Unable to decode data.")
			}
			err = json.Indent(&out, parsedConfig, "", "\t")
			if err != nil {
				return errors.Wrap(err, "Unable to parse encoded data.")
			}
			data[".dockerconfigjson"] = out.Bytes()

		} else {
			return errors.Errorf("ImagePullSecret must be of type: kubernetes.io/dockerconfigjson")
		}
		secret := corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:         imagePullSecret.Name,
				GenerateName: "troubleshoot",
				Namespace:    pod.Namespace,
			},
			Data: data,
			Type: corev1.SecretType(imagePullSecret.SecretType),
		}
		created, err := client.CoreV1().Secrets(pod.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to create secret")
		}
		pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: created.Name})
		return nil
	}
	//In case secret must only be added to the specs.
	if imagePullSecret.Name != "" {
		pod.Spec.ImagePullSecrets = append(pod.Spec.ImagePullSecrets, corev1.LocalObjectReference{Name: imagePullSecret.Name})
		return nil
	}
	return errors.Errorf("Secret must at least have a Name")
}
