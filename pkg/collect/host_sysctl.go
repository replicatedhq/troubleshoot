package collect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os/exec"
	"regexp"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

// Ensure `CollectHostSysctl` implements `HostCollector` interface at compile time.
var _ HostCollector = (*CollectHostSysctl)(nil)

// Helper var to allow stubbing `exec.Command` for tests
var execCommand = exec.Command

const HostSysctlPath = `host-collectors/system/sysctl.json`
const HostSysctlFileName = `sysctl.json`

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

func (c *CollectHostSysctl) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostSysctl) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	klog.V(2).Info("Running sysctl collector")
	cmd := execCommand("sysctl", "-a")
	out, err := cmd.Output()
	if err != nil {
		klog.V(2).ErrorS(err, "failed to run sysctl")
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, errors.Wrapf(err, "failed to run sysctl exit-code=%d stderr=%s", exitErr.ExitCode(), exitErr.Stderr)
		} else {
			return nil, errors.Wrap(err, "failed to run sysctl")
		}
	}
	values := parseSysctlParameters(out)

	payload, err := json.Marshal(values)
	if err != nil {
		klog.V(2).ErrorS(err, "failed to marshal data to json")
		return nil, errors.Wrap(err, "failed to marshal data to json")
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, HostSysctlPath, bytes.NewBuffer(payload))
	klog.V(2).Info("Finished writing JSON output")
	return output, nil
}

// Linux sysctl outputs <key> = <value> where in Darwin you get <key> : <value>
// where <value> can be a string, number or multiple space separated strings
var sysctlLineRegex = regexp.MustCompile(`(\S+)\s*(=|:)\s*(.*)$`)

func parseSysctlParameters(output []byte) map[string]string {
	scanner := bufio.NewScanner(bytes.NewReader(output))

	result := map[string]string{}
	for scanner.Scan() {
		l := scanner.Text()
		// <1:key> <2:separator> <3:value>
		matches := sysctlLineRegex.FindStringSubmatch(l)

		switch len(matches) {
		// there are no matches for the value and separator, ignore and log
		case 0, 1, 2:
			klog.V(2).Infof("skipping sysctl line since we found no matches for it: %s", l)
		// key exists but value could be empty, register as an empty string value but log something for reference
		case 3:
			klog.V(2).Infof("found no value for sysctl line, keeping it with an empty value: %s", l)
			result[matches[1]] = ""
		default:
			result[matches[1]] = matches[3]
		}
	}
	return result
}
