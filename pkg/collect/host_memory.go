package collect

import (
	"bytes"
	"encoding/json"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/shirou/gopsutil/v4/mem"
)

type MemoryInfo struct {
	Total uint64 `json:"total"`
}

const HostMemoryPath = `host-collectors/system/memory.json`
const HostMemoryFileName = `memory.json`

type CollectHostMemory struct {
	hostCollector *troubleshootv1beta2.Memory
	BundlePath    string
}

func (c *CollectHostMemory) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Amount of Memory")
}

func (c *CollectHostMemory) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostMemory) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostMemory) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	memoryInfo := MemoryInfo{}

	vmstat, err := mem.VirtualMemory()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read virtual memory")
	}
	memoryInfo.Total = vmstat.Total

	b, err := json.Marshal(memoryInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal memory info")
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, HostMemoryPath, bytes.NewBuffer(b))

	return output, nil
}

func (c *CollectHostMemory) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
