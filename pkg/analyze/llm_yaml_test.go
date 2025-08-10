package analyzer

import (
	"fmt"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// TestAnalyzeLLM_YAMLParsing tests parsing of LLM analyzer from YAML
func TestAnalyzeLLM_YAMLParsing(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		validate func(t *testing.T, analyzer *troubleshootv1beta2.Analyze)
	}{
		{
			name: "Full LLM spec",
			yaml: `
llm:
  checkName: "Test LLM Analysis"
  collectorName: "test-logs"
  fileName: "*.log"
  maxFiles: 5
  model: "gpt-4"
  outcomes:
    - fail:
        when: "issue_found"
        message: "Critical: {{.Summary}}"
    - pass:
        message: "No issues"`,
			validate: func(t *testing.T, analyzer *troubleshootv1beta2.Analyze) {
				require.NotNil(t, analyzer.LLM)
				assert.Equal(t, "Test LLM Analysis", analyzer.LLM.CheckName)
				assert.Equal(t, "test-logs", analyzer.LLM.CollectorName)
				assert.Equal(t, "*.log", analyzer.LLM.FileName)
				assert.Equal(t, 5, analyzer.LLM.MaxFiles)
				assert.Equal(t, "gpt-4", analyzer.LLM.Model)
				assert.Len(t, analyzer.LLM.Outcomes, 2)
			},
		},
		{
			name: "Minimal LLM spec",
			yaml: `
llm:
  collectorName: "logs"`,
			validate: func(t *testing.T, analyzer *troubleshootv1beta2.Analyze) {
				require.NotNil(t, analyzer.LLM)
				assert.Equal(t, "logs", analyzer.LLM.CollectorName)
				assert.Equal(t, "", analyzer.LLM.CheckName)
				assert.Equal(t, "", analyzer.LLM.Model)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var analyzer troubleshootv1beta2.Analyze
			err := yaml.Unmarshal([]byte(tt.yaml), &analyzer)
			require.NoError(t, err)
			tt.validate(t, &analyzer)
		})
	}
}

// TestAnalyzeLLM_CompleteSpec tests a complete support bundle spec with LLM
func TestAnalyzeLLM_CompleteSpec(t *testing.T) {
	specYAML := `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: test-bundle
spec:
  collectors:
    - clusterInfo: {}
    - logs:
        name: app-logs
        namespace: default
        selector:
          - app=myapp
  analyzers:
    - llm:
        checkName: "Application Health Check"
        collectorName: "app-logs"
        fileName: "*"
        model: "gpt-4o-mini"
        maxFiles: 10
        outcomes:
          - fail:
              when: "issue_found"
              message: "Problem detected: {{.Summary}}"
          - warn:
              when: "potential_issue"
              message: "Warning: {{.Summary}}"
          - pass:
              message: "Application is healthy"
    - clusterVersion:
        outcomes:
          - pass:
              when: ">= 1.20.0"
              message: "Kubernetes version is supported"`

	var bundle troubleshootv1beta2.SupportBundle
	err := yaml.Unmarshal([]byte(specYAML), &bundle)
	require.NoError(t, err)

	// Verify the bundle was parsed correctly
	assert.Equal(t, "test-bundle", bundle.Name)
	assert.Len(t, bundle.Spec.Collectors, 2)
	assert.Len(t, bundle.Spec.Analyzers, 2)

	// Check LLM analyzer
	var foundLLM bool
	for _, analyzer := range bundle.Spec.Analyzers {
		if analyzer.LLM != nil {
			foundLLM = true
			assert.Equal(t, "Application Health Check", analyzer.LLM.CheckName)
			assert.Equal(t, "app-logs", analyzer.LLM.CollectorName)
			assert.Equal(t, "gpt-4o-mini", analyzer.LLM.Model)
			assert.Equal(t, 10, analyzer.LLM.MaxFiles)
			assert.Len(t, analyzer.LLM.Outcomes, 3)
		}
	}
	assert.True(t, foundLLM, "LLM analyzer not found in spec")
}

// TestAnalyzeLLM_InvalidSpecs tests invalid YAML configurations
func TestAnalyzeLLM_InvalidSpecs(t *testing.T) {
	invalidSpecs := []string{
		// Invalid outcome when condition
		`
llm:
  outcomes:
    - fail:
        when: "invalid_condition"
        message: "Test"`,
		
		// Invalid model (this would be validated at runtime)
		`
llm:
  model: "not-a-real-model"`,
	}

	for i, spec := range invalidSpecs {
		t.Run(fmt.Sprintf("InvalidSpec_%d", i), func(t *testing.T) {
			var analyzer troubleshootv1beta2.Analyze
			// These should parse but might fail at runtime
			err := yaml.Unmarshal([]byte(spec), &analyzer)
			// YAML parsing should succeed even for semantically invalid specs
			assert.NoError(t, err)
		})
	}
}