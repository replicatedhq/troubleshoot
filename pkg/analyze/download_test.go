package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestDownloadAndExtractBundle(t *testing.T) {
	// TODO: Add tests for web url downloads
	tests := []struct {
		name      string
		bundleURL string
		wantErr   bool
	}{
		{
			name:      "extract a bundle from a local file path",
			bundleURL: filepath.Join(testutils.FileDir(), "../../testdata/supportbundle/support-bundle.tar.gz"),
			wantErr:   false,
		},
		{
			name:      "extract a bundle from a non-existent file path",
			bundleURL: "/home/someone/gibberish",
			wantErr:   true,
		},
		{
			name:      "extract an invalid support bundle which has no version file",
			bundleURL: filepath.Join(testutils.FileDir(), "../../testdata/supportbundle/missing-version.tar.gz"),
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, _, err := DownloadAndExtractBundle(tt.bundleURL)
			defer os.RemoveAll(tmpDir) // clean up. Ignore error

			assert.Equal(t, (err != nil), tt.wantErr)
			t.Log(err)
		})
	}
}
