package collect

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

// Use cases to test
// 1. Single file
// 2. Directory with files and other directories
// 3. Directory with symlinks & hardlinks to files/directories
// 4. Symlink/hardlink to a file or directory
// 5. Preserve permissions & ownership
// 6. Permission/or ownership errors should not fail the entire copy. Log the error and continue.
// 7. Paths that are not files or directories should be ignored e.g devices, unix sockets. Logs should be emitted for these.
// 8. dot paths should be copied

type CollectHostCopy struct {
	hostCollector *troubleshootv1beta2.HostCopy
	BundlePath    string
}

func (c *CollectHostCopy) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "copy")
}

func (c *CollectHostCopy) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostCopy) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	output := NewResult()

	// 1. Make a subdirectory in the bundle path to copy files into
	bundleRelPath := filepath.Join("host-collectors", "copy", c.Title())
	bundlePathDest := filepath.Join(c.BundlePath, bundleRelPath)
	err := os.MkdirAll(bundlePathDest, 0755)
	if err != nil {
		return nil, err
	}

	// 2. Enumerate all files that match the glob pattern
	paths, err := filepath.Glob(c.hostCollector.Path)
	if err != nil {
		klog.Errorf("invalid glob %q: %v", c.hostCollector.Path, err)
		return nil, err
	}

	if len(paths) == 0 {
		klog.V(1).Info("No files found to copy")
		return NewResult(), nil
	}

	// 3. Copy content in found host paths to the subdirectory
	klog.V(1).Infof("Copy files from %q to %q", c.hostCollector.Path, bundleRelPath)
	err = c.copyFilesToBundle(paths, bundlePathDest)
	if err != nil {
		klog.Errorf("Failed to copy files from %q to %q: %v", c.hostCollector.Path, "<bundle>" + bundleRelPath, err)
		fileName := fmt.Sprintf("%s/errors.json", bundlePathDest)
		err := output.SaveResult(c.BundlePath, fileName, marshalErrors([]string{err.Error()}))
		if err != nil {
			return nil, err
		}
		return output, nil
	}

	// 4. After successful copying, add the subdirectory with collected files to the output
	err = output.SaveDirResult(c.BundlePath, bundleRelPath)
	if err != nil {
		return nil, err
	}

	return output, nil
}

func (c *CollectHostCopy) relBundlePath(path string) string {
	return strings.ReplaceAll(path, c.BundlePath, "")
}

// copyFilesToBundle copies files from the host to the bundle.
func (c *CollectHostCopy) copyFilesToBundle(paths []string, dstDir string) error {
	for _, path := range paths {
		fileStat, err := os.Stat(path)
		if err != nil {
			return err
		}

		dst := filepath.Join(dstDir, fileStat.Name())
		if fileStat.Mode().IsRegular() {
			err = c.copyFile(path, dst)
			if err != nil {
				return err
			}
		} else if fileStat.IsDir() {
			err = c.copyDir(path, dst)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a file from src to dst. The file permissions are preserved.
func (c *CollectHostCopy) copyFile(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	// Preserve file permissions
	err = os.Chmod(dst, srcInfo.Mode())
	if err != nil {
		return err
	}

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	klog.V(2).Infof("Copied %q to %q", src, "<bundle>" + c.relBundlePath(dst))
	return nil
}

// copyDir recursively copies a directory tree, attempting to preserve permissions.
func (c *CollectHostCopy) copyDir(src, dst string) error {
	// Get properties of source dir
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination dir and preserve dir permissions
	err = os.MkdirAll(dst, srcInfo.Mode())
	if err != nil {
		return err
	}

	// Enunmerate all entries in the source dir
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPointer := filepath.Join(src, entry.Name())
		dstPointer := filepath.Join(dst, entry.Name())
		info, err := entry.Info()
		if err != nil {
			return err
		}

		if info.IsDir() {
			err = c.copyDir(srcPointer, dstPointer)
			if err != nil {
				return err
			}
		} else if info.Mode().IsRegular() {
			err = c.copyFile(srcPointer, dstPointer)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
