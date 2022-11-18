package collect

import (
	"context"
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func Test_setLogLimits(t *testing.T) {
	defaultMaxLines := int64(10000)
	customLines := int64(20)
	maxAge := "10h"
	sinceWhen := metav1.NewTime(time.Now().Add(-10 * time.Hour))

	convertMaxAgeToTime := func(maxAge string) *metav1.Time {
		return &sinceWhen
	}

	tests := []struct {
		name     string
		limits   *troubleshootv1beta2.LogLimits
		expected corev1.PodLogOptions
		validate func(t *testing.T, podLogOpts *corev1.PodLogOptions)
	}{
		{
			name:   "default limits",
			limits: nil,
			expected: corev1.PodLogOptions{
				TailLines: &defaultMaxLines,
			},
		},
		{
			name: "custom limit lines",
			limits: &troubleshootv1beta2.LogLimits{
				MaxLines: customLines,
			},
			expected: corev1.PodLogOptions{
				TailLines: &customLines,
			},
		},
		{
			name: "max age",
			limits: &troubleshootv1beta2.LogLimits{
				MaxAge: maxAge,
			},
			expected: corev1.PodLogOptions{
				SinceTime: &sinceWhen,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual := corev1.PodLogOptions{}
			setLogLimits(&actual, test.limits, convertMaxAgeToTime)

			if test.expected.TailLines != nil {
				req.NotNil(actual.TailLines)
				assert.Equal(t, *test.expected.TailLines, *actual.TailLines)
			} else {
				req.Nil(actual.TailLines)
			}

			if test.expected.SinceTime != nil {
				req.NotNil(actual.SinceTime)
				assert.Equal(t, *test.expected.SinceTime, *actual.SinceTime)
			} else {
				req.Nil(actual.SinceTime)
			}
		})
	}
}

func Test_savePodLogs(t *testing.T) {
	tests := []struct {
		name              string
		withContainerName bool
		collectorName     string
		want              CollectorResult
	}{
		{
			name:              "with container name",
			withContainerName: true,
			collectorName:     "all-logs",
			want: CollectorResult{
				"all-logs/test-pod/nginx.log":                                          []byte("fake logs"),
				"all-logs/test-pod/nginx-previous.log":                                 []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx.log":          []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx-previous.log": []byte("fake logs"),
			},
		},
		{
			name:              "without container name",
			withContainerName: false,
			collectorName:     "all-logs",
			want: CollectorResult{
				"all-logs/test-pod.log":                                                []byte("fake logs"),
				"all-logs/test-pod-previous.log":                                       []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx.log":          []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx-previous.log": []byte("fake logs"),
			},
		},
		{
			name:              "without container name or collector name",
			withContainerName: false,
			want: CollectorResult{
				"/test-pod.log":          []byte("fake logs"),
				"/test-pod-previous.log": []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx.log":          []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx-previous.log": []byte("fake logs"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			containerName := "nginx"
			client := testclient.NewSimpleClientset()
			limits := &troubleshootv1beta2.LogLimits{
				MaxLines: 500,
			}
			pod, err := client.CoreV1().Pods("my-namespace").Create(ctx, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: containerName,
						},
					},
				},
			}, metav1.CreateOptions{})
			assert.NoError(t, err)
			if !tt.withContainerName {
				containerName = ""
			}
			got, err := savePodLogs(ctx, "", client, pod, tt.collectorName, containerName, limits, false)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
