// This Control Groups collector is heavily based on k0s'
// probes implementation https://github.com/k0sproject/k0s/blob/main/internal/pkg/sysinfo/probes/linux/cgroups.go

package collect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

const hostCGroupsPath = `host-collectors/system/cgroups.json`

type CollectHostCGroups struct {
	hostCollector *troubleshootv1beta2.HostCGroups
	BundlePath    string
}

type cgroupResult struct {
	Enabled     bool     `json:"enabled"`
	MountPoint  string   `json:"mountPoint"`
	Controllers []string `json:"controllers"`
}

type cgroupsResult struct {
	CGroupEnabled bool         `json:"cgroup-enabled"`
	CGroupV1      cgroupResult `json:"cgroup-v1"`
	CGroupV2      cgroupResult `json:"cgroup-v2"`
	// AllControllers is a list of all cgroup controllers found in the system
	AllControllers []string `json:"allControllers"`
}

func (c *CollectHostCGroups) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "cgroups")
}

func (c *CollectHostCGroups) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostCGroups) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	// https://man7.org/linux/man-pages/man7/cgroups.7.html
	// Implementation is based on https://github.com/k0sproject/k0s/blob/main/internal/pkg/sysinfo/probes/linux/cgroups.go

	if c.hostCollector.MountPoint == "" {
		c.hostCollector.MountPoint = "/sys/fs/cgroup"
	}

	results, err := discoverConfiguration(c.hostCollector.MountPoint)
	if err != nil {
		return nil, err
	}

	// Save the results
	resultsJson, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return nil, err
	}

	output := NewResult()
	err = output.SaveResult(c.BundlePath, hostCGroupsPath, bytes.NewBuffer(resultsJson))
	if err != nil {
		return nil, err
	}

	return output, nil
}

func parseV1ControllerNames(r io.Reader) ([]string, error) {
	names := []string{}
	var lineNo uint
	lines := bufio.NewScanner(r)
	for lines.Scan() {
		lineNo = lineNo + 1
		if err := lines.Err(); err != nil {
			return nil, fmt.Errorf("failed to parse /proc/cgroups at line %d: %w ", lineNo, err)
		}
		text := lines.Text()
		if len(text) == 0 {
			continue
		}

		if text[0] != '#' {
			parts := strings.Fields(text)
			if len(parts) >= 4 && parts[3] != "0" {
				names = append(names, parts[0])
			}
		}
	}
	klog.V(2).Info("cgroup v1 controllers loaded")

	return names, nil
}
