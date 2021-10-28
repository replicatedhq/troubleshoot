package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_analyzeReplicaSetStatus(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta2.ReplicaSetStatus
		expectResult []*AnalyzeResult
		files        map[string][]byte
	}{
		{
			name: "fail because 0 ready",
			analyzer: troubleshootv1beta2.ReplicaSetStatus{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ready == 1",
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				Namespace: "rook-ceph",
				Name:      "rook-ceph-mds-rook-shared-fs-b-7895f484f5",
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "rook-ceph-mds-rook-shared-fs-b-7895f484f5 Status",
					Message: "fail",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/replicasets/rook-ceph.json": []byte(collectedReplicaSets),
			},
		},
		{
			name: "fail because < 2 ready",
			analyzer: troubleshootv1beta2.ReplicaSetStatus{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "available < 2",
							Message: "fail",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
						},
					},
				},
				Namespace: "rook-ceph",
				Selector:  []string{"app=csi-cephfsplugin-provisioner"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "csi-cephfsplugin-provisioner-56d4db5b99 Status",
					Message: "fail",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/replicasets/rook-ceph.json": []byte(collectedReplicaSets),
			},
		},
		{
			name: "find the one failing replicaset",
			analyzer: troubleshootv1beta2.ReplicaSetStatus{
				Namespace: "rook-ceph",
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "rook-ceph/rook-ceph-mds-rook-shared-fs-b-7895f484f5 ReplicaSet Status",
					Message: "The replicaset rook-ceph/rook-ceph-mds-rook-shared-fs-b-7895f484f5 is not ready",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/replicasets/rook-ceph.json": []byte(collectedReplicaSets),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getFiles := func(n string) (map[string][]byte, error) {
				return test.files, nil
			}

			actual, err := analyzeReplicaSetStatus(&test.analyzer, getFiles)
			req.NoError(err)

			assert.Equal(t, test.expectResult, actual)

		})
	}
}
