package collect

import (
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/require"
)

func Test_interfaces(t *testing.T) {
	runCollector := &v1beta2.Run{}
	clusterInfoCollector := &v1beta2.ClusterInfo{}

	tests := []struct {
		name       string
		collect    *v1beta2.Collect
		want       interface{}
		wantRunner bool
	}{
		{
			name: "image runner collector",
			collect: &v1beta2.Collect{
				Run: runCollector,
			},
			want:       runCollector,
			wantRunner: true,
		},
		{
			name: "not image runner collector",
			collect: &v1beta2.Collect{
				ClusterInfo: clusterInfoCollector,
			},
			want:       clusterInfoCollector,
			wantRunner: false,
		},
		{
			name:       "no collector",
			collect:    &v1beta2.Collect{},
			want:       nil,
			wantRunner: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			got := v1beta2.GetCollector(tt.collect)
			req.EqualValues(tt.want, got)

			runner, ok := got.(ImageRunner)
			req.EqualValues(tt.wantRunner, ok)
			if tt.wantRunner {
				req.EqualValues(tt.want, runner)
			}
		})
	}
}
