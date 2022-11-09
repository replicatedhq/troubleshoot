package collect

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
)

func Test_ensureClusterResourcesFirst(t *testing.T) {

	testCases := []struct {
		name string
		want []*troubleshootv1beta2.Collect
		list []*troubleshootv1beta2.Collect
	}{
		{
			name: "Reorg OK",
			want: []*troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{},
				},
				{
					Data: &troubleshootv1beta2.Data{},
				},
			},
			list: []*troubleshootv1beta2.Collect{
				{
					Data: &troubleshootv1beta2.Data{},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{},
				},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := EnsureClusterResourcesFirst(tc.list)
			assert.Equal(t, tc.want, got)
		})
	}
}
