package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
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
		isError        bool
	}{
		{
			name:           "=",
			conditional:    "= 2",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "count()",
			conditional:    "count() == 2",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "<",
			conditional:    "< 3",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "count() <",
			conditional:    "count() < 3",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           ">",
			conditional:    "> 2",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       false,
			isError:        false,
		},
		{
			name:           "count() >",
			conditional:    "count() > 1",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "count() >= 1 (true)",
			conditional:    "count() > 1",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "count() <= 2 (true)",
			conditional:    "count() <= 2",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "count() <= 1 (false)",
			conditional:    "count() <= 1",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       false,
			isError:        false,
		},
		{
			name:           "min(memoryCapacity) < 4Gi (true)",
			conditional:    "min(memoryCapacity) < 4Gi",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "min(memoryCapacity) >= 4Gi (false)",
			conditional:    "min(memoryCapacity) >= 4Gi",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       false,
			isError:        false,
		},
		{
			name:           "min(memoryAllocatable) == 16Ki (true)",
			conditional:    "min(memoryAllocatable) == 16Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "min(cpuCapacity) == 2 (true)",
			conditional:    "min(cpuCapacity) == 2",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "min(cpuAllocatable) == 1.5 (true)",
			conditional:    "min(cpuAllocatable) == 1.5",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "min(podCapacity) == 15 (true)",
			conditional:    "min(podCapacity) == 15",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "min(podAllocatable) == 12 (true)",
			conditional:    "min(podAllocatable) == 12",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "min(ephemeralStorageCapacity) <= 20Gi (true)",
			conditional:    "min(ephemeralStorageCapacity) <= 20Gi",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "min(ephemeralStorageCapacity) > 20Gi (false)",
			conditional:    "min(ephemeralStorageCapacity) > 20Gi",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       false,
			isError:        false,
		},
		{
			name:           "min(ephemeralStorageAllocatable) == 12316009748 (true)",
			conditional:    "min(ephemeralStorageAllocatable) == 12316009748",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "max(memoryCapacity) == 7951376Ki (true)",
			conditional:    "max(memoryCapacity) == 7951376Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "max(memoryAllocatable) == 7848976Ki (true)",
			conditional:    "max(memoryAllocatable) == 7848976Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "max(cpuCapacity) == 12 (false)",
			conditional:    "max(cpuCapacity) == 12",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       false,
			isError:        false,
		},
		{
			name:           "max(cpuCapacity) == 4 (true)",
			conditional:    "max(cpuCapacity) == 4",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "max(cpuAllocatable) == 3 (true)",
			conditional:    "max(cpuAllocatable) == 3",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "max(podCapacity) == 29 (true)",
			conditional:    "max(podCapacity) == 29",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "max(podAllocatable) == 14 (true)",
			conditional:    "max(podAllocatable) == 14",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "max(ephemeralStorageCapacity) == 20959212Ki (true)",
			conditional:    "max(ephemeralStorageCapacity) == 20959212Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "max(ephemeralStorageAllocatable) == 19316009748 (true)",
			conditional:    "max(ephemeralStorageAllocatable) == 19316009748",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "sum(memoryCapacity) > 7951376Ki (true)",
			conditional:    "sum(memoryCapacity) > 7951376Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "sum(memoryAllocatable) > 7848976Ki (true)",
			conditional:    "sum(memoryAllocatable) > 7848976Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "sum(cpuCapacity) > 5 (true)",
			conditional:    "sum(cpuCapacity) > 5",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "sum(cpuAllocatable) == 4.5 (true)",
			conditional:    "sum(cpuAllocatable) == 4.5",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "sum(podCapacity) == 44 (true)",
			conditional:    "sum(podCapacity) == 44",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "sum(podAllocatable) == 26 (true)",
			conditional:    "sum(podAllocatable) == 26",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "sum(ephemeralStorageCapacity) > 20959212Ki (true)",
			conditional:    "sum(ephemeralStorageCapacity) > 20959212Ki",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "sum(ephemeralStorageAllocatable) > 19316009748 (true)",
			conditional:    "sum(ephemeralStorageAllocatable) > 19316009748",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       true,
			isError:        false,
		},
		{
			name:           "sum(ephemeralStorageAllocatable) > 19316009748 (error)",
			conditional:    "sum(ephemeralStorageAllocatable) > \"19316009748\"",
			matchingNodes:  nodeData,
			totalNodeCount: len(nodeData),
			expected:       false,
			isError:        true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := compareNodeResourceConditionalToActual(test.conditional, test.matchingNodes)
			if test.isError {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.expected, actual)

		})
	}
}

func Test_nodeMatchesFilters(t *testing.T) {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"label": "value",
			},
		},
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
		filters      *troubleshootv1beta2.NodeResourceFilters
		expectResult bool
	}{
		{
			name:         "true when empty filters",
			node:         node,
			filters:      &troubleshootv1beta2.NodeResourceFilters{},
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
			filters: &troubleshootv1beta2.NodeResourceFilters{
				MemoryAllocatable: "16Gi",
			},
			expectResult: false,
		},
		{
			name: "true when allocatable memory is available",
			node: node,
			filters: &troubleshootv1beta2.NodeResourceFilters{
				MemoryAllocatable: "4Gi",
			},
			expectResult: true,
		},
		{
			name: "false when the label does not exist",
			node: node,
			filters: &troubleshootv1beta2.NodeResourceFilters{
				Selector: &troubleshootv1beta2.NodeResourceSelectors{
					MatchLabel: map[string]string{
						"label2": "value",
					},
				},
			},
			expectResult: false,
		},
		{
			name: "false when the label value differs",
			node: node,
			filters: &troubleshootv1beta2.NodeResourceFilters{
				Selector: &troubleshootv1beta2.NodeResourceSelectors{
					MatchLabel: map[string]string{
						"label": "value2",
					},
				},
			},
			expectResult: false,
		},
		{
			name: "true when the label key and value match",
			node: node,
			filters: &troubleshootv1beta2.NodeResourceFilters{
				Selector: &troubleshootv1beta2.NodeResourceSelectors{
					MatchLabel: map[string]string{
						"label": "value",
					},
				},
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

func Test_analyzeNodeResources(t *testing.T) {
	tests := []struct {
		name     string
		analyzer *troubleshootv1beta2.NodeResources
		want     *AnalyzeResult
		wantErr  bool
	}{
		{
			name: "at least one pod per node capacity", // this is intended as a general "yes, the end-to-end test works"
			analyzer: &troubleshootv1beta2.NodeResources{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "min(podCapacity) < 1",
							Message: "There exist nodes with no pod capacity",
							URI:     "",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "min(podCapacity) < 50",
							Message: "There exist nodes with under 50 pod capacity",
							URI:     "",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "min(podCapacity) >= 50",
							Message: "All nodes can host at least 50 pods",
							URI:     "",
						},
					},
				},
				Filters: nil,
			},
			want: &AnalyzeResult{
				IsPass:  true,
				IsFail:  false,
				IsWarn:  false,
				Title:   "Node Resources",
				Message: "All nodes can host at least 50 pods",
				URI:     "",
				IconKey: "kubernetes_node_resources",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
			},
		},
		{
			name: "at least 16GB ram", // this is intended as a general "yes, the end-to-end fails properly"
			analyzer: &troubleshootv1beta2.NodeResources{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "min(memoryCapacity) < 16Gi",
							Message: "There exist nodes with under 16Gb of RAM",
							URI:     "",
						},
					},
				},
				Filters: nil,
			},
			want: &AnalyzeResult{
				IsPass:  false,
				IsFail:  true,
				IsWarn:  false,
				Title:   "Node Resources",
				Message: "There exist nodes with under 16Gb of RAM",
				URI:     "",
				IconKey: "kubernetes_node_resources",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
			},
		},
		{
			name: "at least 16GB ram in g-8vcpu-32gb nodes", // this is intended as a "does filtering work" test
			analyzer: &troubleshootv1beta2.NodeResources{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "min(memoryCapacity) < 16Gi",
							Message: "There exist nodes with under 16Gb of RAM",
							URI:     "",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "min(memoryCapacity) >= 16Gi",
							Message: "All nodes have at least 16Gb of RAM",
							URI:     "",
						},
					},
				},
				Filters: &troubleshootv1beta2.NodeResourceFilters{
					Selector: &troubleshootv1beta2.NodeResourceSelectors{
						MatchLabel: map[string]string{
							"node.kubernetes.io/instance-type": "g-8vcpu-32gb",
						},
					},
				},
			},
			want: &AnalyzeResult{
				IsPass:  true,
				IsFail:  false,
				IsWarn:  false,
				Title:   "Node Resources",
				Message: "All nodes have at least 16Gb of RAM",
				URI:     "",
				IconKey: "kubernetes_node_resources",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
			},
		},
		{
			name: "at least 4 cores in all nodes", // cpu count end-to-end
			analyzer: &troubleshootv1beta2.NodeResources{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "quadcore",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "min(cpuCapacity) < 4",
							Message: "There exist nodes with under 4 cores",
							URI:     "",
						},
					},
				},
			},
			want: &AnalyzeResult{
				IsPass:  false,
				IsFail:  true,
				IsWarn:  false,
				Title:   "quadcore",
				Message: "There exist nodes with under 4 cores",
				URI:     "",
				IconKey: "kubernetes_node_resources",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
			},
		},
		{
			name: "at least 4 cores in all g-8vcpu-32gb nodes", // cpu count end-to-end with filtering
			analyzer: &troubleshootv1beta2.NodeResources{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "min(cpuCapacity) < 4",
							Message: "There exist nodes with under 4 cores",
							URI:     "",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "min(cpuCapacity) >= 4",
							Message: "All nodes have at least 4 cores",
							URI:     "",
						},
					},
				},
				Filters: &troubleshootv1beta2.NodeResourceFilters{
					Selector: &troubleshootv1beta2.NodeResourceSelectors{
						MatchLabel: map[string]string{
							"node.kubernetes.io/instance-type": "g-8vcpu-32gb",
						},
					},
				},
			},
			want: &AnalyzeResult{
				IsPass:  true,
				IsFail:  false,
				IsWarn:  false,
				Title:   "Node Resources",
				Message: "All nodes have at least 4 cores",
				URI:     "",
				IconKey: "kubernetes_node_resources",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
			},
		},
		{
			name: "at least 8 cores in one node", // "max" e2e test
			analyzer: &troubleshootv1beta2.NodeResources{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "bignode-exists",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "max(cpuCapacity) < 8",
							Message: "There isn't a node with 8 or more cores",
							URI:     "",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "max(cpuCapacity) >= 8",
							Message: "There is a node with at least 8 cores",
							URI:     "",
						},
					},
				},
			},
			want: &AnalyzeResult{
				IsPass:  true,
				IsFail:  false,
				IsWarn:  false,
				Title:   "bignode-exists",
				Message: "There is a node with at least 8 cores",
				URI:     "",
				IconKey: "kubernetes_node_resources",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
			},
		},
		{
			name: "unfiltered CPU totals",
			analyzer: &troubleshootv1beta2.NodeResources{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "total-cpu",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "sum(cpuCapacity) < 6",
							Message: "there are less than 6 total cores",
							URI:     "",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "sum(cpuCapacity) > 6",
							Message: "there are more than 6 total cores",
							URI:     "",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "sum(cpuCapacity) = 6",
							Message: "There are exactly 6 total cores",
							URI:     "",
						},
					},
				},
			},
			want: &AnalyzeResult{
				IsPass:  false,
				IsFail:  false,
				IsWarn:  true,
				Title:   "total-cpu",
				Message: "there are more than 6 total cores",
				URI:     "",
				IconKey: "kubernetes_node_resources",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
			},
		},
		{
			name: "6 cores in s-2vcpu-4gb nodes",
			analyzer: &troubleshootv1beta2.NodeResources{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "s-2vcpu-4gb total",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "sum(cpuCapacity) < 6",
							Message: "there are less than 3 s-2vcpu-4gb nodes",
							URI:     "",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "sum(cpuCapacity) > 6",
							Message: "there are more than 3 s-2vcpu-4gb nodes",
							URI:     "",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "sum(cpuCapacity) = 6",
							Message: "There are exactly 3 s-2vcpu-4gb nodes",
							URI:     "",
						},
					},
				},
				Filters: &troubleshootv1beta2.NodeResourceFilters{
					Selector: &troubleshootv1beta2.NodeResourceSelectors{
						MatchLabel: map[string]string{
							"node.kubernetes.io/instance-type": "s-2vcpu-4gb",
						},
					},
				},
			},
			want: &AnalyzeResult{
				IsPass:  true,
				IsFail:  false,
				IsWarn:  false,
				Title:   "s-2vcpu-4gb total",
				Message: "There are exactly 3 s-2vcpu-4gb nodes",
				URI:     "",
				IconKey: "kubernetes_node_resources",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
			},
		},
		{
			name: "8 cores in nodes with at least 8gb of ram", // validate that filtering based on memory capacity works
			analyzer: &troubleshootv1beta2.NodeResources{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "memory filter",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "sum(cpuCapacity) < 8",
							Message: "less than 8 CPUs in nodes with 8Gb of ram",
							URI:     "",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "sum(cpuCapacity) = 8",
							Message: "exactly 8 CPUs total in nodes with 8Gb of ram",
							URI:     "",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "sum(cpuCapacity) > 8",
							Message: "more than 8 CPUs in nodes with 8Gb of ram",
							URI:     "",
						},
					},
				},
				Filters: &troubleshootv1beta2.NodeResourceFilters{
					MemoryCapacity: "8Gi",
				},
			},
			want: &AnalyzeResult{
				IsPass:  true,
				IsFail:  false,
				IsWarn:  false,
				Title:   "memory filter",
				Message: "more than 8 CPUs in nodes with 8Gb of ram",
				URI:     "",
				IconKey: "kubernetes_node_resources",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
			},
		},

		{
			name: "no pass or fail", // validate that the pass message is not always shown
			analyzer: &troubleshootv1beta2.NodeResources{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "no outcome",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "sum(cpuCapacity) = 8",
							Message: "exactly 8 CPUs total in nodes",
							URI:     "",
						},
					},
				},
			},
			want: &AnalyzeResult{
				IsPass:  false,
				IsFail:  false,
				IsWarn:  false,
				Title:   "no outcome",
				Message: "",
				URI:     "",
				IconKey: "kubernetes_node_resources",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/node-resources.svg?w=16&h=18",
			},
		},
	}

	getExampleNodeContents := func(nodeName string) ([]byte, error) {
		return []byte(collectedNodes), nil
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			got, err := analyzeNodeResources(tt.analyzer, getExampleNodeContents)
			req.NoError(err)
			req.Equal(tt.want, got)
		})
	}
}
