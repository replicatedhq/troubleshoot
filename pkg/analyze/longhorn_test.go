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

func TestAnalyzeLonghornEngine(t *testing.T) {
	tests := []struct {
		name   string
		engine *longhornv1beta1.Engine
		expect *AnalyzeResult
	}{
		{
			name: "running",
			engine: &longhornv1beta1.Engine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-uuid-1",
				},
				Spec: longhorntypes.EngineSpec{
					InstanceSpec: longhorntypes.InstanceSpec{
						DesireState: longhorntypes.InstanceStateRunning,
					},
				},
				Status: longhorntypes.EngineStatus{
					InstanceStatus: longhorntypes.InstanceStatus{
						CurrentState: longhorntypes.InstanceStateRunning,
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Longhorn Engine: pvc-uuid-1",
				IsPass:  true,
				Message: "Engine is running",
			},
		},
		{
			name: "stopped",
			engine: &longhornv1beta1.Engine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-uuid-1",
				},
				Spec: longhorntypes.EngineSpec{
					InstanceSpec: longhorntypes.InstanceSpec{
						DesireState: longhorntypes.InstanceStateRunning,
					},
				},
				Status: longhorntypes.EngineStatus{
					InstanceStatus: longhorntypes.InstanceStatus{
						CurrentState: longhorntypes.InstanceStateStopped,
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Longhorn Engine: pvc-uuid-1",
				IsWarn:  true,
				Message: `Longhorn engine pvc-uuid-1 current status "stopped", should be "running"`,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := analyzeLonghornEngine(test.engine)

			assert.Equal(t, test.expect, got)
		})
	}
}

func TestAnalyzeLonghornReplicaChecksums(t *testing.T) {
	tests := []struct {
		name       string
		checksums  []map[string]string
		volumeName string
		expect     *AnalyzeResult
	}{
		{
			name: "3 ok",
			checksums: []map[string]string{
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
				{
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
				{
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
				},
			},
			volumeName: "pvc-uuid-123",
			expect: &AnalyzeResult{
				Title:   "Longhorn Volume Replica Corruption: pvc-uuid-123",
				IsPass:  true,
				Message: "No replica corruption detected",
			},
		},
		{
			name: "2 ok",
			checksums: []map[string]string{
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
				{
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
				},
			},
			volumeName: "pvc-uuid-123",
			expect: &AnalyzeResult{
				Title:   "Longhorn Volume Replica Corruption: pvc-uuid-123",
				IsPass:  true,
				Message: "No replica corruption detected",
			},
		},
		{
			name: "1 of 3 corrupt",
			checksums: []map[string]string{
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
			},
			volumeName: "pvc-uuid-123",
			expect: &AnalyzeResult{
				Title:   "Longhorn Volume Replica Corruption: pvc-uuid-123",
				IsWarn:  true,
				Message: "Replica corruption detected",
			},
		},
		{
			name: "2 of 3 corrupt",
			checksums: []map[string]string{
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
			},
			volumeName: "pvc-uuid-123",
			expect: &AnalyzeResult{
				Title:   "Longhorn Volume Replica Corruption: pvc-uuid-123",
				IsWarn:  true,
				Message: "Replica corruption detected",
			},
		},
		{
			name: "3 of 3 corrupt",
			checksums: []map[string]string{
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
			},
			volumeName: "pvc-uuid-123",
			expect: &AnalyzeResult{
				Title:   "Longhorn Volume Replica Corruption: pvc-uuid-123",
				IsWarn:  true,
				Message: "Replica corruption detected",
			},
		},
		{
			name: "1 of 2 corrupt",
			checksums: []map[string]string{
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
				{
					"revision.counter":         "7cc93e21d84bb7d0db0a72281f21500ba3847dea6467631cca91523d01ace8c9",
					"volume-head-000.img":      "7637cb563f796f8d6358ff4fc635ce596e5326a7f940cc2ea2eaee0acff843ce",
					"volume-head-000.img.meta": "ca21027be32ef389de0b21d0c4713e824cad7114a287e05e56de49c948492fc9",
					"volume.meta":              "e9ce811b3f11dfe3af0bdd46581f23ba2c570be5dc3b807652ad6142322c706b",
				},
			},
			volumeName: "pvc-uuid-123",
			expect: &AnalyzeResult{
				Title:   "Longhorn Volume Replica Corruption: pvc-uuid-123",
				IsWarn:  true,
				Message: "Replica corruption detected",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := analyzeLonghornReplicaChecksums(test.volumeName, test.checksums)
			assert.Equal(t, test.expect, got)
		})
	}
}

func TestSimplifyLonghornResults(t *testing.T) {
	tests := []struct {
		name   string
		input  []*AnalyzeResult
		expect []*AnalyzeResult
	}{
		{
			name: "All pass",
			input: []*AnalyzeResult{
				{
					Title:   "Replica 1",
					IsPass:  true,
					Message: "Replica 1 ok",
				},
				{
					Title:   "Node 1",
					IsPass:  true,
					Message: "Node 1 ok",
				},
			},
			expect: []*AnalyzeResult{
				{
					Title:   "Longhorn Health Status",
					IsPass:  true,
					Message: "Longhorn is healthy",
				},
			},
		},
		{
			name:   "No Results",
			input:  []*AnalyzeResult{},
			expect: []*AnalyzeResult{},
		},
		{
			name: "Mixed results",
			input: []*AnalyzeResult{
				{
					Title:   "Replica 1",
					IsPass:  true,
					Message: "Replica 1 ok",
				},
				{
					Title:   "Replica 2",
					IsWarn:  true,
					Message: "Replica 1 is down",
				},
				{
					Title:   "Node 1",
					IsPass:  true,
					Message: "Node 1 ok",
				},
				{
					Title:   "Node 2",
					IsFail:  true,
					Message: "Node 2 is down",
				},
			},
			expect: []*AnalyzeResult{
				{
					Title:   "Replica 2",
					IsWarn:  true,
					Message: "Replica 1 is down",
				},
				{
					Title:   "Node 2",
					IsFail:  true,
					Message: "Node 2 is down",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := simplifyLonghornResults(test.input)

			assert.Equal(t, test.expect, got)
		})
	}
}
