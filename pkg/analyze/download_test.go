package analyzer

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tarEntry describes one entry to write into an in-memory bundle archive.
type tarEntry struct {
	name     string
	typeflag byte
	linkname string
	content  string
}

// makeTarGz builds a gzipped tar archive in memory from the given entries.
func makeTarGz(t *testing.T, entries []tarEntry) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Typeflag: e.typeflag,
			Mode:     0o755,
		}
		switch e.typeflag {
		case tar.TypeReg:
			hdr.Mode = 0o644
			hdr.Size = int64(len(e.content))
		case tar.TypeSymlink:
			hdr.Linkname = e.linkname
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if e.typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(e.content))
			require.NoError(t, err)
		}
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return &buf
}

func TestExtractTroubleshootBundle(t *testing.T) {
	tests := []struct {
		name    string
		entries []tarEntry
		wantErr bool
		// verify runs after a successful extraction
		verify func(t *testing.T, destDir string)
	}{
		{
			name: "regular file within destination succeeds",
			entries: []tarEntry{
				{name: "bundle/file.txt", typeflag: tar.TypeReg, content: "hello"},
			},
			verify: func(t *testing.T, destDir string) {
				assert.FileExists(t, filepath.Join(destDir, "bundle", "file.txt"))
			},
		},
		{
			name: "directory within destination succeeds",
			entries: []tarEntry{
				{name: "bundle/subdir", typeflag: tar.TypeDir},
			},
			verify: func(t *testing.T, destDir string) {
				assert.DirExists(t, filepath.Join(destDir, "bundle", "subdir"))
			},
		},
		{
			name: "path traversal regular file is rejected",
			entries: []tarEntry{
				{name: "../outside.txt", typeflag: tar.TypeReg, content: "escape"},
			},
			wantErr: true,
		},
		{
			name: "path traversal directory is rejected",
			entries: []tarEntry{
				{name: "../outside-dir", typeflag: tar.TypeDir},
			},
			wantErr: true,
		},
		{
			name: "symlink with in-root target succeeds",
			entries: []tarEntry{
				{name: "bundle/target.txt", typeflag: tar.TypeReg, content: "data"},
				{name: "bundle/link.txt", typeflag: tar.TypeSymlink, linkname: "target.txt"},
			},
			verify: func(t *testing.T, destDir string) {
				link := filepath.Join(destDir, "bundle", "link.txt")
				fi, err := os.Lstat(link)
				require.NoError(t, err)
				assert.NotZero(t, fi.Mode()&os.ModeSymlink)
			},
		},
		{
			name: "symlink escaping the destination is rejected",
			entries: []tarEntry{
				{name: "bundle/evil-link", typeflag: tar.TypeSymlink, linkname: "../../outside-target"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destDir := t.TempDir()
			err := ExtractTroubleshootBundle(makeTarGz(t, tt.entries), destDir)

			if tt.wantErr {
				require.Error(t, err)
				// Nothing from the offending entry may exist outside destDir.
				parent := filepath.Dir(destDir)
				assert.NoFileExists(t, filepath.Join(parent, "outside.txt"))
				assert.NoDirExists(t, filepath.Join(parent, "outside-dir"))
				return
			}
			require.NoError(t, err)
			if tt.verify != nil {
				tt.verify(t, destDir)
			}
		})
	}
}

// TestExtractTroubleshootBundleSiblingPrefix ensures the containment check is
// separator-aware: an entry resolving to a sibling directory that merely shares
// the destination's string prefix (e.g. destination "/tmp/bundle" and entry
// resolving to "/tmp/bundle-escape") must be rejected.
func TestExtractTroubleshootBundleSiblingPrefix(t *testing.T) {
	parent := t.TempDir()
	destDir := filepath.Join(parent, "bundle")
	require.NoError(t, os.Mkdir(destDir, 0o755))

	archive := makeTarGz(t, []tarEntry{
		{name: "../bundle-escape/file.txt", typeflag: tar.TypeReg, content: "escape"},
	})

	err := ExtractTroubleshootBundle(archive, destDir)
	require.Error(t, err)
	assert.NoDirExists(t, filepath.Join(parent, "bundle-escape"))
}

func TestDownloadAndExtractSupportBundle(t *testing.T) {
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
			tmpDir, bundleDir, err := DownloadAndExtractSupportBundle(tt.bundleURL)
			defer os.RemoveAll(tmpDir) // clean up. Ignore error

			if err == nil {
				assert.DirExists(t, bundleDir)
				assert.FileExists(t, filepath.Join(bundleDir, "version.yaml"))
			} else {
				assert.Equal(t, "", tmpDir)
				assert.Equal(t, "", bundleDir)
			}
		})
	}
}
