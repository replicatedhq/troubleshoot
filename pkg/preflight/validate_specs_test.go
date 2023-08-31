package preflight

import (
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidatePreflight(t *testing.T) {
	testingFiles := map[string]string{
		"noCollectorsPreflightFile":                 "troubleshoot_v1beta2_preflight_validate_empty_collectors_gotest.yaml",
		"noAnalyzersPreflightFile":                  "troubleshoot_v1beta2_preflight_validate_empty_analyzers_gotest.yaml",
		"excludedAllDefaultCollectorsPreflightFile": "troubleshoot_v1beta2_preflight_validate_excluded_all_default_collectors_gotest.yaml",
		"excludedOneDefaultCollectorsPreflightFile": "troubleshoot_v1beta2_preflight_validate_excluded_one_default_collectors_gotest.yaml",
		"excludedAllNonCollectorsPreflightFile":     "troubleshoot_v1beta2_preflight_validate_excluded_all_non_default_collectors_gotest.yaml",
		"excludedAnalyzersPreflightFile":            "troubleshoot_v1beta2_preflight_validate_excluded_analyzers_gotest.yaml",
		"noCollectorsHostPreflightFile":             "troubleshoot_v1beta2_host_preflight_validate_empty_collectors_gotest.yaml",
		"noAnalyzersHostPreflightFile":              "troubleshoot_v1beta2_host_preflight_validate_empty_analyzers_gotest.yaml",
		"excludedHostCollectorsPreflightFile":       "troubleshoot_v1beta2_host_preflight_validate_excluded_collectors_gotest.yaml",
		"excludedHostAnalyzersPreflightFile":        "troubleshoot_v1beta2_host_preflight_validate_excluded_analyzers_gotest.yaml",
		"uploadResultsPreflightFile":                "troubleshoot_v1beta2_preflight_validate_spec_with_upload_results_gotest.yaml",
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
		{
			name:          "no-host-preflight-collectores",
			preflightSpec: testingFiles["noCollectorsHostPreflightFile"],
			wantWarning:   types.NewExitCodeWarning("No collectors found"),
		},
		{
			name:          "no-host-preflight-analyzers",
			preflightSpec: testingFiles["noAnalyzersHostPreflightFile"],
			wantWarning:   types.NewExitCodeWarning("No analyzers found"),
		},
		{
			name:          "excluded-host-preflight-collectors",
			preflightSpec: testingFiles["excludedHostCollectorsPreflightFile"],
			wantWarning:   types.NewExitCodeWarning("All collectors were excluded by the applied values"),
		},
		{
			name:          "excluded-host-preflight-analyzers",
			preflightSpec: testingFiles["excludedHostAnalyzersPreflightFile"],
			wantWarning:   types.NewExitCodeWarning("All analyzers were excluded by the applied values"),
		},
		{
			name:          "upload-results-preflight",
			preflightSpec: testingFiles["uploadResultsPreflightFile"],
			wantWarning:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFilePath := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/"+tt.preflightSpec)
			kinds, err := readSpecs([]string{testFilePath})
			require.NoError(t, err)
			gotWarning := validatePreflight(kinds)
			assert.Equal(t, tt.wantWarning, gotWarning)
		})
	}
}
