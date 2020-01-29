package analyzer

import (
	"testing"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_compareNodeResourceConditionalToActual(t *testing.T) {
	tests := []struct {
		name        string
		conditional string
		actual      int
		expected    bool
	}{
		{
			name:        "=",
			conditional: "= 5",
			actual:      5,
			expected:    true,
		},
		{
			name:        "<= (pass)",
			conditional: "<= 5",
			actual:      4,
			expected:    true,
		},
		{
			name:        "<= (fail)",
			conditional: "<= 5",
			actual:      6,
			expected:    false,
		},
		{
			name:        "> (pass)",
			conditional: "> 5",
			actual:      6,
			expected:    true,
		},
		{
			name:        ">=(fail)",
			conditional: ">= 5",
			actual:      4,
			expected:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := compareNodeResourceConditionalToActual(test.conditional, test.actual)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)

		})
	}
}

func Test_nodeMatchesFilters(t *testing.T) {
	tests := []struct {
		name         string
		node         corev1.Node
		filters      *troubleshootv1beta1.NodeResourceFilters
		expectResult bool
	}{
		{
			name: "true when empty filters",
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Capacity: corev1.ResourceList{
						"attachable-volumes-aws-ebs": resource.MustParse("25"),
						"cpu":                        resource.MustParse("2"),
						"ephemeral-storage":          resource.MustParse("20959212Ki"),
						"hugepages-1Gi":              resource.MustParse("0"),
						"hugepages-2Mi":              resource.MustParse("0"),
						"memory":                     resource.MustParse("7951376Ki"),
						"pods":                       resource.MustParse("29"),
					},
					Allocatable: corev1.ResourceList{
						"attachable-volumes-aws-ebs": resource.MustParse("25"),
						"cpu":                        resource.MustParse("2"),
						"ephemeral-storage":          resource.MustParse("19316009748"),
						"hugepages-1Gi":              resource.MustParse("0"),
						"hugepages-2Mi":              resource.MustParse("0"),
						"memory":                     resource.MustParse("7848976Ki"),
						"pods":                       resource.MustParse("29"),
					},
				},
			},
			filters:      &troubleshootv1beta1.NodeResourceFilters{},
			expectResult: true,
		},
		{
			name: "true while nil/missing filters",
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Capacity: corev1.ResourceList{
						"attachable-volumes-aws-ebs": resource.MustParse("25"),
						"cpu":                        resource.MustParse("2"),
						"ephemeral-storage":          resource.MustParse("20959212Ki"),
						"hugepages-1Gi":              resource.MustParse("0"),
						"hugepages-2Mi":              resource.MustParse("0"),
						"memory":                     resource.MustParse("7951376Ki"),
						"pods":                       resource.MustParse("29"),
					},
					Allocatable: corev1.ResourceList{
						"attachable-volumes-aws-ebs": resource.MustParse("25"),
						"cpu":                        resource.MustParse("2"),
						"ephemeral-storage":          resource.MustParse("19316009748"),
						"hugepages-1Gi":              resource.MustParse("0"),
						"hugepages-2Mi":              resource.MustParse("0"),
						"memory":                     resource.MustParse("7848976Ki"),
						"pods":                       resource.MustParse("29"),
					},
				},
			},
			expectResult: true,
		},
		{
			name: "false when allocatable memory is too high",
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Capacity: corev1.ResourceList{
						"attachable-volumes-aws-ebs": resource.MustParse("25"),
						"cpu":                        resource.MustParse("2"),
						"ephemeral-storage":          resource.MustParse("20959212Ki"),
						"hugepages-1Gi":              resource.MustParse("0"),
						"hugepages-2Mi":              resource.MustParse("0"),
						"memory":                     resource.MustParse("7951376Ki"),
						"pods":                       resource.MustParse("29"),
					},
					Allocatable: corev1.ResourceList{
						"attachable-volumes-aws-ebs": resource.MustParse("25"),
						"cpu":                        resource.MustParse("2"),
						"ephemeral-storage":          resource.MustParse("19316009748"),
						"hugepages-1Gi":              resource.MustParse("0"),
						"hugepages-2Mi":              resource.MustParse("0"),
						"memory":                     resource.MustParse("7848976Ki"),
						"pods":                       resource.MustParse("29"),
					},
				},
			},
			filters: &troubleshootv1beta1.NodeResourceFilters{
				MemoryAllocatable: "16Gi",
			},
			expectResult: false,
		},
		{
			name: "true when allocatable memory is available",
			node: corev1.Node{
				Status: corev1.NodeStatus{
					Capacity: corev1.ResourceList{
						"attachable-volumes-aws-ebs": resource.MustParse("25"),
						"cpu":                        resource.MustParse("2"),
						"ephemeral-storage":          resource.MustParse("20959212Ki"),
						"hugepages-1Gi":              resource.MustParse("0"),
						"hugepages-2Mi":              resource.MustParse("0"),
						"memory":                     resource.MustParse("7951376Ki"),
						"pods":                       resource.MustParse("29"),
					},
					Allocatable: corev1.ResourceList{
						"attachable-volumes-aws-ebs": resource.MustParse("25"),
						"cpu":                        resource.MustParse("2"),
						"ephemeral-storage":          resource.MustParse("19316009748"),
						"hugepages-1Gi":              resource.MustParse("0"),
						"hugepages-2Mi":              resource.MustParse("0"),
						"memory":                     resource.MustParse("7848976Ki"),
						"pods":                       resource.MustParse("29"),
					},
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

			assert.Equal(t, test.expectResult, actual)

		})
	}
}
