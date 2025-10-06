package collect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

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
	// 1. Construct subdirectory path in the bundle path to copy files into
	// output.SaveResult() will create the directory if it doesn't exist
	bundleRelPath := filepath.Join("host-collectors", c.Title())
	bundlePathDest := filepath.Join(c.BundlePath, bundleRelPath)

	// 2. Enumerate all files that match the glob pattern
	paths, err := filepath.Glob(c.hostCollector.Path)
	if err != nil {
		klog.Errorf("Invalid glob pattern %q: %v", c.hostCollector.Path, err)
		return nil, err
	}

	if len(paths) == 0 {
		klog.V(1).Info("No files found to copy")
		return NewResult(), nil
	}

	// 3. Copy content in found host paths to the subdirectory
	klog.V(1).Infof("Copy files from %q to %q", c.hostCollector.Path, bundleRelPath)
	result, err := c.copyFilesToBundle(paths, bundlePathDest)
	if err != nil {
		klog.Errorf("Failed to copy files from %q to %q: %v", c.hostCollector.Path, "<bundle>/"+bundleRelPath, err)
		fileName := fmt.Sprintf("%s/errors.json", c.relBundlePath(bundlePathDest))
		output := NewResult()
		saveErr := output.SaveResult(c.BundlePath, fileName, marshalErrors([]string{err.Error()}))
		if saveErr != nil {
			return nil, saveErr
		}
		return output, err
	}

	return result, nil
}

func (c *CollectHostCopy) relBundlePath(path string) string {
	s := strings.ReplaceAll(path, c.BundlePath, "")
	return strings.TrimPrefix(s, "/")
}

// copyFilesToBundle copies files from the host to the bundle.
func (c *CollectHostCopy) copyFilesToBundle(paths []string, dstDir string) (CollectorResult, error) {
	result := NewResult()

	for _, path := range paths {
		dst := filepath.Join(dstDir, filepath.Base(path))
		err := c.doCopy(path, dst, result)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (c *CollectHostCopy) doCopy(src, dst string, result CollectorResult) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if srcInfo.Mode().IsRegular() {
		err := c.copyFile(src, dst, result)
		if err != nil {
			return err
		}
	} else if srcInfo.IsDir() {
		err := c.copyDir(src, dst, result)
		if err != nil {
			return err
		}
	} else {
		klog.V(2).Infof("Skipping non-file, non-directory path: %q", src)
	}

	return nil
}

// copyFile copies a file from src to the bundle
func (c *CollectHostCopy) copyFile(src, dst string, result CollectorResult) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	relDest := c.relBundlePath(dst)
	err = result.SaveResult(c.BundlePath, relDest, in)
	if err != nil {
		return err
	}
	return nil
}

// copyDir recursively copies a directory tree to the bundle
func (c *CollectHostCopy) copyDir(src, dst string, result CollectorResult) error {
	// Enunmerate all entries in the source dir
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPointer := filepath.Join(src, entry.Name())
		dstPointer := filepath.Join(dst, entry.Name())

		err = c.doCopy(srcPointer, dstPointer, result)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *CollectHostCopy) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
