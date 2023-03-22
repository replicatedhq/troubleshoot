package collect

import (
	"os"
	"path/filepath"

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

	bundlePathDest := filepath.Join(c.BundlePath, "host-collectors", c.Title())
	err := os.MkdirAll(bundlePathDest, 0755)	// Allow rwx for owner, r-x for group and others
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(bundlePathDest)
	klog.V(1).Infof("Collecting host files to %q", c.hostCollector.Path)

	// Make a subdirectory in the bundle path to copy files into
	copyToBundle("path", "bundleDir")

	// After successful copy, add the subdirectory with collected files to the output
	// TODO: Handle empty directory. Count of copied files perhaps?
	output := NewResult()
	return output, nil
}

func copyToBundle(path, bundleDir string) error {
	// Copy files from host
	// Use cases:
	// 1. Single file
	// 2. Directory with files and other directories
	// 3. Directory with symlinks & hardlinks to files/directories
	// 4. Symlink/hardlink to a file or directory
	// 5. Preserve permissions & ownership
	// 6. Permission/or ownership errors should not fail the entire copy. Log the error and continue.
	// 7. Paths that are not files or directories should be ignored e.g devices, unix sockets. Logs should be emitted for these.
	// 8. dot paths should be copied
	return nil
}
