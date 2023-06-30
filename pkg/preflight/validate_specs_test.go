package preflight

import (
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestValidatePreflight(t *testing.T) {
	testingFiles := map[string]string{
		"noCollectorsPreflightFile":                 "troubleshoot_v1beta2_preflight_validate_empty_collectors_gotest.yaml",
		"noAnalyzersPreflightFile":                  "troubleshoot_v1beta2_preflight_validate_empty_analyzers_gotest.yaml",
		"excludedAllDefaultCollectorsPreflightFile": "troubleshoot_v1beta2_preflight_validate_excluded_all_default_collectors_gotest.yaml",
		"excludedOneDefaultCollectorsPreflightFile": "troubleshoot_v1beta2_preflight_validate_excluded_one_default_collectors_gotest.yaml",
		"excludedAllNonCollectorsPreflightFile":     "troubleshoot_v1beta2_preflight_validate_excluded_all_non_default_collectors_gotest.yaml",
		"excludedAnalyzersPreflightFile":            "troubleshoot_v1beta2_preflight_validate_excluded_analyzers_gotest.yaml",
	}

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
			preflightSpec: testingFiles["noCollectorsPreflightFile"],
			wantWarning:   nil,
		},
		{
			name:          "no-analyzers",
			preflightSpec: testingFiles["noAnalyzersPreflightFile"],
			wantWarning:   types.NewExitCodeWarning("No analyzers found"),
		},
		{
			name:          "excluded-all-default-collectors",
			preflightSpec: testingFiles["excludedAllDefaultCollectorsPreflightFile"],
			wantWarning:   types.NewExitCodeWarning("All collectors were excluded by the applied values"),
		},
		{
			name:          "excluded-one-default-collectors",
			preflightSpec: testingFiles["excludedOneDefaultCollectorsPreflightFile"],
			wantWarning:   nil,
		},
		{
			name:          "excluded-all-non-default-collectors",
			preflightSpec: testingFiles["excludedAllNonCollectorsPreflightFile"],
			wantWarning:   nil,
		},
		{
			name:          "excluded-analyzers",
			preflightSpec: testingFiles["excludedAnalyzersPreflightFile"],
			wantWarning:   types.NewExitCodeWarning("All analyzers were excluded by the applied values"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFilePath := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/"+tt.preflightSpec)
			specs := PreflightSpecs{}
			specs.Read([]string{testFilePath})
			gotWarning := validatePreflight(specs)
			assert.Equal(t, tt.wantWarning, gotWarning)
		})
	}
}
