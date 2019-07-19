package collect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

func Test_selectorToString(t *testing.T) {
	tests := []struct {
		name         string
		selector []string
		expect string
	}{
		{
			name:        "app=api",
			selector: []string{"app=api"},
			expect: "app-api",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := selectorToString(test.selector)
			assert.Equal(t, test.expect, actual)
		})
	}
}

func Test_DeterministicIDForCollector(t *testing.T) {
	tests := []struct {
		name         string
		collector *troubleshootv1beta1.Collect
		expect string
	}{
		{
			name:        "cluster-info",
			collector: &troubleshootv1beta1.Collect{
				ClusterInfo: &troubleshootv1beta1.ClusterInfo{},
			},
			expect: "cluster-info",
		},
		{
			name:        "cluster-resources",
			collector: &troubleshootv1beta1.Collect{
				ClusterResources: &troubleshootv1beta1.ClusterResources{},
			},
			expect: "cluster-resources",
		},
		{
			name:        "secret",
			collector: &troubleshootv1beta1.Collect{
				Secret: &troubleshootv1beta1.Secret{
					Name: "secret-agent-woman",
					Namespace: "top-secret",
				},
			},
			expect: "secret-top-secret-secret-agent-woman",
		},
		{
			name:        "logs",
			collector: &troubleshootv1beta1.Collect{
				Logs: &troubleshootv1beta1.Logs{
					Namespace: "top-secret",
					Selector: []string{"this=is", "rather=long", "for=testing", "more=words", "too=many", "abcdef!=123456"},
				},
			},
			expect: "logs-top-secret-this-is-rather-long-for-testing-more-words-too-",
		},
	}


	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := DeterministicIDForCollector(test.collector)
			assert.Equal(t, test.expect, actual)
		})
	}
}
