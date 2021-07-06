package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	runnerContainerName = "collector"
)

func CreateCollector(client *kubernetes.Clientset, scheme *runtime.Scheme, ownerRef metav1.Object, name string, namespace string, nodeName string, serviceAccountName string, jobType string, collect *troubleshootv1beta2.HostCollect, image string, pullPolicy string) (*corev1.ConfigMap, *corev1.Pod, error) {
	configMap, err := createCollectorConfigMap(client, scheme, ownerRef, name, namespace, collect)
	if err != nil {
		return nil, nil, err
	}

	pod, err := createCollectorPod(client, scheme, ownerRef, name, namespace, nodeName, serviceAccountName, jobType, collect, configMap, image, pullPolicy)
	if err != nil {
		return nil, nil, err
	}

	return configMap, pod, nil
}

func createCollectorConfigMap(client *kubernetes.Clientset, scheme *runtime.Scheme, ownerRef metav1.Object, name string, namespace string, collect *troubleshootv1beta2.HostCollect) (*corev1.ConfigMap, error) {
	_, err := client.CoreV1().ConfigMaps(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err == nil || !kuberneteserrors.IsNotFound(err) {
		return nil, err
	}

	collector := troubleshootv1beta2.HostCollector{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "troubleshoot.sh/v1beta2",
			Kind:       "HostCollector",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "collector",
		},
		Spec: troubleshootv1beta2.HostCollectorSpec{
			Collectors: []*troubleshootv1beta2.HostCollect{collect},
		},
	}

	// Use json as TypeMeta and ObjectMeta don't have tags for yaml, so
	// capitalization (e.g. apiVersion) is not preserved.
	contents, err := json.Marshal(collector)
	if err != nil {
		return nil, err
	}

	specData := make(map[string]string)
	specData["collector.json"] = string(contents)

	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		Data: specData,
	}

	if ownerRef != nil && scheme != nil {
		if err := controllerutil.SetControllerReference(ownerRef, &configMap, scheme); err != nil {
			return nil, err
		}
	}

	created, err := client.CoreV1().ConfigMaps(namespace).Create(context.Background(), &configMap, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return created, nil
}

func createCollectorPod(client kubernetes.Interface, scheme *runtime.Scheme, ownerRef metav1.Object, name string, namespace string, nodeName string, serviceAccountName string, jobType string, collect *troubleshootv1beta2.HostCollect, configMap *corev1.ConfigMap, image string, pullPolicy string) (*corev1.Pod, error) {
	if serviceAccountName == "" {
		serviceAccountName = "default"
	}

	_, err := client.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err == nil {
		return nil, fmt.Errorf("pod %q already exists", name)
	} else if !kuberneteserrors.IsNotFound(err) {
		return nil, err
	}

	imageName := "replicated/troubleshoot:latest"
	imagePullPolicy := corev1.PullAlways

	if image != "" {
		imageName = image
	}
	if pullPolicy != "" {
		imagePullPolicy = corev1.PullPolicy(pullPolicy)
	}

	podLabels := make(map[string]string)

	podLabels[jobType] = name
	podLabels["troubleshoot-role"] = jobType

	nodeSelector := map[string]string{
		"kubernetes.io/hostname": nodeName,
	}

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    podLabels,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		Spec: corev1.PodSpec{
			NodeSelector:       nodeSelector,
			ServiceAccountName: serviceAccountName,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Image:           imageName,
					ImagePullPolicy: imagePullPolicy,
					Name:            runnerContainerName,
					Command:         []string{"collect"},
					Args: []string{
						"--collect-without-permissions",
						"--format=raw",
						"/troubleshoot/specs/collector.json",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "collector",
							MountPath: "/troubleshoot/specs",
							ReadOnly:  true,
						},
						{
							Name:      "kernel-modules",
							MountPath: "/lib/modules",
							ReadOnly:  true,
						},
						{
							Name:      "ntp",
							MountPath: "/run/dbus",
							ReadOnly:  true,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "collector",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMap.Name,
							},
						},
					},
				},
				{
					Name: "kernel-modules",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/lib/modules",
						},
					},
				},
				{
					Name: "ntp",
					VolumeSource: corev1.VolumeSource{
						HostPath: &corev1.HostPathVolumeSource{
							Path: "/run/dbus",
						},
					},
				},
			},
		},
	}

	if ownerRef != nil && scheme != nil {
		if err := controllerutil.SetControllerReference(ownerRef, &pod, scheme); err != nil {
			return nil, err
		}
	}

	created, err := client.CoreV1().Pods(namespace).Create(context.Background(), &pod, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return created, nil
}

type podCondition func(pod *corev1.Pod) (bool, error)

// WaitForPodCondition waits for a pod to match the given condition.
func WaitForPodCondition(ctx context.Context, client kubernetes.Interface, namespace string, podName string, interval time.Duration, condition podCondition) error {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			pod, err := client.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				if kuberneteserrors.IsNotFound(err) {
					return err
				}
				continue
			}
			if done, err := condition(pod); done {
				if err == nil {
					return nil
				}
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WaitForPodCompleted returns nil if the pod reached state success, or an error if it reached failure or ran too long.
func WaitForPodCompleted(ctx context.Context, client kubernetes.Interface, namespace string, podName string, interval time.Duration) error {
	return WaitForPodCondition(ctx, client, namespace, podName, interval, func(pod *corev1.Pod) (bool, error) {
		if pod.Spec.RestartPolicy == corev1.RestartPolicyAlways {
			return true, fmt.Errorf("pod %q will never terminate with a succeeded state since its restart policy is Always", podName)
		}
		switch pod.Status.Phase {
		case corev1.PodSucceeded, corev1.PodFailed:
			return true, nil
		default:
			return false, nil
		}
	})
}

func GetContainerLogs(ctx context.Context, client kubernetes.Interface, namespace string, podName string, containerName string, waitForComplete bool, interval time.Duration) (string, error) {
	if waitForComplete {
		if err := WaitForPodCompleted(ctx, client, namespace, podName, interval); err != nil {
			return "", err
		}
	}
	return getContainerLogsInternal(ctx, client, namespace, podName, containerName, false)
}

func getContainerLogsInternal(ctx context.Context, client kubernetes.Interface, namespace string, podName string, containerName string, previous bool) (string, error) {
	logs, err := client.CoreV1().RESTClient().Get().
		Resource("pods").
		Namespace(namespace).
		Name(podName).SubResource("log").
		Param("container", containerName).
		Param("previous", strconv.FormatBool(previous)).
		Do(ctx).
		Raw()
	if err != nil {
		return "", err
	}
	if err == nil && strings.Contains(string(logs), "Internal Error") {
		return "", fmt.Errorf("Fetched log contains \"Internal Error\": %q", string(logs))
	}
	return string(logs), err
}
