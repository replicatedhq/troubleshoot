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
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func Test_setLogLimits(t *testing.T) {
	maxBytes := int64(5000000)
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
			name: "max bytes",
			limits: &troubleshootv1beta2.LogLimits{
				MaxBytes: maxBytes,
			},
			expected: corev1.PodLogOptions{
				LimitBytes: &maxBytes,
				TailLines:  &defaultMaxLines,
			},
		},

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

			actual := corev1.PodLogOptions{}
			setLogLimits(&actual, test.limits, convertMaxAgeToTime)

			if test.expected.LimitBytes != nil {
				assert.NotNil(t, actual.LimitBytes)
				assert.Equal(t, *test.expected.LimitBytes, *actual.LimitBytes)
			}

			if test.expected.TailLines != nil {
				assert.NotNil(t, actual.TailLines)
				assert.Equal(t, *test.expected.TailLines, *actual.TailLines)
			} else {
				assert.Nil(t, actual.TailLines)
			}

			if test.expected.SinceTime != nil {
				assert.NotNil(t, actual.SinceTime)
				assert.Equal(t, *test.expected.SinceTime, *actual.SinceTime)
			} else {
				assert.Nil(t, actual.SinceTime)
			}
		})
	}
}

func Test_savePodLogs(t *testing.T) {
	tests := []struct {
		name              string
		withContainerName bool
		collectorName     string
		createSymLinks    bool
		timestamps        bool
		want              CollectorResult
	}{
		{
			name:              "with container name",
			withContainerName: true,
			collectorName:     "all-logs",
			createSymLinks:    true,
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
			createSymLinks:    true,
			want: CollectorResult{
				"all-logs/test-pod.log":                                                []byte("fake logs"),
				"all-logs/test-pod-previous.log":                                       []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx.log":          []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx-previous.log": []byte("fake logs"),
			},
		},
		{
			name:              "without container or collector names",
			withContainerName: false,
			createSymLinks:    true,
			want: CollectorResult{
				"/test-pod.log":          []byte("fake logs"),
				"/test-pod-previous.log": []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx.log":          []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx-previous.log": []byte("fake logs"),
			},
		},
		{
			name:              "without sym links",
			withContainerName: true,
			collectorName:     "all-logs",
			createSymLinks:    false,
			want: CollectorResult{
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx.log":          []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/test-pod/nginx-previous.log": []byte("fake logs"),
			},
		},
		{
			name:              "with timestamps",
			withContainerName: true,
			collectorName:     "all-logs",
			timestamps:        true,
			want: CollectorResult{
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
				MaxBytes: 10000000,
			}
			pod, err := createPod(client, containerName, "test-pod", "my-namespace")
			require.NoError(t, err)
			if !tt.withContainerName {
				containerName = ""
			}
			got, err := savePodLogs(ctx, "", client, pod, tt.collectorName, containerName, limits, false, tt.createSymLinks, tt.timestamps)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_CollectLogs(t *testing.T) {
	tests := []struct {
		name          string
		collectorName string
		podNames      []string
		want          CollectorResult
	}{
		{
			name:          "from multiple pods",
			collectorName: "all-logs",
			podNames: []string{
				"firstPod",
				"secondPod",
			},
			want: CollectorResult{
				"all-logs/firstPod/nginx.log":                                           []byte("fake logs"),
				"all-logs/firstPod/nginx-previous.log":                                  []byte("fake logs"),
				"all-logs/secondPod/nginx.log":                                          []byte("fake logs"),
				"all-logs/secondPod/nginx-previous.log":                                 []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/firstPod/nginx.log":           []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/firstPod/nginx-previous.log":  []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/secondPod/nginx.log":          []byte("fake logs"),
				"cluster-resources/pods/logs/my-namespace/secondPod/nginx-previous.log": []byte("fake logs"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			ns := "my-namespace"
			client := testclient.NewSimpleClientset()

			for _, podName := range tt.podNames {
				_, err := createPod(client, "nginx", podName, ns)
				require.NoError(t, err)
			}

			progresChan := make(chan any)
			c := &CollectLogs{
				Context:   ctx,
				Namespace: ns,
				Collector: &troubleshootv1beta2.Logs{
					Name: tt.collectorName,
				},
			}
			got, err := c.CollectWithClient(progresChan, client)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func createPod(client kubernetes.Interface, containerName, podName, ns string) (*corev1.Pod, error) {
	return client.CoreV1().Pods(ns).Create(context.TODO(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name: containerName,
				},
			},
		},
	}, metav1.CreateOptions{})
}
