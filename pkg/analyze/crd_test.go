package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAnalyzeCustomResourceDefinition(t *testing.T) {
	getFile := func(_ string) ([]byte, error) {
		crdsList := v1beta1.CustomResourceDefinitionList{
			Items: []v1beta1.CustomResourceDefinition{
				{ObjectMeta: metav1.ObjectMeta{Name: "servicemonitors.monitoring.coreos.com"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "probes.monitoring.coreos.com"}},
			},
		}
		return json.Marshal(crdsList)
	}

	tests := []struct {
		name           string
		analyzer       *troubleshootv1beta2.CustomResourceDefinition
		expectedResult *AnalyzeResult
	}{
		{
			name: "CRD exists and pass",
			analyzer: &troubleshootv1beta2.CustomResourceDefinition{
				CustomResourceDefinitionName: "servicemonitors.monitoring.coreos.com",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "The ServiceMonitor CRD is installed and available.",
						},
					},
				},
			},
			expectedResult: &AnalyzeResult{
				Title:   "Custom resource definition servicemonitors.monitoring.coreos.com",
				IconKey: "kubernetes_custom_resource_definition",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/custom-resource-definition.svg?w=13&h=16",
				IsPass:  true,
				Message: "The ServiceMonitor CRD is installed and available.",
			},
		},
		{
			name: "CRD not exists and warn",
			analyzer: &troubleshootv1beta2.CustomResourceDefinition{
				CustomResourceDefinitionName: "podmonitors.monitoring.coreos.com",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							Message: "The Prometheus PodMonitor CRD was not found in the cluster.",
						},
					},
				},
			},
			expectedResult: &AnalyzeResult{
				Title:   "Custom resource definition podmonitors.monitoring.coreos.com",
				IconKey: "kubernetes_custom_resource_definition",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/custom-resource-definition.svg?w=13&h=16",
				IsWarn:  true,
				Message: "The Prometheus PodMonitor CRD was not found in the cluster.",
			},
		},
		{
			name: "CRD not exists and fail",
			analyzer: &troubleshootv1beta2.CustomResourceDefinition{
				CustomResourceDefinitionName: "backupstoragelocations.velero.io",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "The BackupStorageLocation CRD was not found in the cluster.",
						},
					},
				},
			},
			expectedResult: &AnalyzeResult{
				Title:   "Custom resource definition backupstoragelocations.velero.io",
				IconKey: "kubernetes_custom_resource_definition",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/custom-resource-definition.svg?w=13&h=16",
				IsFail:  true,
				Message: "The BackupStorageLocation CRD was not found in the cluster.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AnalyzeCustomResourceDefinition{
				analyzer: tt.analyzer,
			}
			result, err := a.analyzeCustomResourceDefinition(tt.analyzer, getFile)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
