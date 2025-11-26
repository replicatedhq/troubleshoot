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
			name: "Reorg OK - clusterResources moved to front",
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
		{
			name: "Already first - no change",
			want: []*troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{},
				},
				{
					Data: &troubleshootv1beta2.Data{},
				},
				{
					Secret: &troubleshootv1beta2.Secret{},
				},
			},
			list: []*troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{},
				},
				{
					Data: &troubleshootv1beta2.Data{},
				},
				{
					Secret: &troubleshootv1beta2.Secret{},
				},
			},
		},
		{
			name: "Multiple clusterResources - all moved to front",
			want: []*troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{},
				},
				{
					Data: &troubleshootv1beta2.Data{},
				},
				{
					Secret: &troubleshootv1beta2.Secret{},
				},
			},
			list: []*troubleshootv1beta2.Collect{
				{
					Data: &troubleshootv1beta2.Data{},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{},
				},
				{
					Secret: &troubleshootv1beta2.Secret{},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{},
				},
			},
		},
		{
			name: "No clusterResources - no change",
			want: []*troubleshootv1beta2.Collect{
				{
					Data: &troubleshootv1beta2.Data{},
				},
				{
					Secret: &troubleshootv1beta2.Secret{},
				},
			},
			list: []*troubleshootv1beta2.Collect{
				{
					Data: &troubleshootv1beta2.Data{},
				},
				{
					Secret: &troubleshootv1beta2.Secret{},
				},
			},
		},
		{
			name: "Empty list - no change",
			want: []*troubleshootv1beta2.Collect{},
			list: []*troubleshootv1beta2.Collect{},
		},
		{
			name: "Only clusterResources - no change",
			want: []*troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{},
				},
			},
			list: []*troubleshootv1beta2.Collect{
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
