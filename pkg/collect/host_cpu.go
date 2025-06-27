package collect

import (
	"bytes"
	"encoding/json"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/host"
)

type CPUInfo struct {
	LogicalCount  int      `json:"logicalCount"`
	PhysicalCount int      `json:"physicalCount"`
	Flags         []string `json:"flags"`
	MachineArch   string   `json:"machineArch"`
}

const HostCPUPath = `host-collectors/system/cpu.json`
const HostCPUFileName = `cpu.json`

type CollectHostCPU struct {
	hostCollector *troubleshootv1beta2.CPU
	BundlePath    string
}

func (c *CollectHostCPU) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "CPU Info")
}

func (c *CollectHostCPU) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostCPU) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostCPU) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	cpuInfo := CPUInfo{}

	logicalCount, err := cpu.Counts(true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to count logical cpus")
	}
	cpuInfo.LogicalCount = logicalCount

	physicalCount, err := cpu.Counts(false)
	if err != nil {
		return nil, errors.Wrap(err, "failed to count physical cpus")
	}
	cpuInfo.PhysicalCount = physicalCount

	cpuInfo.MachineArch, err = host.KernelArch()
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch cpu architecture")
	}

	// XXX even though the cpu.Info() returns a slice per CPU it is way
	// common to have the same flags for all CPUs. We consolidate them here
	// so the output is a list of all different flags present in all CPUs.
	info, err := cpu.Info()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cpu info")
	}

	seen := make(map[string]bool)
	for _, infoForCPU := range info {
		for _, flag := range infoForCPU.Flags {
			if seen[flag] {
				continue
			}
			seen[flag] = true
			cpuInfo.Flags = append(cpuInfo.Flags, flag)
		}
	}

	b, err := json.Marshal(cpuInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal cpu info")
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, HostCPUPath, bytes.NewBuffer(b))

	return output, nil
}

func (c *CollectHostCPU) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
