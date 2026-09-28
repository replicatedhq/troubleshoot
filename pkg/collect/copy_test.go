package collect

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type tarEntry struct {
	name     string
	mode     int64
	content  string // ignored for directories and symlinks
	linkname string // makes the entry a symlink
}

func buildTar(t *testing.T, entries []tarEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		hdr := &tar.Header{Name: e.name, Mode: e.mode, Typeflag: tar.TypeReg, Size: int64(len(e.content))}
		if e.name[len(e.name)-1] == '/' {
			hdr.Typeflag = tar.TypeDir
			hdr.Size = 0
		} else if e.linkname != "" {
			hdr.Typeflag = tar.TypeSymlink
			hdr.Linkname = e.linkname
			hdr.Size = 0
		}
		require.NoError(t, tw.WriteHeader(hdr))
		if hdr.Typeflag == tar.TypeReg {
			_, err := tw.Write([]byte(e.content))
			require.NoError(t, err)
		}
	}
	require.NoError(t, tw.Close())

	// GNU tar pads archives to a 10240 byte record, beyond the end-of-archive marker
	padding := 10240 - buf.Len()%10240
	buf.Write(make([]byte, padding))

	return buf.Bytes()
}

// folderTar mimics `tar -C /tmp -cf - test` on a folder with a subfolder, a read-only subfolder
// and symlinks
func folderTar(t *testing.T) []byte {
	return buildTar(t, []tarEntry{
		{name: "test/", mode: 0755},
		{name: "test/a.txt", mode: 0644, content: "a"},
		{name: "test/link", mode: 0777, linkname: "a.txt"},
		{name: "test/passwd", mode: 0777, linkname: "/etc/passwd"},
		{name: "test/ro/", mode: 0555},
		{name: "test/ro/r.txt", mode: 0644, content: "r"},
		{name: "test/sub/", mode: 0755},
		{name: "test/sub/b.txt", mode: 0644, content: "b"},
	})
}

func Test_extractTar(t *testing.T) {
	dstPath := t.TempDir()
	result := NewResult()

	err := extractTar(bytes.NewReader(folderTar(t)), dstPath, result)
	require.NoError(t, err)

	assert.Equal(t, CollectorResult{
		"test/a.txt":          nil,
		"test/link.symlink":   nil,
		"test/passwd.symlink": nil,
		"test/ro/r.txt":       nil,
		"test/sub/b.txt":      nil,
	}, result)

	for file, content := range map[string]string{
		"test/a.txt":          "a",
		"test/link.symlink":   "a.txt\n",
		"test/passwd.symlink": "/etc/passwd\n",
		"test/ro/r.txt":       "r",
		"test/sub/b.txt":      "b",
	} {
		info, err := os.Lstat(filepath.Join(dstPath, file))
		require.NoError(t, err)
		assert.True(t, info.Mode().IsRegular(), "%s should be a regular file", file)

		data, err := os.ReadFile(filepath.Join(dstPath, file))
		require.NoError(t, err)
		assert.Equal(t, content, string(data))
	}

	// Symlinks themselves are never created
	assert.NoFileExists(t, filepath.Join(dstPath, "test/link"))
	assert.NoFileExists(t, filepath.Join(dstPath, "test/passwd"))
}

func Test_extractTar_rejectsPathsOutsideDestination(t *testing.T) {
	for _, name := range []string{"../evil.txt", "test/../../evil.txt", "/tmp/evil.txt"} {
		t.Run(name, func(t *testing.T) {
			parent := t.TempDir()
			dstPath := filepath.Join(parent, "bundle")
			result := NewResult()

			data := buildTar(t, []tarEntry{{name: name, mode: 0644, content: "evil"}})
			err := extractTar(bytes.NewReader(data), dstPath, result)
			require.Error(t, err)

			assert.Empty(t, result)
			assert.NoFileExists(t, filepath.Join(parent, "evil.txt"))
		})
	}
}

func Test_newTarExtractor(t *testing.T) {
	dstPath := t.TempDir()
	result := NewResult()

	w, wait := newTarExtractor(dstPath, result)

	// Write in chunks like exec.Stream does. Every write, including the trailing padding,
	// must succeed rather than fail with "read/write on closed pipe".
	_, err := io.CopyBuffer(w, bytes.NewReader(folderTar(t)), make([]byte, 1000))
	require.NoError(t, err)

	require.NoError(t, wait(nil))
	assert.Len(t, result, 5)
	assert.FileExists(t, filepath.Join(dstPath, "test/ro/r.txt"))
}

func Test_newTarExtractor_returnsExtractionError(t *testing.T) {
	w, wait := newTarExtractor(t.TempDir(), NewResult())

	data := buildTar(t, []tarEntry{
		{name: "../evil.txt", mode: 0644, content: "evil"},
		{name: "test/a.txt", mode: 0644, content: "a"},
	})

	// The writer is unblocked instead of hanging once extraction fails
	_, err := w.Write(data)
	require.Error(t, err)

	err = wait(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside the destination directory")
}

func Test_newTarExtractor_streamError(t *testing.T) {
	w, wait := newTarExtractor(t.TempDir(), NewResult())

	// A stream that ends partway through the archive
	data := folderTar(t)
	_, err := w.Write(data[:700])
	require.NoError(t, err)

	streamErr := errors.New("command terminated with exit code 2")
	require.Error(t, wait(streamErr))
}
