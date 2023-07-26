package collect

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func Test_checkDaemonsePodStatus(t *testing.T) {
	tests := []struct {
		name         string
		namespace    string
		podStatus    corev1.PodPhase
		mockPod      *corev1.Pod
		mockEvent    *corev1.Event
		labels       map[string]string
		expectedErr  bool
		eventMessage string
	}{
		{
			name:      "Pod running without FailedMount event",
			namespace: "test",
			podStatus: corev1.PodRunning,
			mockPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
					Labels:    map[string]string{"test": "true"},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			expectedErr: false,
		},
		{
			name:      "Pod not running without FailedMount event",
			namespace: "test",
			labels:    map[string]string{"app.kubernetes.io/managed-by": "troubleshoot.sh"},
			podStatus: corev1.PodPending,
			mockPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
					Labels:    map[string]string{"app.kubernetes.io/managed-by": "troubleshoot.sh"},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			expectedErr: false,
		},
		{
			name:      "Pod running with FailedMount event",
			namespace: "test",
			podStatus: corev1.PodRunning,
			mockPod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test",
					Labels:    map[string]string{"app.kubernetes.io/managed-by": "troubleshoot.sh"},
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodPending,
				},
			},
			mockEvent: &corev1.Event{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-event",
					Namespace: "test",
				},
				Reason: "FailedMount",
			},
			expectedErr:  true,
			eventMessage: `MountVolume.SetUp failed for volume "host" : hostPath type check failed: /var/lib/collectd is not a directory`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := testclient.NewSimpleClientset()

			if tt.mockPod != nil {
				pod, err := client.CoreV1().Pods(tt.namespace).Create(ctx, tt.mockPod, metav1.CreateOptions{})
				require.NoError(t, err)

				if tt.mockEvent != nil {
					event := tt.mockEvent
					event.InvolvedObject = corev1.ObjectReference{
						UID: pod.UID,
					}
					_, err = client.CoreV1().Events(tt.namespace).Create(ctx, event, metav1.CreateOptions{})
					require.NoError(t, err)
				}
			}

			err := checkDaemonsePodStatus(client, ctx, tt.labels, tt.namespace)
			if tt.expectedErr {
				require.Error(t, err)
				if tt.mockEvent != nil {
					require.Contains(t, err.Error(), "path does not exist")
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}
