package analyzer

import (
	"testing"

	longhornv1beta1 "github.com/longhorn/longhorn-manager/k8s/pkg/apis/longhorn/v1beta1"
	longhorntypes "github.com/longhorn/longhorn-manager/types"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAnalyzeLonghornNodeSchedulable(t *testing.T) {
	tests := []struct {
		name   string
		node   *longhornv1beta1.Node
		expect *AnalyzeResult
	}{
		{
			name: "schedulable",
			node: &longhornv1beta1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prod-1",
				},
				Status: longhorntypes.NodeStatus{
					Conditions: map[string]longhorntypes.Condition{
						longhorntypes.NodeConditionTypeSchedulable: longhorntypes.Condition{
							Status: longhorntypes.ConditionStatusTrue,
						},
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Longhorn Node Schedulable: prod-1",
				Message: "Longhorn node is schedulable",
				IsPass:  true,
			},
		},
		{
			name: "unschedulable",
			node: &longhornv1beta1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prod-1",
				},
				Status: longhorntypes.NodeStatus{
					Conditions: map[string]longhorntypes.Condition{
						longhorntypes.NodeConditionTypeSchedulable: longhorntypes.Condition{
							Status: longhorntypes.ConditionStatusFalse,
						},
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Longhorn Node Schedulable: prod-1",
				Message: "Longhorn node is not schedulable",
				IsWarn:  true,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := analyzeLonghornNodeSchedulable(test.node)

			assert.Equal(t, test.expect, got)
		})
	}
}
