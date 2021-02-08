package collect

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/mem"
)

type MemoryInfo struct {
	Total uint64 `json:"total"`
}

func HostMemory(c *HostCollector) (map[string][]byte, error) {
	memoryInfo := MemoryInfo{}

	vmstat, err := mem.VirtualMemory()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read virtual memory")
	}
	memoryInfo.Total = vmstat.Available

	b, err := json.Marshal(memoryInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal memory info")
	}

	return map[string][]byte{
		"system/memory.json": b,
	}, nil
}
