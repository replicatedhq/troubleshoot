package collect

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/shirou/gopsutil/disk"
)

type DiskUsageInfo struct {
	TotalBytes uint64 `json:"total_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
}

func HostDiskUsage(c *HostCollector) (map[string][]byte, error) {
	result := map[string][]byte{}

	if c.Collect.DiskUsage == nil {
		return result, nil
	}

	du, err := disk.Usage(c.Collect.DiskUsage.Path)
	if err != nil {
		return result, errors.Wrapf(err, "collect disk usage for %s", c.Collect.DiskUsage.Path)
	}
	diskSpaceInfo := DiskUsageInfo{
		TotalBytes: du.Total,
		UsedBytes:  du.Used,
	}
	b, err := json.Marshal(diskSpaceInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal disk space info")
	}
	key := HostDiskUsageKey(c.Collect.DiskUsage.CollectorName)
	result[key] = b

	return result, nil
}

func HostDiskUsageKey(name string) string {
	return fmt.Sprintf("diskUsage/%s.json", name)
}
