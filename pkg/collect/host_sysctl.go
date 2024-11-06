package collect

import (
	"bytes"
	"os/exec"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

var _ HostCollector = (*CollectHostSysctl)(nil)

var execCommand = exec.Command

const HostSysctlPath = `host-collectors/system/sysctl.txt`

type CollectHostSysctl struct {
	hostCollector *troubleshootv1beta2.HostSysctl
	BundlePath    string
}

func (c *CollectHostSysctl) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Sysctl")
}

func (c *CollectHostSysctl) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostSysctl) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {

	cmd := execCommand("sysctl", "-a")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, errors.Wrapf(err, "failed to run sysctl exit-code=%d stderr=%s", exitErr.ExitCode(), exitErr.Stderr)
		} else {
			return nil, errors.Wrap(err, "failed to run sysctl")
		}
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, HostSysctlPath, bytes.NewBuffer(out))
	return output, nil
}
