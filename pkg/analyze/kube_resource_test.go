package analyzer

import (
	"path/filepath"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_findResource(t *testing.T) {
	tests := []struct {
		name           string
		isError        bool
		resourceExists bool
		analyzer       troubleshootv1beta2.ClusterResource
	}{
		{
			name:           "namespaced resource",
			resourceExists: true,
			analyzer: troubleshootv1beta2.ClusterResource{
				CollectorName: "Check namespaced resource",
				Kind:          "Deployment",
				Namespace:     "kube-system",
				Name:          "coredns",
			},
		},
		{
			name:           "check default fallthrough",
			resourceExists: true,
			analyzer: troubleshootv1beta2.ClusterResource{
				CollectorName: "Check namespaced resource",
				Kind:          "Deployment",
				Name:          "kotsadm-api",
			},
		},
		{
			name:           "cluster scoped resource",
			resourceExists: true,
			analyzer: troubleshootv1beta2.ClusterResource{
				CollectorName: "Check namespaced resource",
				Kind:          "Node",
				ClusterScoped: true,
				Name:          "repldev-marc",
			},
		},
		{
			name:           "resource does not exist",
			resourceExists: false,
			analyzer: troubleshootv1beta2.ClusterResource{
				CollectorName: "Check namespaced resource",
				Kind:          "Node",
				ClusterScoped: true,
				Name:          "resource-does-not-exist",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			rootDir := filepath.Join("files", "support-bundle")
			fcp := fileContentProvider{rootDir: rootDir}

			analyzer := &test.analyzer
			item, err := FindResource(analyzer.Kind, analyzer.ClusterScoped, analyzer.Namespace, analyzer.Name, fcp.getFileContents)
			assert.Equal(t, test.resourceExists, item != nil)
			assert.Nil(t, err)
		})
	}
}

func Test_analyzeResource(t *testing.T) {
	tests := []struct {
		name         string
		isError      bool
		analyzer     troubleshootv1beta2.ClusterResource
		expectResult AnalyzeResult
	}{
		{
			name: "pass-when-pvc-exists-and-is-right-access-mode",
			analyzer: troubleshootv1beta2.ClusterResource{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "check-pvc-is-rwx",
				},
				Kind:          "PersistentVolumeClaim",
				Name:          "redis-data-redis-replicas-0",
				Namespace:     "default",
				YamlPath:      "spec.accessModes.[0]",
				ExpectedValue: "ReadWriteMany",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "false",
							Message: "fail",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "true",
							Message: "pass",
						},
					},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "check-pvc-is-rwx",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg?w=13&h=16",
			},
		},
		{
			name: "fail-when-pvc-exists-but-is-wrong-access-mode",
			analyzer: troubleshootv1beta2.ClusterResource{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "check-pvc-is-rwo",
				},
				Kind:          "PersistentVolumeClaim",
				Name:          "redis-data-redis-replicas-0",
				Namespace:     "default",
				YamlPath:      "spec.accessModes.[0]",
				ExpectedValue: "ReadWriteOnce",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "false",
							Message: "fail",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "true",
							Message: "pass",
						},
					},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "check-pvc-is-rwo",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg?w=13&h=16",
			},
		},
		{
			name: "fail-when-pvc-doesnt-exist",
			analyzer: troubleshootv1beta2.ClusterResource{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "check-pvc-exists",
				},
				Kind:          "PersistentVolumeClaim",
				Name:          "data-postgresql-00",
				Namespace:     "default",
				YamlPath:      "metadata.name",
				ExpectedValue: "data-postgresql-00",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "false",
							Message: "fail",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "true",
							Message: "pass",
						},
					},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "check-pvc-exists",
				Message: "YAML path provided is invalid",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
		},
		{
			name: "pass-when-pvc-exists-and-is-right-access-mode-regex",
			analyzer: troubleshootv1beta2.ClusterResource{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "check-pvc-is-rwx",
				},
				Kind:         "PersistentVolumeClaim",
				Name:         "redis-data-redis-replicas-0",
				Namespace:    "default",
				YamlPath:     "spec.accessModes",
				RegexPattern: "ReadWriteMany",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "false",
							Message: "fail",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "true",
							Message: "pass",
						},
					},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "check-pvc-is-rwx",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg",
			},
		},
		{
			name: "pass-when-pvc-exists-and-is-at-least-8Gi-regexGroup",
			analyzer: troubleshootv1beta2.ClusterResource{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "check-pvc-is-at-least-8Gi",
				},
				Kind:        "PersistentVolumeClaim",
				Name:        "data-postgresql-0",
				Namespace:   "default",
				YamlPath:    "spec.resources.requests.storage",
				RegexGroups: "(?P<PVCSize>\\d+)Gi",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "PVCSize >= 8",
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "check-pvc-is-at-least-8Gi",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg?w=13&h=16",
			},
		},
		{
			name: "pass-when-pvc-exists-and-is-at-least-4Gi",
			analyzer: troubleshootv1beta2.ClusterResource{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "check-pvc-is-at-least-4Gi",
				},
				Kind:      "PersistentVolumeClaim",
				Name:      "data-postgresql-0",
				Namespace: "default",
				YamlPath:  "spec.resources.requests.storage",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    ">= 4Gi",
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "check-pvc-is-at-least-4Gi",
				Message: "pass",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg?w=13&h=16",
			},
		},
		{
			name: "fail-when-pvc-exists-and-is-not-at-least-16Gi",
			analyzer: troubleshootv1beta2.ClusterResource{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "check-pvc-is-at-least-16Gi",
				},
				Kind:      "PersistentVolumeClaim",
				Name:      "data-postgresql-0",
				Namespace: "default",
				YamlPath:  "spec.resources.requests.storage",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    ">= 16Gi",
							Message: "pass",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "fail",
						},
					},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "check-pvc-is-at-least-16Gi",
				Message: "fail",
				IconKey: "kubernetes_text_analyze",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/text-analyze.svg?w=13&h=16",
			},
		},
	}
	{
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				req := require.New(t)

				rootDir := filepath.Join("files", "support-bundle")
				fcp := fileContentProvider{rootDir: rootDir}

				a := AnalyzeClusterResource{
					analyzer: &test.analyzer,
				}

				actual, err := a.analyzeResource(&test.analyzer, fcp.getFileContents)
				if !test.isError {
					req.NoError(err)
					req.Equal(test.expectResult, *actual)
				} else {
					req.Error(err)
				}

			})
		}
	}
}
