package preflight

import (
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPreflightSpecsRead(t *testing.T) {
	t.Parallel()

	// A very simple preflight spec (local file)
	preflightFile := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_gotest.yaml")
	preflightSecretFile := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_secret_gotest.yaml")

	expectPreflightSpec := troubleshootv1beta2.Preflight{}
	expectHostPreflightSpec := troubleshootv1beta2.HostPreflight{}
	expectUploadResultSpecs := []*troubleshootv1beta2.Preflight{}

	tests := []struct {
		name string
		args []string
		//
		wantErr               bool
		wantPreflightSpec     *troubleshootv1beta2.Preflight
		wantHostPreflightSpec *troubleshootv1beta2.HostPreflight
		wantUploadResultSpecs []*troubleshootv1beta2.Preflight
	}{
		// TODOLATER: URL support? local mock webserver? would prefer for these tests to not require internet :)
		{
			name:                  "file-preflight",
			args:                  []string{preflightFile},
			wantErr:               false,
			wantPreflightSpec:     expectPreflightSpec,
			wantHostPreflightSpec: expectHostPreflightSpec,
			wantUploadResultSpecs: expectUploadResultSpecs,
		},
		{
			name:                  "file-secret",
			args:                  []string{preflightSecretFile},
			wantErr:               false,
			wantPreflightSpec:     expectPreflightSpec,
			wantHostPreflightSpec: expectHostPreflightSpec,
			wantUploadResultSpecs: expectUploadResultSpecs,
		},
		{
			name: "stdin-preflight",
			args: []string{"-"},
			// TODO: how do we feed in stdin?
			wantErr:               false,
			wantPreflightSpec:     expectPreflightSpec,
			wantHostPreflightSpec: expectHostPreflightSpec,
			wantUploadResultSpecs: expectUploadResultSpecs,
		},
		{
			name: "stdin-secret",
			args: []string{"-"},
			// TODO: how do we feed in stdin?
			wantErr:               false,
			wantPreflightSpec:     expectPreflightSpec,
			wantHostPreflightSpec: expectHostPreflightSpec,
			wantUploadResultSpecs: expectUploadResultSpecs,
		},
		/* TODOLATER: needs a cluster with a spec installed?
		{
			name:     "cluster-secret",
			args:     []string{"/secret/some-secret-spec"},
			wantErr:  false,
			wantPreflightSpec:     expectPreflightSpec,
			wantHostPreflightSpec: expectHostPreflightSpec,
			wantUploadResultsSpecs: expectUploadResultSpecs,
		},
		*/
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := PreflightSpecs{}
			tErr := specs.Read(tt.args)

			if tt.wantErr {
				assert.Error(t, tErr)
			} else {
				require.NoError(t, tErr)
			}

			assert.Equal(t, specs.PreflightSpec, tt.wantPreflightSpec)
			assert.Equal(t, specs.HostPreflightSpec, tt.wantHostPreflightSpec)
			assert.Equal(t, specs.UploadResultSpecs, tt.wantUploadResultsSpecs)
		})
	}
}
