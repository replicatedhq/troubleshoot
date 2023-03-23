package collect

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectHostCopy_Collect_WithFileName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.txt")
	testutils.CreateTestFile(t, path)

	c := &CollectHostCopy{
		hostCollector: &troubleshootv1beta2.HostCopy{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: "test-1",
			},
			Path: path,
		},
	}
	got, err := c.Collect(nil)
	require.NoError(t, err)
	testutils.LogJSON(t, got)

	assert.Len(t, got, 1)
	assert.Contains(t, got, "host-collectors/test-1/test.txt")
}

func TestCollectHostCopy_Collect_WithDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-dir")
	testutils.CreateTestFile(t, filepath.Join(dir, "test-1.txt"))
	testutils.CreateTestFile(t, filepath.Join(dir, "subdir", "another.txt"))

	c := &CollectHostCopy{
		hostCollector: &troubleshootv1beta2.HostCopy{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: "test-2",
			},
			Path: dir,
		},
	}
	got, err := c.Collect(nil)
	require.NoError(t, err)
	testutils.LogJSON(t, got)

	assert.Len(t, got, 2)
	assert.Contains(t, got, "host-collectors/test-2/test-dir/test-1.txt")
	assert.Contains(t, got, "host-collectors/test-2/test-dir/subdir/another.txt")
}

func TestCollectHostCopy_Collect_UsingGlobWildcard(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-dir")
	testutils.CreateTestFile(t, filepath.Join(dir, "not-collected.txt"))
	testutils.CreateTestFile(t, filepath.Join(dir, "subdir", "another.txt"))

	c := &CollectHostCopy{
		hostCollector: &troubleshootv1beta2.HostCopy{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: "test-3",
			},
			Path: dir + "/su*",
		},
	}
	got, err := c.Collect(nil)
	require.NoError(t, err)
	testutils.LogJSON(t, got)

	assert.Len(t, got, 1)
	assert.Contains(t, got, "host-collectors/test-3/subdir/another.txt")
}

func TestCollectHostCopy_Collect_WithFile_AbsolutePathSymlinks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-dir")
	testutils.CreateTestFile(t, filepath.Join(dir, "subdir", "another.txt"))

	target := filepath.Join(t.TempDir(), "target.txt")
	testutils.CreateTestFileWithData(t, target, "symlink target data")
	require.NoError(t, os.Symlink(target, filepath.Join(dir, "symlink.txt")))

	targetDir := filepath.Join(t.TempDir(), "targetDir")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	fileInSymlinkedDir := filepath.Join(targetDir, "symlink-in-dir.txt")
	testutils.CreateTestFileWithData(t, fileInSymlinkedDir, "target data in dir")
	require.NoError(t, os.Symlink(targetDir, filepath.Join(dir, "symlink-dir")))

	// Creates
	// /tmp/random/path-1/target.txt
	// /tmp/random/path-2/targetDir/symlink-in-dir.txt
	//
	// test-dir/
	// ├── subdir/
	// │   └── another.txt
	// ├── symlink.txt -> /tmp/random/path-1/target.txt
	// └── symlink-dir -> /tmp/random/path-2/targetDir

	c := &CollectHostCopy{
		hostCollector: &troubleshootv1beta2.HostCopy{
			Path: dir,
		},
	}
	got, err := c.Collect(nil)
	require.NoError(t, err)
	testutils.LogJSON(t, got)

	assert.Len(t, got, 3)
	assert.Contains(t, got, "host-collectors/copy/test-dir/subdir/another.txt")
	assert.Contains(t, got, "host-collectors/copy/test-dir/symlink.txt")
	assert.Equal(t, got["host-collectors/copy/test-dir/symlink.txt"], []byte("symlink target data"))
	assert.Contains(t, got, "host-collectors/copy/test-dir/symlink-dir/symlink-in-dir.txt")
	assert.Equal(t, got["host-collectors/copy/test-dir/symlink-dir/symlink-in-dir.txt"], []byte("target data in dir"))
}

func TestCollectHostCopy_Collect_WithRelativePathSymlinks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-dir")
	testutils.CreateTestFile(t, filepath.Join(dir, "subdir", "another.txt"))

	target := filepath.Join(t.TempDir(), "target.txt")
	testutils.CreateTestFileWithData(t, target, "symlink target data")
	relTargetPath, err := filepath.Rel(dir, target)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(relTargetPath, filepath.Join(dir, "symlink.txt")))

	targetDir := filepath.Join(t.TempDir(), "targetDir")
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	fileInSymlinkedDir := filepath.Join(targetDir, "symlink-in-dir.txt")
	testutils.CreateTestFileWithData(t, fileInSymlinkedDir, "target data in dir")
	relTargetDirPath, err := filepath.Rel(dir, targetDir)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(relTargetDirPath, filepath.Join(dir, "symlink-dir")))

	// Creates
	// /tmp/random/path-1/target.txt
	// /tmp/random/path-2/targetDir/symlink-in-dir.txt
	//
	// test-dir/
	// ├── subdir/
	// │   └── another.txt
	// ├── symlink.txt -> ../../../target.txt
	// └── symlink-dir -> ../../../targetDir

	c := &CollectHostCopy{
		hostCollector: &troubleshootv1beta2.HostCopy{
			Path: dir,
		},
	}
	got, err := c.Collect(nil)
	require.NoError(t, err)
	testutils.LogJSON(t, got)

	assert.Len(t, got, 3)
	assert.Contains(t, got, "host-collectors/copy/test-dir/subdir/another.txt")
	assert.Contains(t, got, "host-collectors/copy/test-dir/symlink.txt")
	assert.Equal(t, got["host-collectors/copy/test-dir/symlink.txt"], []byte("symlink target data"))
	assert.Contains(t, got, "host-collectors/copy/test-dir/symlink-dir/symlink-in-dir.txt")
	assert.Equal(t, got["host-collectors/copy/test-dir/symlink-dir/symlink-in-dir.txt"], []byte("target data in dir"))
}

func TestCollectHostCopy_Collect_WithPipeSymlink_IgnoresUnknownFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-dir")
	targetDir := filepath.Join(t.TempDir(), "test-dir2")
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.MkdirAll(targetDir, 0755))
	require.NoError(t, syscall.Mkfifo(filepath.Join(targetDir, "my.fifo"), 0666))
	require.NoError(t, os.Symlink(targetDir, filepath.Join(dir, "symlink-dir")))

	c := &CollectHostCopy{
		hostCollector: &troubleshootv1beta2.HostCopy{
			Path: dir,
		},
	}
	got, err := c.Collect(nil)
	require.NoError(t, err)
	testutils.LogJSON(t, got)
}

func TestCollectHostCopy_Collect_WithPipe_IgnoresUnknownFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test-dir")
	testutils.CreateTestFile(t, filepath.Join(dir, "some.txt"))
	require.NoError(t, syscall.Mkfifo(filepath.Join(dir, "my.fifo"), 0666))

	c := &CollectHostCopy{
		hostCollector: &troubleshootv1beta2.HostCopy{
			Path: dir,
		},
	}
	got, err := c.Collect(nil)
	require.NoError(t, err)
	testutils.LogJSON(t, got)

	assert.Len(t, got, 1)
	assert.Contains(t, got, "host-collectors/copy/test-dir/some.txt")
}

func TestCollectHostCopy_Collect_NoPath(t *testing.T) {
	c := &CollectHostCopy{
		hostCollector: &troubleshootv1beta2.HostCopy{
			Path: "",
		},
	}
	got, err := c.Collect(nil)
	require.NoError(t, err)
	testutils.LogJSON(t, got)

	assert.Len(t, got, 0)
}
