package k8sutil

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsPodUnhealthy(t *testing.T) {
	andTrue := true
	tests := []struct {
		name string
		pod  *corev1.Pod
		want bool
	}{
		{
			name: "healthy pod with init containers",
			want: false,
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase:      "Running",
					Conditions: []corev1.PodCondition{
						// ignored here
					},
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "init",
							State: corev1.ContainerState{
								Terminated: &corev1.ContainerStateTerminated{
									ExitCode: 0,
									Reason:   "Completed",
								},
							},
							Ready:        true,
							RestartCount: 2,
							Image:        "projectcontour/contour:v1.11.0",
							ImageID:      "docker://sha256:12878e02b6f969de1456b51b0093f289fd195db956837ccd75aa679e9dac24d9",
							ContainerID:  "docker://50fc0794fa1402b1a48521e2fd380a92521a69db70ad25c9699c91fccd839d70",
						},
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "contour",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
							Ready:        true,
							RestartCount: 5,
							Image:        "projectcontour/contour:v1.11.0",
							ImageID:      "docker://sha256:12878e02b6f969de1456b51b0093f289fd195db956837ccd75aa679e9dac24d9",
							ContainerID:  "docker://50fc0794fa1402b1a48521e2fd380a92521a69db70ad25c9699c91fccd839d70",
							Started:      &andTrue,
						},
					},
					QOSClass: corev1.PodQOSBestEffort,
				},
			},
		},
		{
			name: "running pod with one unhealthy container",
			want: true,
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					Phase:      "Running",
					Conditions: []corev1.PodCondition{
						// ignored here
					},
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "envoy",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
							Ready:        false,
							RestartCount: 5,
							Started:      &andTrue,
						},
						{
							Name: "shutdown-manager",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{},
							},
							Ready:        true,
							RestartCount: 5,
							Started:      &andTrue,
						},
					},
					QOSClass: corev1.PodQOSBestEffort,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			got := IsPodUnhealthy(tt.pod)
			req.Equal(tt.want, got)
		})
	}
}

// TestGetPodStatusReason_HealthyNativeSidecar tests that a pod with a running native sidecar
// is correctly reported as "Running" and not stuck initializing.
func TestGetPodStatusReason_HealthyNativeSidecar(t *testing.T) {
	startedTrue := true
	restartPolicyAlways := corev1.ContainerRestartPolicyAlways

	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:          "istio-proxy",
					Image:         "istio/proxyv2:1.20",
					RestartPolicy: &restartPolicyAlways, // Native sidecar!
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "myapp:latest",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "istio-proxy",
					Ready:   true,
					Started: &startedTrue,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: metav1.Now(),
						},
					},
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "app",
					Ready:   true,
					Started: &startedTrue,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: metav1.Now(),
						},
					},
				},
			},
		},
	}

	reason, message := GetPodStatusReason(pod)

	// Should report as Running, not Init:0/1
	if reason != "Running" {
		t.Errorf("Expected reason 'Running', got '%s'", reason)
	}

	if message != "" {
		t.Errorf("Expected empty message, got '%s'", message)
	}
}

// TestIsPodUnhealthy_HealthyNativeSidecar tests that a pod with a healthy running native sidecar
// is not marked as unhealthy.
func TestIsPodUnhealthy_HealthyNativeSidecar(t *testing.T) {
	startedTrue := true
	restartPolicyAlways := corev1.ContainerRestartPolicyAlways

	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:          "istio-proxy",
					RestartPolicy: &restartPolicyAlways,
				},
			},
			Containers: []corev1.Container{
				{
					Name: "app",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "istio-proxy",
					Ready:   true,
					Started: &startedTrue,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "app",
					Ready:   true,
					Started: &startedTrue,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}

	unhealthy := IsPodUnhealthy(pod)

	if unhealthy {
		t.Error("Pod with healthy native sidecar should not be marked as unhealthy")
	}
}

// TestGetPodStatusReason_TraditionalInitAndNativeSidecar tests that a pod with both
// a completed traditional init container and a running native sidecar is reported as "Running".
func TestGetPodStatusReason_TraditionalInitAndNativeSidecar(t *testing.T) {
	startedTrue := true
	restartPolicyAlways := corev1.ContainerRestartPolicyAlways

	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name: "init-setup",
					// No RestartPolicy = traditional init container
				},
				{
					Name:          "istio-proxy",
					RestartPolicy: &restartPolicyAlways, // Native sidecar
				},
			},
			Containers: []corev1.Container{
				{
					Name: "app",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:   corev1.PodReady,
					Status: corev1.ConditionTrue,
				},
			},
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "init-setup",
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 0,
							Reason:   "Completed",
						},
					},
				},
				{
					Name:    "istio-proxy",
					Ready:   true,
					Started: &startedTrue,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:    "app",
					Ready:   true,
					Started: &startedTrue,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}

	reason, _ := GetPodStatusReason(pod)

	if reason != "Running" {
		t.Errorf("Expected reason 'Running', got '%s'", reason)
	}
}

// TestGetPodStatusReason_NativeSidecarCrashLoopBackOff tests that a native sidecar
// in CrashLoopBackOff is still correctly detected as an error.
func TestGetPodStatusReason_NativeSidecarCrashLoopBackOff(t *testing.T) {
	restartPolicyAlways := corev1.ContainerRestartPolicyAlways

	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:          "istio-proxy",
					RestartPolicy: &restartPolicyAlways,
				},
			},
			Containers: []corev1.Container{
				{
					Name: "app",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "istio-proxy",
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "CrashLoopBackOff",
							Message: "Back-off 5m0s restarting failed container",
						},
					},
				},
			},
		},
	}

	reason, _ := GetPodStatusReason(pod)

	// Should still catch the error
	if reason != "Init:CrashLoopBackOff" {
		t.Errorf("Expected reason 'Init:CrashLoopBackOff', got '%s'", reason)
	}
}

// TestGetPodStatusReason_TraditionalInitStuck tests that a traditional init container
// that is stuck running is still correctly detected as stuck initializing.
func TestGetPodStatusReason_TraditionalInitStuck(t *testing.T) {
	pod := &corev1.Pod{
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name: "init-setup",
					// No RestartPolicy = traditional init
				},
			},
			Containers: []corev1.Container{
				{
					Name: "app",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "init-setup",
					Ready: false,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}

	reason, _ := GetPodStatusReason(pod)

	// Traditional init running = stuck
	if reason != "Init:0/1" {
		t.Errorf("Expected reason 'Init:0/1', got '%s'", reason)
	}
}
