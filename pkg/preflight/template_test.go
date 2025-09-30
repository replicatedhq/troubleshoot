package preflight

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// repoPath returns a path relative to the repository root from within pkg/preflight tests
func repoPath(rel string) string {
	if rel == "v1beta3.yaml" {
		// Use an existing v1beta3 example file for testing
		return filepath.Join("..", "..", "examples", "preflight", "simple-v1beta3.yaml")
	}
	return filepath.Join("..", "..", rel)
}

func TestDetectAPIVersion_V1Beta3(t *testing.T) {
	t.Parallel()
	content, err := os.ReadFile(repoPath("v1beta3.yaml"))
	require.NoError(t, err)
	api := detectAPIVersion(string(content))
	assert.Equal(t, "troubleshoot.sh/v1beta3", api)
}

func TestRender_V1Beta3_MinimalValues_YieldsNoAnalyzers(t *testing.T) {
	t.Parallel()
	tpl, err := os.ReadFile(repoPath("v1beta3.yaml"))
	require.NoError(t, err)

	valuesFile := repoPath("values-v1beta3-minimal.yaml")
	vals, err := loadValuesFile(valuesFile)
	require.NoError(t, err)

	rendered, err := RenderWithHelmTemplate(string(tpl), vals)
	require.NoError(t, err)

	kinds, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf := kinds.PreflightsV1Beta2[0]
	assert.Len(t, pf.Spec.Analyzers, 0)
}

func TestRender_V1Beta3_FullValues_ContainsExpectedAnalyzers(t *testing.T) {
	t.Parallel()
	tpl, err := os.ReadFile(repoPath("v1beta3.yaml"))
	require.NoError(t, err)

	valuesFile := repoPath("values-v1beta3-full.yaml")
	vals, err := loadValuesFile(valuesFile)
	require.NoError(t, err)

	rendered, err := RenderWithHelmTemplate(string(tpl), vals)
	require.NoError(t, err)

	kinds, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf := kinds.PreflightsV1Beta2[0]

	var hasStorageClass, hasCRD, hasRuntime, hasDistribution bool
	nodeResourcesCount := 0
	for _, a := range pf.Spec.Analyzers {
		if a.StorageClass != nil {
			hasStorageClass = true
			assert.Equal(t, "Default StorageClass", a.StorageClass.CheckName)
			assert.Equal(t, "default", a.StorageClass.StorageClassName)
		}
		if a.CustomResourceDefinition != nil {
			hasCRD = true
			assert.Equal(t, "Contour IngressRoute CRD", a.CustomResourceDefinition.CheckName)
			assert.Equal(t, "ingressroutes.contour.heptio.com", a.CustomResourceDefinition.CustomResourceDefinitionName)
		}
		if a.ContainerRuntime != nil {
			hasRuntime = true
		}
		if a.Distribution != nil {
			hasDistribution = true
		}
		if a.NodeResources != nil {
			nodeResourcesCount++
		}
	}

	assert.True(t, hasStorageClass, "expected StorageClass analyzer present")
	assert.True(t, hasCRD, "expected CustomResourceDefinition analyzer present")
	assert.True(t, hasRuntime, "expected ContainerRuntime analyzer present")
	assert.True(t, hasDistribution, "expected Distribution analyzer present")
	assert.Equal(t, 4, nodeResourcesCount, "expected 4 NodeResources analyzers (count, cpu, memory, ephemeral)")
}

func TestRender_V1Beta3_MergeMultipleValuesFiles_And_SetPrecedence(t *testing.T) {
	t.Parallel()
	tpl, err := os.ReadFile(repoPath("v1beta3.yaml"))
	require.NoError(t, err)

	// Merge minimal + 1 + 3 => kubernetes.enabled should end up false due to last wins in file 3
	vals := map[string]interface{}{}
	for _, f := range []string{
		repoPath("values-v1beta3-minimal.yaml"),
		repoPath("values-v1beta3-1.yaml"),
		repoPath("values-v1beta3-3.yaml"),
	} {
		m, err := loadValuesFile(f)
		require.NoError(t, err)
		vals = mergeMaps(vals, m)
	}

	// First render without --set; expect NO kubernetes analyzer
	rendered, err := RenderWithHelmTemplate(string(tpl), vals)
	require.NoError(t, err)
	kinds, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf := kinds.PreflightsV1Beta2[0]
	assert.False(t, containsAnalyzer(pf.Spec.Analyzers, "clusterVersion"))

	// Apply --set kubernetes.enabled=true and re-render; expect kubernetes analyzer present
	require.NoError(t, applySetValue(vals, "kubernetes.enabled=true"))
	rendered2, err := RenderWithHelmTemplate(string(tpl), vals)
	require.NoError(t, err)
	kinds2, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered2, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds2.PreflightsV1Beta2, 1)
	pf2 := kinds2.PreflightsV1Beta2[0]
	assert.True(t, containsAnalyzer(pf2.Spec.Analyzers, "clusterVersion"))
}

func containsAnalyzer(analyzers []*troubleshootv1beta2.Analyze, kind string) bool {
	for _, a := range analyzers {
		switch kind {
		case "clusterVersion":
			if a.ClusterVersion != nil {
				return true
			}
		case "storageClass":
			if a.StorageClass != nil {
				return true
			}
		case "customResourceDefinition":
			if a.CustomResourceDefinition != nil {
				return true
			}
		case "containerRuntime":
			if a.ContainerRuntime != nil {
				return true
			}
		case "distribution":
			if a.Distribution != nil {
				return true
			}
		case "nodeResources":
			if a.NodeResources != nil {
				return true
			}
		}
	}
	return false
}

func TestRender_V1Beta3_CLI_ValuesAndSetFlags(t *testing.T) {
	t.Parallel()
	tpl, err := os.ReadFile(repoPath("v1beta3.yaml"))
	require.NoError(t, err)

	// Start with minimal values (no analyzers enabled)
	vals, err := loadValuesFile(repoPath("values-v1beta3-minimal.yaml"))
	require.NoError(t, err)

	// Test: render with minimal values - should have no analyzers
	rendered, err := RenderWithHelmTemplate(string(tpl), vals)
	require.NoError(t, err)
	kinds, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf := kinds.PreflightsV1Beta2[0]
	assert.Len(t, pf.Spec.Analyzers, 0, "minimal values should produce no analyzers")

	// Test: simulate CLI --set flag to enable kubernetes checks
	err = applySetValue(vals, "kubernetes.enabled=true")
	require.NoError(t, err)
	rendered, err = RenderWithHelmTemplate(string(tpl), vals)
	require.NoError(t, err)
	kinds, err = loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf = kinds.PreflightsV1Beta2[0]
	assert.True(t, containsAnalyzer(pf.Spec.Analyzers, "clusterVersion"), "kubernetes analyzer should be present after --set kubernetes.enabled=true")

	// Test: simulate CLI --set flag to override specific values
	err = applySetValue(vals, "kubernetes.minVersion=1.25.0")
	require.NoError(t, err)
	err = applySetValue(vals, "kubernetes.recommendedVersion=1.27.0")
	require.NoError(t, err)
	rendered, err = RenderWithHelmTemplate(string(tpl), vals)
	require.NoError(t, err)
	kinds, err = loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf = kinds.PreflightsV1Beta2[0]

	// Verify the overridden values appear in the rendered spec
	var clusterVersionAnalyzer *troubleshootv1beta2.ClusterVersion
	for _, a := range pf.Spec.Analyzers {
		if a.ClusterVersion != nil {
			clusterVersionAnalyzer = a.ClusterVersion
			break
		}
	}
	require.NotNil(t, clusterVersionAnalyzer, "cluster version analyzer should be present")

	// Check that our --set values are used in the rendered outcomes
	foundMinVersion := false
	foundRecommendedVersion := false
	for _, outcome := range clusterVersionAnalyzer.Outcomes {
		if outcome.Fail != nil && strings.Contains(outcome.Fail.When, "1.25.0") {
			foundMinVersion = true
		}
		if outcome.Warn != nil && strings.Contains(outcome.Warn.When, "1.27.0") {
			foundRecommendedVersion = true
		}
	}
	assert.True(t, foundMinVersion, "should find --set minVersion in rendered spec")
	assert.True(t, foundRecommendedVersion, "should find --set recommendedVersion in rendered spec")

	// Test: multiple --set flags to enable multiple analyzer types
	err = applySetValue(vals, "storage.enabled=true")
	require.NoError(t, err)
	err = applySetValue(vals, "runtime.enabled=true")
	require.NoError(t, err)
	rendered, err = RenderWithHelmTemplate(string(tpl), vals)
	require.NoError(t, err)
	kinds, err = loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)
	pf = kinds.PreflightsV1Beta2[0]

	assert.True(t, containsAnalyzer(pf.Spec.Analyzers, "clusterVersion"), "kubernetes analyzer should remain enabled")
	assert.True(t, containsAnalyzer(pf.Spec.Analyzers, "storageClass"), "storage analyzer should be enabled")
	assert.True(t, containsAnalyzer(pf.Spec.Analyzers, "containerRuntime"), "runtime analyzer should be enabled")
}

func TestRender_V1Beta3_InvalidTemplate_ErrorHandling(t *testing.T) {
	t.Parallel()

	// Test: malformed YAML syntax (actually, this should pass template rendering but fail YAML parsing later)
	invalidYaml := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: invalid-yaml
spec:
  analyzers:
    - this is not valid yaml
      missing proper structure:
        - and wrong indentation
`
	vals := map[string]interface{}{}
	rendered, err := RenderWithHelmTemplate(invalidYaml, vals)
	require.NoError(t, err, "template rendering should succeed even with malformed YAML")

	// But loading the spec should fail due to invalid YAML structure
	_, err = loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	assert.Error(t, err, "loading malformed YAML should produce an error")

	// Test: invalid Helm template syntax
	invalidTemplate := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: invalid-template
spec:
  analyzers:
    {{- if .Values.invalid.syntax with unclosed brackets
    - clusterVersion:
        outcomes:
          - pass:
              message: "This should fail"
`
	_, err = RenderWithHelmTemplate(invalidTemplate, vals)
	assert.Error(t, err, "invalid template syntax should produce an error")

	// Test: template referencing undefined values with proper conditional check
	templateWithUndefined := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: undefined-values
spec:
  analyzers:
    {{- if and .Values.nonexistent (ne .Values.nonexistent.field nil) }}
    - clusterVersion:
        checkName: "Version: {{ .Values.nonexistent.version }}"
        outcomes:
          - pass:
              message: "Should not appear"
    {{- end }}
`
	rendered, err = RenderWithHelmTemplate(templateWithUndefined, vals)
	require.NoError(t, err, "properly guarded undefined values should not cause template error")
	kinds2, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{RawSpec: rendered, Strict: true})
	require.NoError(t, err)
	require.Len(t, kinds2.PreflightsV1Beta2, 1)
	pf2 := kinds2.PreflightsV1Beta2[0]
	assert.Len(t, pf2.Spec.Analyzers, 0, "undefined values should result in no analyzers")

	// Test: template that directly accesses undefined field (should error)
	templateWithDirectUndefined := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: direct-undefined
spec:
  analyzers:
    - clusterVersion:
        checkName: "{{ .Values.nonexistent.field }}"
        outcomes:
          - pass:
              message: "Should fail"
`
	_, err = RenderWithHelmTemplate(templateWithDirectUndefined, vals)
	assert.Error(t, err, "directly accessing undefined nested values should cause template error")

	// Test: template with missing required value (should error during template rendering)
	templateMissingRequired := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: missing-required
spec:
  analyzers:
    - storageClass:
        checkName: "Storage Test"
        storageClassName: {{ .Values.storage.className }}
        outcomes:
          - pass:
              message: "Storage is good"
`
	valsWithoutStorage := map[string]interface{}{
		"other": map[string]interface{}{
			"field": "value",
		},
	}
	_, err = RenderWithHelmTemplate(templateMissingRequired, valsWithoutStorage)
	assert.Error(t, err, "template rendering should fail when accessing undefined nested values")

	// Test: circular reference in values (this would be a user config error)
	circularVals := map[string]interface{}{
		"test": map[string]interface{}{
			"field": "{{ .Values.test.field }}", // This would create infinite loop if processed
		},
	}
	templateWithCircular := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: circular-test
spec:
  analyzers:
    - data:
        name: test.json
        data: |
          {"value": "{{ .Values.test.field }}"}
`
	// Helm template engine should handle this gracefully (it doesn't recursively process string values)
	rendered, err = RenderWithHelmTemplate(templateWithCircular, circularVals)
	require.NoError(t, err, "circular reference in values should not crash template engine")
	assert.Contains(t, rendered, "{{ .Values.test.field }}", "circular reference should render as literal string")
}
