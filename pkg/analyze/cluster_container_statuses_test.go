package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/require"
)

func Test_analyzeContainerStatuses(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta2.ClusterContainerStatuses
		expectResult []*AnalyzeResult
		files        map[string][]byte
	}{
		{
			name: "fail when there is OOMKilled container",
			analyzer: troubleshootv1beta2.ClusterContainerStatuses{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "oomkilled-container",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "== OOMKilled",
							Message: "Container {{ .ContainerName }} from pod {{ .Namespace }}/{{ .PodName }} has OOMKilled",
						},
					},
				},
				Namespaces: []string{"message-oomkill-pod"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsFail:  true,
					IsWarn:  false,
					IsPass:  false,
					Title:   "oomkilled-container",
					Message: "Container memory-eater from pod message-oomkill-pod/oom-kill-job3-gbb89 has OOMKilled",
					IconKey: "kubernetes_container_statuses",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/kubernetes.svg?w=16&h=16",
				},
			},
			files: map[string][]byte{
				"cluster-resources/pods/message-oomkill-pod.json": []byte(messageOOMKillPod),
			},
		},
		{
			name: "pass when there is no status detected",
			analyzer: troubleshootv1beta2.ClusterContainerStatuses{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "oomkilled-container",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "== OOMKilled",
							Message: "Container {{ .ContainerName }} from pod {{ .Namespace }}/{{ .PodName }} has OOMKilled",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "No OOMKilled container found",
						},
					},
				},
				Namespaces: []string{"default"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsFail:  false,
					IsWarn:  false,
					IsPass:  true,
					Title:   "oomkilled-container",
					Message: "No OOMKilled container found",
					IconKey: "kubernetes_container_statuses",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/kubernetes.svg?w=16&h=16",
				},
			},
			files: map[string][]byte{
				"cluster-resources/pods/default.json": []byte(defaultPods),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			getFiles := func(n string, _ []string) (map[string][]byte, error) {
				return test.files, nil
			}

			a := AnalyzeClusterContainerStatuses{
				analyzer: &test.analyzer,
			}

			actual, err := a.Analyze(nil, getFiles)
			req.NoError(err)
			req.Equal(test.expectResult, actual)
		})
	}
}
