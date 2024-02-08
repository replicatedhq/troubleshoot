package collect

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestWaitForDaemonSetPods(t *testing.T) {
	ctx := context.TODO()
	client := fake.NewSimpleClientset()

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "connectivity",
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 2,
			CurrentNumberScheduled: 2,
		},
	}

	_, err := client.AppsV1().DaemonSets("default").Create(ctx, ds, metav1.CreateOptions{})
	assert.NoError(t, err)

	err = waitForDaemonSetPods(ctx, client, ds)
	assert.NoError(t, err)
}
func TestGetPodNodeAtCompletion(t *testing.T) {
	ctx := context.TODO()
	client := fake.NewSimpleClientset()

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "test-pod",
		},
		Spec: corev1.PodSpec{
			NodeName: "foo-node",
			Containers: []corev1.Container{
				{
					Name:  "connectivity",
					Image: "curlimages/curl",
					Args:  []string{"-IsL", "https://docs.replicated.com"},
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "connectivity",
					RestartCount: 1,
				},
			},
		},
	}
	_, err := client.CoreV1().Pods("default").Create(ctx, &pod, metav1.CreateOptions{})
	assert.NoError(t, err)

	nodeName, err := getPodNodeAtCompletion(ctx, client.CoreV1(), pod)
	assert.NoError(t, err)
	assert.Equal(t, "foo-node", nodeName)
}
