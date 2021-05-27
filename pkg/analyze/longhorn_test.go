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

func TestAnalyzeLonghornReplica(t *testing.T) {
	tests := []struct {
		name    string
		replica *longhornv1beta1.Replica
		expect  *AnalyzeResult
	}{
		{
			name: "running",
			replica: &longhornv1beta1.Replica{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-uuid-1",
				},
				Spec: longhorntypes.ReplicaSpec{
					InstanceSpec: longhorntypes.InstanceSpec{
						DesireState: longhorntypes.InstanceStateRunning,
					},
				},
				Status: longhorntypes.ReplicaStatus{
					InstanceStatus: longhorntypes.InstanceStatus{
						CurrentState: longhorntypes.InstanceStateRunning,
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Longhorn Replica: pvc-uuid-1",
				IsPass:  true,
				Message: "Replica is running",
			},
		},
		{
			name: "stopped",
			replica: &longhornv1beta1.Replica{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-uuid-1",
				},
				Spec: longhorntypes.ReplicaSpec{
					InstanceSpec: longhorntypes.InstanceSpec{
						DesireState: longhorntypes.InstanceStateRunning,
					},
				},
				Status: longhorntypes.ReplicaStatus{
					InstanceStatus: longhorntypes.InstanceStatus{
						CurrentState: longhorntypes.InstanceStateStopped,
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Longhorn Replica: pvc-uuid-1",
				IsWarn:  true,
				Message: `Longhorn replica pvc-uuid-1 current status "stopped", should be "running"`,
			},
		},
		{
			name: "failed",
			replica: &longhornv1beta1.Replica{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-uuid-1",
				},
				Spec: longhorntypes.ReplicaSpec{
					FailedAt: "20210527T19:43:35",
				},
			},
			expect: &AnalyzeResult{
				Title:   "Longhorn Replica: pvc-uuid-1",
				IsWarn:  true,
				Message: "Longhorn replica pvc-uuid-1 failed at 20210527T19:43:35",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := analyzeLonghornReplica(test.replica)

			assert.Equal(t, test.expect, got)
		})
	}
}
