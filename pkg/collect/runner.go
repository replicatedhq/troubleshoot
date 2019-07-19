package collect

import (
	"context"
	"fmt"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func CreateCollector(client client.Client, scheme *runtime.Scheme, ownerRef metav1.Object, jobName string, jobNamespace string, jobType string, collect *troubleshootv1beta1.Collect, image string, pullPolicy string) (*corev1.ConfigMap, *corev1.Pod, error) {
	configMap, err := createCollectorSpecConfigMap(client, scheme, ownerRef, jobName, jobNamespace, collect)
	if err != nil {
		return nil, nil, err
	}

	pod, err := createCollectorPod(client, scheme, ownerRef, jobName, jobNamespace, jobType, collect, configMap, image, pullPolicy)
	if err != nil {
		return nil, nil, err
	}

	return configMap, pod, nil
}

func createCollectorSpecConfigMap(client client.Client, scheme *runtime.Scheme, ownerRef metav1.Object, jobName string, jobNamespace string, collect *troubleshootv1beta1.Collect) (*corev1.ConfigMap, error) {
	name := fmt.Sprintf("%s-%s", jobName, DeterministicIDForCollector(collect))
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: jobNamespace,
	}

	found := &corev1.ConfigMap{}
	err := client.Get(context.Background(), namespacedName, found)
	if err == nil || !kuberneteserrors.IsNotFound(err) {
		return nil, err
	}

	specContents, err := yaml.Marshal(collect)
	if err != nil {
		return nil, err
	}

	specData := make(map[string]string)
	specData["collector.yaml"] = string(specContents)

	configMap := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: jobNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		Data: specData,
	}

	if scheme != nil {
		if err := controllerutil.SetControllerReference(ownerRef, &configMap, scheme); err != nil {
			return nil, err
		}
	}

	if err := client.Create(context.Background(), &configMap); err != nil {
		return nil, err
	}

	return &configMap, nil
}

func createCollectorPod(client client.Client, scheme *runtime.Scheme, ownerRef metav1.Object, jobName string, jobNamespace string, jobType string, collect *troubleshootv1beta1.Collect, configMap *corev1.ConfigMap, image string, pullPolicy string) (*corev1.Pod, error) {
	name := fmt.Sprintf("%s-%s", jobName, DeterministicIDForCollector(collect))

	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: jobNamespace,
	}

	found := &corev1.Pod{}
	err := client.Get(context.Background(), namespacedName, found)
	if err == nil || !kuberneteserrors.IsNotFound(err) {
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

	podLabels[jobType] = jobName
	podLabels["troubleshoot-role"] = jobType

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: jobNamespace,
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
					Image:           imageName,
					ImagePullPolicy: imagePullPolicy,
					Name:            DeterministicIDForCollector(collect),
					Command:         []string{"collector"},
					Args: []string{
						"run",
						"--collector",
						"/troubleshoot/specs/collector.yaml",
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "collector",
							MountPath: "/troubleshoot/specs",
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
			},
		},
	}

	if scheme != nil {
		if err := controllerutil.SetControllerReference(ownerRef, &pod, scheme); err != nil {
			return nil, err
		}
	}

	if err := client.Create(context.Background(), &pod); err != nil {
		return nil, err
	}

	return &pod, nil
}
