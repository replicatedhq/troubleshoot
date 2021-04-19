package collect

import (
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
