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
	nodeData := []corev1.Node{
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
					"pods":              resource.MustParse("15"),
				},
				Allocatable: corev1.ResourceList{
					"cpu":               resource.MustParse("1.5"),
					"ephemeral-storage": resource.MustParse("19316009748"),
					"memory":            resource.MustParse("16Ki"),
					"pods":              resource.MustParse("14"),
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
					"cpu":               resource.MustParse("4"),
					"ephemeral-storage": resource.MustParse("10959212Ki"),
					"memory":            resource.MustParse("7951376Ki"),
					"pods":              resource.MustParse("29"),
				},
				Allocatable: corev1.ResourceList{
					"cpu":               resource.MustParse("3"),
					"ephemeral-storage": resource.MustParse("12316009748"),
					"memory":            resource.MustParse("7848976Ki"),
					"pods":              resource.MustParse("12"),
				},
			},
		},
	}

	tests := []struct {
		name           string
		conditional    string
		totalNodeCount int
		matchingNodes  []corev1.Node
		expected       bool
	}{
		{
			name:           "=",
			conditional:    "= 2",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "count()",
			conditional:    "count() == 2",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "<",
			conditional:    "< 3",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "count() <",
			conditional:    "count() < 3",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           ">",
			conditional:    "> 2",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       false,
		},
		{
			name:           "count() >",
			conditional:    "count() > 1",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "count() >= 1 (true)",
			conditional:    "count() > 1",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "count() <= 2 (true)",
			conditional:    "count() <= 2",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "count() <= 1 (false)",
			conditional:    "count() <= 1",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       false,
		},
		{
			name:           "min(memoryCapacity) < 4Gi (true)",
			conditional:    "min(memoryCapacity) < 4Gi",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "min(memoryCapacity) >= 4Gi (false)",
			conditional:    "min(memoryCapacity) >= 4Gi",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       false,
		},
		{
			name:           "min(memoryAllocatable) == 16Ki (true)",
			conditional:    "min(memoryAllocatable) == 16Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "min(cpuCapacity) == 2 (true)",
			conditional:    "min(cpuCapacity) == 2",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "min(cpuAllocatable) == 1.5 (true)",
			conditional:    "min(cpuAllocatable) == 1.5",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "min(podCapacity) == 15 (true)",
			conditional:    "min(podCapacity) == 15",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "min(podAllocatable) == 12 (true)",
			conditional:    "min(podAllocatable) == 12",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "min(ephemeralStorageCapacity) <= 20Gi (true)",
			conditional:    "min(ephemeralStorageCapacity) <= 20Gi",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "min(ephemeralStorageCapacity) > 20Gi (false)",
			conditional:    "min(ephemeralStorageCapacity) > 20Gi",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       false,
		},
		{
			name:           "min(ephemeralStorageAllocatable) == 12316009748 (true)",
			conditional:    "min(ephemeralStorageAllocatable) == 12316009748",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "max(memoryCapacity) == 7951376Ki (true)",
			conditional:    "max(memoryCapacity) == 7951376Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "max(memoryAllocatable) == 7848976Ki (true)",
			conditional:    "max(memoryAllocatable) == 7848976Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "max(cpuCapacity) == 12 (false)",
			conditional:    "max(cpuCapacity) == 12",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       false,
		},
		{
			name:           "max(cpuCapacity) == 4 (true)",
			conditional:    "max(cpuCapacity) == 4",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "max(cpuAllocatable) == 3 (true)",
			conditional:    "max(cpuAllocatable) == 3",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "max(podCapacity) == 29 (true)",
			conditional:    "max(podCapacity) == 29",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "max(podAllocatable) == 14 (true)",
			conditional:    "max(podAllocatable) == 14",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "max(ephemeralStorageCapacity) == 20959212Ki (true)",
			conditional:    "max(ephemeralStorageCapacity) == 20959212Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "max(ephemeralStorageAllocatable) == 19316009748 (true)",
			conditional:    "max(ephemeralStorageAllocatable) == 19316009748",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "sum(memoryCapacity) > 7951376Ki (true)",
			conditional:    "sum(memoryCapacity) > 7951376Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "sum(memoryAllocatable) > 7848976Ki (true)",
			conditional:    "sum(memoryAllocatable) > 7848976Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "sum(cpuCapacity) > 5 (true)",
			conditional:    "sum(cpuCapacity) > 5",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "sum(cpuAllocatable) == 4.5 (true)",
			conditional:    "sum(cpuAllocatable) == 4.5",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "sum(podCapacity) == 44 (true)",
			conditional:    "sum(podCapacity) == 44",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "sum(podAllocatable) == 26 (true)",
			conditional:    "sum(podAllocatable) == 26",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "sum(ephemeralStorageCapacity) > 20959212Ki (true)",
			conditional:    "sum(ephemeralStorageCapacity) > 20959212Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
		},
		{
			name:           "sum(ephemeralStorageAllocatable) > 19316009748 (true)",
			conditional:    "sum(ephemeralStorageAllocatable) > 19316009748",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
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
	node := corev1.Node{
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
	}

	tests := []struct {
		name         string
		node         corev1.Node
		filters      *troubleshootv1beta1.NodeResourceFilters
		expectResult bool
	}{
		{
			name:         "true when empty filters",
			node:         node,
			filters:      &troubleshootv1beta1.NodeResourceFilters{},
			expectResult: true,
		},
		{
			name:         "true while nil/missing filters",
			node:         node,
			expectResult: true,
		},
		{
			name: "false when allocatable memory is too high",
			node: node,
			filters: &troubleshootv1beta1.NodeResourceFilters{
				MemoryAllocatable: "16Gi",
			},
			expectResult: false,
		},
		{
			name: "true when allocatable memory is available",
			node: node,
			filters: &troubleshootv1beta1.NodeResourceFilters{
				MemoryAllocatable: "4Gi",
			},
			expectResult: true,
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
