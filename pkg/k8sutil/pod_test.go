package k8sutil

import (
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"testing"
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
