package preflight

import (
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestValidatePreflight(t *testing.T) {
	noCollectorsPreflightFile := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_validate_empty_collectors_gotest.yaml")
	noAnalyzersPreflightFile := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_validate_empty_analyzers_gotest.yaml")
	excludedCollectorsPreflightFile := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_validate_excluded_collectors_gotest.yaml")
	excludedAnalyzersPreflightFile := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_validate_excluded_analyzers_gotest.yaml")
	tests := []struct {
		name          string
		preflightSpec string
		wantWarning   *types.ExitCodeWarning
	}{
		{
			name:          "empty-preflight",
			preflightSpec: "",
			wantWarning:   types.NewExitCodeWarning("no preflight or host preflight spec was found"),
		},
		{
			name:          "no-collectores",
			preflightSpec: noCollectorsPreflightFile,
			wantWarning:   types.NewExitCodeWarning("No collectors found"),
		},
		{
			name:          "no-analyzers",
			preflightSpec: noAnalyzersPreflightFile,
			wantWarning:   types.NewExitCodeWarning("No analyzers found"),
		},
		{
			name:          "excluded-collectors",
			preflightSpec: excludedCollectorsPreflightFile,
			wantWarning:   types.NewExitCodeWarning("All collectors were excluded by the applied values"),
		},
		{
			name:          "excluded-analyzers",
			preflightSpec: excludedAnalyzersPreflightFile,
			wantWarning:   types.NewExitCodeWarning("All analyzers were excluded by the applied values"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := PreflightSpecs{}
			specs.Read([]string{tt.preflightSpec})
			gotWarning := validatePreflight(specs)
			assert.Equal(t, tt.wantWarning.Warning(), gotWarning.Warning())
		})
	}
}
