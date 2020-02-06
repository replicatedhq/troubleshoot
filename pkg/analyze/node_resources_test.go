package analyzer

import (
	"testing"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_compareNodeResourceConditionalToActual(t *testing.T) {
	tests := []struct {
		name           string
		conditional    string
		totalNodeCount int
		matchingNodes  []corev1.Node
		expected       bool
	}{
		{
			name:        "=",
			conditional: "= 2",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
			},
			totalNodeCount: 2,
			expected:       true,
		},
		{
			name:        "count()",
			conditional: "count() == 2",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
			},
			totalNodeCount: 2,
			expected:       true,
		},
		{
			name:        "<",
			conditional: "< 3",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
			},
			totalNodeCount: 2,
			expected:       true,
		},
		{
			name:        "count() <",
			conditional: "count() < 3",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
			},
			totalNodeCount: 2,
			expected:       true,
		},
		{
			name:        ">",
			conditional: "> 2",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
			},
			totalNodeCount: 2,
			expected:       false,
		},
		{
			name:        "count() >",
			conditional: "count() > 1",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
			},
			totalNodeCount: 2,
			expected:       true,
		},
		{
			name:        "count() >= 1 (true)",
			conditional: "count() > 1",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
			},
			totalNodeCount: 2,
			expected:       true,
		},
		{
			name:        "count() <= 2 (true)",
			conditional: "count() <= 2",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
			},
			totalNodeCount: 2,
			expected:       true,
		},
		{
			name:        "count() <= 1 (false)",
			conditional: "count() <= 1",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
				},
			},
			totalNodeCount: 2,
			expected:       false,
		},
		{
			name:        "min(memoryCapacity) <= 4Gi (true)",
			conditional: "min(memoryCapacity) <= 4Gi",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("3999Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("16Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("7951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
			},
			totalNodeCount: 2,
			expected:       true,
		},
		{
			name:        "min(memoryCapacity) <= 4Gi (false)",
			conditional: "min(memoryCapacity) <= 4Gi",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("17951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("7951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
			},
			totalNodeCount: 2,
			expected:       false,
		},
		{
			name:        "max(cpuCapacity) == 12 (false)",
			conditional: "max(cpuCapacity) == 12",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("17951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("7951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
			},
			totalNodeCount: 2,
			expected:       false,
		},
		{
			name:        "max(cpuCapacity) == 2 (true)",
			conditional: "max(cpuCapacity) == 2",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("17951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("7951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("2"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
			},
			totalNodeCount: 2,
			expected:       true,
		},
		{
			name:        "sum(cpuCapacity) > 32 (true)",
			conditional: "sum(cpuCapacity) > 32",
			matchingNodes: []corev1.Node{
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node1",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("8"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("17951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("8"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node2",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("8"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("7951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("8"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node3",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("8"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("7951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("8"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node4",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("8"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("7951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("8"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
				corev1.Node{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "Node",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "node5",
					},
					Status: corev1.NodeStatus{
						Capacity: corev1.ResourceList{
							"cpu":               resource.MustParse("8"),
							"ephemeral-storage": resource.MustParse("20959212Ki"),
							"memory":            resource.MustParse("7951376Ki"),
							"pods":              resource.MustParse("29"),
						},
						Allocatable: corev1.ResourceList{
							"cpu":               resource.MustParse("8"),
							"ephemeral-storage": resource.MustParse("19316009748"),
							"memory":            resource.MustParse("7848976Ki"),
							"pods":              resource.MustParse("29"),
						},
					},
				},
			},
			totalNodeCount: 2,
			expected:       true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := compareNodeResourceConditionalToActual(test.conditional, test.matchingNodes, test.totalNodeCount)
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
