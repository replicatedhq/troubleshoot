package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_analyzeStatefulsetStatus(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta2.StatefulsetStatus
		expectResult []*AnalyzeResult
		files        map[string][]byte
	}{
		{
			name: "fail when absent",
			analyzer: troubleshootv1beta2.StatefulsetStatus{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "absent",
							Message: "fail",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "= 1",
							Message: "pass",
						},
					},
				},
				Namespace: "default",
				Name:      "nonexistant",
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "nonexistant Status",
					Message: "fail",
					IconKey: "kubernetes_statefulset_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/statefulset-status.svg?w=23&h=14",
				},
			},
			files: map[string][]byte{
				"cluster-resources/statefulsets/default.json": []byte(defaultStatefulSets),
			},
		},
		{
			name:     "analyze all statefulsets",
			analyzer: troubleshootv1beta2.StatefulsetStatus{},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "monitoring/alertmanager-prometheus-alertmanager Statefulset Status",
					Message: "The statefulset monitoring/alertmanager-prometheus-alertmanager has 1/2 replicas",
					IconKey: "kubernetes_statefulset_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/statefulset-status.svg?w=23&h=14",
				},
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "default/kotsadm-postgres Statefulset Status",
					Message: "The statefulset default/kotsadm-postgres has 1/2 replicas",
					IconKey: "kubernetes_statefulset_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/statefulset-status.svg?w=23&h=14",
				},
			},
			files: map[string][]byte{
				"cluster-resources/statefulsets/monitoring.json": []byte(monitoringStatefulSets),
				"cluster-resources/statefulsets/default.json":    []byte(defaultStatefulSets),
			},
		},
		{
			name: "analyze all statefulsets with namespaces",
			analyzer: troubleshootv1beta2.StatefulsetStatus{
				Namespaces: []string{"monitoring"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "monitoring/alertmanager-prometheus-alertmanager Statefulset Status",
					Message: "The statefulset monitoring/alertmanager-prometheus-alertmanager has 1/2 replicas",
					IconKey: "kubernetes_statefulset_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/statefulset-status.svg?w=23&h=14",
				},
			},
			files: map[string][]byte{
				"cluster-resources/statefulsets/monitoring.json": []byte(monitoringStatefulSets),
				"cluster-resources/statefulsets/default.json":    []byte(defaultStatefulSets),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getFiles := func(n string, _ []string) (map[string][]byte, error) {
				if file, ok := test.files[n]; ok {
					return map[string][]byte{n: file}, nil
				}
				return test.files, nil
			}

			actual, err := analyzeStatefulsetStatus(&test.analyzer, getFiles)
			req.NoError(err)

			req.Equal(len(test.expectResult), len(actual))
			for _, a := range actual {
				assert.Contains(t, test.expectResult, a)
			}
		})
	}
}
