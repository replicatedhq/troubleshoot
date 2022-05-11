package collect

import (
	"bytes"
	"encoding/json"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/shirou/gopsutil/cpu"
)

type CPUInfo struct {
	LogicalCount  int `json:"logicalCount"`
	PhysicalCount int `json:"physicalCount"`
}

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

	b, err := json.Marshal(cpuInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal cpu info")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "cpu"
	}
	name := filepath.Join("system", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return map[string][]byte{
		name: b,
	}, nil
}
