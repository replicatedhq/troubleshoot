package collect

import (
	"context"
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreatePodStruct(t *testing.T) {
	runPodCollector := &troubleshootv1beta2.RunPod{
		Namespace: "test-namespace",
		Name:      "test-pod",
		Annotations: map[string]string{
			"annotation1": "value1",
			"annotation2": "value2",
		},
		PodSpec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
				},
			},
		},
	}

	expectedPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-pod",
			Namespace:   "test-namespace",
			Labels:      map[string]string{"troubleshoot-role": "run-collector"},
			Annotations: map[string]string{"annotation1": "value1", "annotation2": "value2"},
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "test-image",
				},
			},
		},
	}

	pod := createPodStruct(runPodCollector)

	if pod.Name != expectedPod.Name {
		t.Errorf("Expected pod name %s, but got %s", expectedPod.Name, pod.Name)
	}

	if pod.Namespace != expectedPod.Namespace {
		t.Errorf("Expected pod namespace %s, but got %s", expectedPod.Namespace, pod.Namespace)
	}

	if len(pod.Labels) != len(expectedPod.Labels) {
		t.Errorf("Expected %d labels, but got %d", len(expectedPod.Labels), len(pod.Labels))
	}

	for key, value := range expectedPod.Labels {
		if pod.Labels[key] != value {
			t.Errorf("Expected label %s=%s, but got %s=%s", key, value, key, pod.Labels[key])
		}
	}

	if len(pod.Annotations) != len(expectedPod.Annotations) {
		t.Errorf("Expected %d annotations, but got %d", len(expectedPod.Annotations), len(pod.Annotations))
	}

	for key, value := range expectedPod.Annotations {
		if pod.Annotations[key] != value {
			t.Errorf("Expected annotation %s=%s, but got %s=%s", key, value, key, pod.Annotations[key])
		}
	}

	if len(pod.Spec.Containers) != len(expectedPod.Spec.Containers) {
		t.Errorf("Expected %d containers, but got %d", len(expectedPod.Spec.Containers), len(pod.Spec.Containers))
	}

	for i, container := range expectedPod.Spec.Containers {
		if pod.Spec.Containers[i].Name != container.Name {
			t.Errorf("Expected container name %s, but got %s", container.Name, pod.Spec.Containers[i].Name)
		}

		if pod.Spec.Containers[i].Image != container.Image {
			t.Errorf("Expected container image %s, but got %s", container.Image, pod.Spec.Containers[i].Image)
		}
	}
}

func Test_deleteImagePullSecret(t *testing.T) {
	tests := []struct {
		name         string
		pod          *corev1.Pod
		existingObjs []runtime.Object
		validateFunc func(t *testing.T, client *fake.Clientset)
	}{
		{
			name: "successfully deletes managed secret",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "managed-secret"},
					},
				},
			},
			existingObjs: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "managed-secret",
						Namespace: "test-ns",
						Labels: map[string]string{
							"app.kubernetes.io/managed-by": "troubleshoot.sh",
						},
					},
				},
			},
			validateFunc: func(t *testing.T, client *fake.Clientset) {
				// Secret should be deleted
				_, err := client.CoreV1().Secrets("test-ns").Get(context.Background(), "managed-secret", metav1.GetOptions{})
				require.True(t, kuberneteserrors.IsNotFound(err))
			},
		},
		{
			name: "does not delete unmanaged secret",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "unmanaged-secret"},
					},
				},
			},
			existingObjs: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unmanaged-secret",
						Namespace: "test-ns",
					},
				},
			},
			validateFunc: func(t *testing.T, client *fake.Clientset) {
				// Secret should still exist
				secret, err := client.CoreV1().Secrets("test-ns").Get(context.Background(), "unmanaged-secret", metav1.GetOptions{})
				require.NoError(t, err)
				assert.NotNil(t, secret)
			},
		},
		{
			name: "handles non-existent secret",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "non-existent-secret"},
					},
				},
			},
			existingObjs: []runtime.Object{},
			validateFunc: func(t *testing.T, client *fake.Clientset) {
				// No error should occur
			},
		},
		{
			name: "does everything all at once",
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "unmanaged-secret"},
						{Name: "non-existent-secret"},
						{Name: "managed-secret"},
					},
				},
			},
			existingObjs: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "managed-secret",
						Namespace: "test-ns",
						Labels: map[string]string{
							"app.kubernetes.io/managed-by": "troubleshoot.sh",
						},
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "unmanaged-secret",
						Namespace: "test-ns",
					},
				},
			},
			validateFunc: func(t *testing.T, client *fake.Clientset) {
				// Secret should be deleted
				_, err := client.CoreV1().Secrets("test-ns").Get(context.Background(), "managed-secret", metav1.GetOptions{})
				require.True(t, kuberneteserrors.IsNotFound(err))

				// Secret should still exist
				secret, err := client.CoreV1().Secrets("test-ns").Get(context.Background(), "unmanaged-secret", metav1.GetOptions{})
				require.NoError(t, err)
				assert.NotNil(t, secret)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset(tt.existingObjs...)
			collector := &CollectRunPod{}

			collector.deleteImagePullSecret(context.Background(), client, tt.pod)

			if tt.validateFunc != nil {
				tt.validateFunc(t, client)
			}
		})
	}
}

func TestSavePodDetails_StripsPodSpec(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-run-pod",
			Namespace: "default",
			Labels: map[string]string{
				"troubleshoot-role": "run-collector",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "test-node",
			Containers: []corev1.Container{
				{
					Name:    "collector",
					Image:   "busybox",
					Command: []string{"sh", "-c", "echo secret-command"},
					Args:    []string{"--token", "super-secret-token"},
					Env: []corev1.EnvVar{
						{Name: "PASSWORD", Value: "hunter2"},
						{
							Name: "API_KEY",
							ValueFrom: &corev1.EnvVarSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
									Key:                  "api-key",
								},
							},
						},
					},
				},
			},
			ImagePullSecrets: []corev1.LocalObjectReference{{Name: "my-pull-secret"}},
			Volumes: []corev1.Volume{
				{
					Name: "secret-vol",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{SecretName: "my-secret"},
					},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "collector",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{ExitCode: 0},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(pod)
	collector := &troubleshootv1beta2.RunPod{
		Name:      "test-collector",
		Namespace: "default",
	}

	result, err := savePodDetails(context.Background(), client, NewResult(), "", pod, collector)
	require.NoError(t, err)

	podKey := "test-collector/test-collector.json"
	require.Contains(t, result, podKey)

	saved := string(result[podKey])

	// Debugging info must still be present
	assert.Contains(t, saved, `"name": "test-run-pod"`)
	assert.Contains(t, saved, `"phase": "Succeeded"`)
	assert.Contains(t, saved, `"exitCode": 0`)

	// Sensitive spec data must not be present
	assert.NotContains(t, saved, "hunter2")
	assert.NotContains(t, saved, "super-secret-token")
	assert.NotContains(t, saved, "secret-command")
	assert.NotContains(t, saved, "my-secret")
	assert.NotContains(t, saved, "my-pull-secret")
	assert.NotContains(t, saved, `"command":`)
	assert.NotContains(t, saved, `"args":`)
	assert.NotContains(t, saved, `"env":`)

	// The saved JSON must still unmarshal into a Pod so downstream consumers
	// (e.g. goldpinger) can read Name and Status.ContainerStatuses.
	var savedPod corev1.Pod
	require.NoError(t, json.Unmarshal(result[podKey], &savedPod))
	assert.Equal(t, "test-run-pod", savedPod.Name)
	assert.Equal(t, corev1.PodSucceeded, savedPod.Status.Phase)
	assert.Len(t, savedPod.Status.ContainerStatuses, 1)
	assert.Equal(t, int32(0), savedPod.Status.ContainerStatuses[0].State.Terminated.ExitCode)
	assert.Empty(t, savedPod.Spec.Containers)
	assert.Empty(t, savedPod.Spec.Volumes)
	assert.Empty(t, savedPod.Spec.ImagePullSecrets)
}
