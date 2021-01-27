package collect

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/cpu"
)

type CPUInfo struct {
	LogicalCount  int `json:"logicalCount"`
	PhysicalCount int `json:"physicalCount"`
}

func HostCPU(c *HostCollector) (map[string][]byte, error) {
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

	b, err := json.Marshal(cpuInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal cpu info")
	}

	return map[string][]byte{
		"system/cpu.json": b,
	}, nil
}
