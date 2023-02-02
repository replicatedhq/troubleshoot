package collect

import (
	"os"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/require"
)

func TestCollectHostSubnetAvailable_Collect(t *testing.T) {
	type fields struct {
		hostCollector *troubleshootv1beta2.SubnetAvailable
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string][]byte
	}{
		{
			name: "TODO",
			want: map[string][]byte{
				"host-collectors/subnetAvailable/subnetAvailable.json": []byte(`{"status":"connected","message":""}`),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "bundle")
			require.NoError(t, err)

			c := &CollectHostSubnetAvailable{
				hostCollector: &troubleshootv1beta2.SubnetAvailable{
					// TODO: implement
				},
				BundlePath: tmpDir,
			}
		})
	}
}
