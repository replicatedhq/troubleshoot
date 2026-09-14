package preflight

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the contract that a vendor's `strict: true` on a v1beta3
// Preflight analyzer survives loading (v1beta3 -> v1beta2 rewrite) and is
// visible to strict enforcement, so downstream consumers (KOTS, Embedded
// Cluster) can rely on it.

const v1beta3StrictPreflight = `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: strict-preservation
spec:
  analyzers:
    - clusterVersion:
        checkName: Kubernetes version
        strict: true
        outcomes:
          - fail:
              when: "< 1.20.0"
              message: Requires at least Kubernetes 1.20.0
          - pass:
              message: Kubernetes version OK
`

func TestV1Beta3PreflightPreservesStrict(t *testing.T) {
	kinds, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{
		RawSpec: v1beta3StrictPreflight,
	})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)

	preflight := kinds.PreflightsV1Beta2[0]
	require.Len(t, preflight.Spec.Analyzers, 1)
	require.NotNil(t, preflight.Spec.Analyzers[0].ClusterVersion)
	assert.True(t, preflight.Spec.Analyzers[0].ClusterVersion.Strict.BoolOrDefaultFalse())

	hasStrict, err := HasStrictAnalyzers(&preflight)
	require.NoError(t, err)
	assert.True(t, hasStrict)
}

func TestV1Beta3TemplatedPreflightPreservesStrict(t *testing.T) {
	// Save and restore viper state
	oldValues := viper.Get("values")
	oldSet := viper.Get("set")
	defer func() {
		viper.Set("values", oldValues)
		viper.Set("set", oldSet)
	}()
	viper.Set("values", []string{})
	viper.Set("set", []string{"kubernetes.enabled=true"})

	templated := `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: strict-preservation-templated
spec:
  analyzers:
    {{- if .Values.kubernetes.enabled }}
    - clusterVersion:
        checkName: Kubernetes version
        strict: true
        outcomes:
          - fail:
              when: "< 1.20.0"
              message: Requires at least Kubernetes 1.20.0
          - pass:
              message: Kubernetes version OK
    {{- end }}
`

	specFile := filepath.Join(t.TempDir(), "strict-v1beta3.yaml")
	require.NoError(t, os.WriteFile(specFile, []byte(templated), 0644))

	processedArgs, tempFiles, err := preprocessV1Beta3Specs([]string{specFile})
	defer func() {
		for _, f := range tempFiles {
			_ = os.Remove(f)
		}
	}()
	require.NoError(t, err)
	require.Len(t, processedArgs, 1)

	rendered, err := os.ReadFile(processedArgs[0])
	require.NoError(t, err)

	kinds, err := loader.LoadSpecs(context.Background(), loader.LoadOptions{
		RawSpec: string(rendered),
	})
	require.NoError(t, err)
	require.Len(t, kinds.PreflightsV1Beta2, 1)

	preflight := kinds.PreflightsV1Beta2[0]
	require.Len(t, preflight.Spec.Analyzers, 1)
	require.NotNil(t, preflight.Spec.Analyzers[0].ClusterVersion)
	assert.True(t, preflight.Spec.Analyzers[0].ClusterVersion.Strict.BoolOrDefaultFalse())

	hasStrict, err := HasStrictAnalyzers(&preflight)
	require.NoError(t, err)
	assert.True(t, hasStrict)
}
