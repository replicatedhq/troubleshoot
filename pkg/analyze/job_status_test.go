package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_JobStatus(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     troubleshootv1beta2.JobStatus
		expectResult []*AnalyzeResult
		files        map[string][]byte
	}{
		{
			name: "1/1, pass when = 1",
			analyzer: troubleshootv1beta2.JobStatus{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "succeeded == 1",
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
				Namespace: "test",
				Name:      "pre-install-job",
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  true,
					IsWarn:  false,
					IsFail:  false,
					Title:   "pre-install-job Status",
					Message: "pass",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/jobs/test.json":           []byte(testJobs),
				"cluster-resources/jobs/projectcontour.json": []byte(projectcontourJobs),
			},
		},
		{
			name: "1/1, fail when < 2",
			analyzer: troubleshootv1beta2.JobStatus{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "succeeded < 2",
							Message: "fail",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "pass",
						},
					},
				},
				Namespace: "test",
				Name:      "pre-install-job",
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "pre-install-job Status",
					Message: "fail",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/jobs/test.json":           []byte(testJobs),
				"cluster-resources/jobs/projectcontour.json": []byte(projectcontourJobs),
			},
		},
		{
			name: "1/1, fail when failed > 0",
			analyzer: troubleshootv1beta2.JobStatus{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "succeeded = 1",
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "failed > 0",
							Message: "fail",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "default fail",
						},
					},
				},
				Namespace: "test",
				Name:      "post-install-job",
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "post-install-job Status",
					Message: "fail",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/jobs/test.json":           []byte(testJobs),
				"cluster-resources/jobs/projectcontour.json": []byte(projectcontourJobs),
			},
		},
		{
			name:     "analyze all jobs",
			analyzer: troubleshootv1beta2.JobStatus{},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "test/post-install-job Job Status",
					Message: "The job test/post-install-job is not complete",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "projectcontour/contour-certgen-v1.19.1 Job Status",
					Message: "The job projectcontour/contour-certgen-v1.19.1 is not complete",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/jobs/test.json":           []byte(testJobs),
				"cluster-resources/jobs/projectcontour.json": []byte(projectcontourJobs),
			},
		},
		{
			name: "analyze all jobs with namespaces",
			analyzer: troubleshootv1beta2.JobStatus{
				Namespaces: []string{"projectcontour"},
			},
			expectResult: []*AnalyzeResult{
				{
					IsPass:  false,
					IsWarn:  false,
					IsFail:  true,
					Title:   "projectcontour/contour-certgen-v1.19.1 Job Status",
					Message: "The job projectcontour/contour-certgen-v1.19.1 is not complete",
					IconKey: "kubernetes_deployment_status",
					IconURI: "https://troubleshoot.sh/images/analyzer-icons/deployment-status.svg?w=17&h=17",
				},
			},
			files: map[string][]byte{
				"cluster-resources/jobs/test.json":           []byte(testJobs),
				"cluster-resources/jobs/projectcontour.json": []byte(projectcontourJobs),
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

			actual, err := analyzeJobStatus(&test.analyzer, getFiles)
			req.NoError(err)

			req.Equal(len(test.expectResult), len(actual))
			for _, a := range actual {
				assert.Contains(t, test.expectResult, a)
			}
		})
	}
}
