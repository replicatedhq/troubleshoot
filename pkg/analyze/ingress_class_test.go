package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAnalyzeIngressClass(t *testing.T) {
	tests := []struct {
		name         string
		analyzer     *troubleshootv1beta2.IngressClass
		ingressList  *networkingv1.IngressClassList
		expectResult AnalyzeResult
	}{
		{
			name: "named ingress class found",
			analyzer: &troubleshootv1beta2.IngressClass{
				IngressClassName: "nginx",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "nginx ingress class found",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "nginx ingress class not found",
						},
					},
				},
			},
			ingressList: &networkingv1.IngressClassList{
				Items: []networkingv1.IngressClass{
					{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				Title:   "Ingress class nginx",
				Message: "nginx ingress class found",
				IconKey: "kubernetes_ingress_class",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/ingress-class.svg?w=12&h=12",
			},
		},
		{
			name: "named ingress class not found",
			analyzer: &troubleshootv1beta2.IngressClass{
				IngressClassName: "nginx",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "nginx ingress class found",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "nginx ingress class not found",
						},
					},
				},
			},
			ingressList: &networkingv1.IngressClassList{
				Items: []networkingv1.IngressClass{
					{ObjectMeta: metav1.ObjectMeta{Name: "traefik"}},
				},
			},
			expectResult: AnalyzeResult{
				IsFail:  true,
				Title:   "Ingress class nginx",
				Message: "nginx ingress class not found",
				IconKey: "kubernetes_ingress_class",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/ingress-class.svg?w=12&h=12",
			},
		},
		{
			name: "default ingress class found",
			analyzer: &troubleshootv1beta2.IngressClass{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "default ingress class exists",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "no default ingress class",
						},
					},
				},
			},
			ingressList: &networkingv1.IngressClassList{
				Items: []networkingv1.IngressClass{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "nginx",
							Annotations: map[string]string{
								"ingressclass.kubernetes.io/is-default-class": "true",
							},
						},
					},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				Title:   "Default Ingress Class",
				Message: "default ingress class exists",
				IconKey: "kubernetes_ingress_class",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/ingress-class.svg?w=12&h=12",
			},
		},
		{
			name: "default ingress class not found",
			analyzer: &troubleshootv1beta2.IngressClass{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "default ingress class exists",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "no default ingress class",
						},
					},
				},
			},
			ingressList: &networkingv1.IngressClassList{
				Items: []networkingv1.IngressClass{
					{ObjectMeta: metav1.ObjectMeta{Name: "nginx"}},
				},
			},
			expectResult: AnalyzeResult{
				IsFail:  true,
				Title:   "Default Ingress Class",
				Message: "no default ingress class",
				IconKey: "kubernetes_ingress_class",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/ingress-class.svg?w=12&h=12",
			},
		},
		{
			name: "default ingress class not found with default message",
			analyzer: &troubleshootv1beta2.IngressClass{
				Outcomes: []*troubleshootv1beta2.Outcome{},
			},
			ingressList: &networkingv1.IngressClassList{},
			expectResult: AnalyzeResult{
				IsFail:  true,
				Title:   "Default Ingress Class",
				Message: "No Default Ingress Class found",
				IconKey: "kubernetes_ingress_class",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/ingress-class.svg?w=12&h=12",
			},
		},
		{
			name: "default ingress class found with default message",
			analyzer: &troubleshootv1beta2.IngressClass{
				Outcomes: []*troubleshootv1beta2.Outcome{},
			},
			ingressList: &networkingv1.IngressClassList{
				Items: []networkingv1.IngressClass{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: "nginx",
							Annotations: map[string]string{
								"ingressclass.kubernetes.io/is-default-class": "true",
							},
						},
					},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				Title:   "Default Ingress Class",
				Message: "Default Ingress Class found",
				IconKey: "kubernetes_ingress_class",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/ingress-class.svg?w=12&h=12",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.ingressList)
			require.NoError(t, err)

			getFile := func(_ string) ([]byte, error) {
				return b, nil
			}

			a := AnalyzeIngressClass{analyzer: tt.analyzer}
			result, err := a.analyzeIngressClass(tt.analyzer, getFile)
			require.NoError(t, err)
			assert.Equal(t, tt.expectResult, *result)
		})
	}
}
