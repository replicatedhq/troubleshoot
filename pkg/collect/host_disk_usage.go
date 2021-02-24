package collect

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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

	pathExists, err := traverseFiletreeDirExists(c.Collect.DiskUsage.Path)
	if err != nil {
		return result, errors.Wrap(err, "traverse file tree")
	}

	du, err := disk.Usage(pathExists)
	if err != nil {
		return result, errors.Wrapf(err, "collect disk usage for %s", pathExists)
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

func traverseFiletreeDirExists(filename string) (string, error) {
	filename = filepath.Clean(filename)
	for i := 0; i < 50; i++ {
		_, err := os.Stat(filename)
		if err == nil {
			return filename, nil
		} else if os.IsNotExist(err) {
			filename = filepath.Dir(filename)
			if filename == "/" {
				return filename, nil
			}
		} else {
			return "", err
		}
	}
	return "", errors.New("max recursion exceeded")
}
