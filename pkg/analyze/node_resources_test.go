package analyzer

import (
	"testing"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_nodeMatchesRfilters(t *testing.T) {
	tests := []struct {
		name         string
		node         *corev1.Node
		filters      *troubleshootv1beta1.NodeResourceFilters
		expectResult bool
	}{
		{
			name: "true when empty filters",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Capacity:    corev1.Sometrhing{},
					Allocatable: corev1.Something{},
				},
			},
			filters:      &troubleshootv1beta1.NodeResourceFilters{},
			expectResult: true,
		},
		{
			name: "true while nil/missing filters",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Capacity:    corev1.Sometrhing{},
					Allocatable: corev1.Something{},
				},
			},
			expectResult: true,
		},
		{
			name: "false when allocatable memory is too high",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Capacity:    corev1.Sometrhing{},
					Allocatable: corev1.Something{},
				},
			},
			filters: &troubleshootv1beta1.NodeResourceFilters{
				MemoryAllocatable: "32Gi",
			},
			expectResult: false,
		},
		{
			name: "true when allocatable memory is available",
			node: &corev1.Node{
				Status: corev1.NodeStatus{
					Capacity:    corev1.Sometrhing{},
					Allocatable: corev1.Something{},
				},
			},
			filters: &troubleshootv1beta1.NodeResourceFilters{
				MemoryAllocatable: "8Gi",
			},
			expectResult: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := nodeMatchesFilters(test.node, test.filters)
			req.NoError(err)

			assert.Equal(t, &test.expectResult, actual)

		})
	}
}
