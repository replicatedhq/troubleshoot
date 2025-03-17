package collect

import (
	"context"
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
