package loader

import (
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestLoadingHelmTemplate_Succeeds(t *testing.T) {
	s := testutils.GetTestFixture(t, "yamldocs/helm-template.yaml")
	kinds, err := LoadFromStrings(s)
	assert.NoError(t, err)
	assert.NotNil(t, kinds)

	assert.Len(t, kinds.Analyzers, 0)
	assert.Len(t, kinds.Collectors, 0)
	assert.Len(t, kinds.HostCollectors, 0)
	assert.Len(t, kinds.HostPreflights, 0)
	assert.Len(t, kinds.Preflights, 2)
	assert.Len(t, kinds.Redactors, 1)
	assert.Len(t, kinds.RemoteCollectors, 0)
	assert.Len(t, kinds.SupportBundles, 2)

	// Assert a few fields from the loaded troubleshoot specs
	assert.Equal(t, "redactor-spec-1", kinds.Redactors[0].ObjectMeta.Name)
	assert.Equal(t, "REDACT SECOND TEXT PLEASE", kinds.Redactors[0].Spec.Redactors[0].Removals.Values[0])
	assert.Equal(t, "sb-spec-1", kinds.SupportBundles[0].ObjectMeta.Name)
	assert.Equal(t, "sb-spec-2", kinds.SupportBundles[1].ObjectMeta.Name)
	assert.Equal(t, "wg-easy", kinds.SupportBundles[1].Spec.Collectors[0].Logs.CollectorName)
	assert.Equal(t, "Node Count Check", kinds.Preflights[0].Spec.Analyzers[0].NodeResources.CheckName)
	assert.Len(t, kinds.Preflights[0].Spec.Collectors, 0)
	assert.Equal(t, true, kinds.Preflights[1].Spec.Collectors[0].ClusterResources.IgnoreRBAC)
}

func TestLoadingRandomValidYaml_IgnoreDoc(t *testing.T) {
	tests := []string{
		"",
		"---",
		"configVersion: v1",
		`
array:
  - 1
  - 2
`,
	}

	for _, ts := range tests {
		kinds, err := LoadFromStrings(ts)
		assert.NoError(t, err)
		assert.Equal(t, NewTroubleshootV1beta2Kinds(), kinds)
	}
}

func TestLoadingInvalidYaml_ReturnError(t *testing.T) {
	tests := []string{
		"@",
		"-",
		`
array:- 1
  - 2
`,
	}

	for _, ts := range tests {
		t.Run(ts, func(t *testing.T) {
			kinds, err := LoadFromStrings(ts)
			assert.Error(t, err)
			assert.Nil(t, kinds)
		})
	}
}

func TestKindsIsEmpty(t *testing.T) {
	assert.True(t, NewTroubleshootV1beta2Kinds().IsEmpty())
	kinds := NewTroubleshootV1beta2Kinds()
	kinds.Analyzers = append(kinds.Analyzers, troubleshootv1beta2.Analyzer{})
	assert.False(t, kinds.IsEmpty())
}
