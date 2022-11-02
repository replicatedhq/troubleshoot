package collect

import (
	"context"
	"reflect"
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
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

func TestCollectLogs_Collect(t *testing.T) {

	client := testclient.NewSimpleClientset()
	ctx := context.Background()
	var results map[string][]byte

	type fields struct {
		Collector    *troubleshootv1beta2.Logs
		BundlePath   string
		Namespace    string
		ClientConfig *rest.Config
		Client       kubernetes.Interface
		Context      context.Context
		SinceTime    *time.Time
		RBACErrors   RBACErrors
	}
	type args struct {
		progressChan chan<- interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    CollectorResult
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "test1",
			fields:  fields{BundlePath: "/tmp", Namespace: "default", Context: ctx},
			want:    results,
			wantErr: false,
		},
	}

	// need to create the kubernetes client here...

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CollectLogs{
				Collector:    tt.fields.Collector,
				BundlePath:   tt.fields.BundlePath,
				Namespace:    tt.fields.Namespace,
				ClientConfig: tt.fields.ClientConfig,
				Client:       tt.fields.Client,
				Context:      tt.fields.Context,
				SinceTime:    tt.fields.SinceTime,
				RBACErrors:   tt.fields.RBACErrors,
			}
			got, err := c.Collect(tt.args.progressChan)
			if (err != nil) != tt.wantErr {
				t.Errorf("CollectLogs.Collect() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CollectLogs.Collect() = %v, want %v", got, tt.want)
			}
		})
	}
}
