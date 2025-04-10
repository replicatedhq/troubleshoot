package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/shirou/gopsutil/v4/disk"
)

type DiskUsageInfo struct {
	TotalBytes uint64 `json:"total_bytes"`
	UsedBytes  uint64 `json:"used_bytes"`
}

type CollectHostDiskUsage struct {
	hostCollector *troubleshootv1beta2.DiskUsage
	BundlePath    string
}

func (c *CollectHostDiskUsage) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, fmt.Sprintf("Disk Usage %s", c.hostCollector.CollectorName))
}

func (c *CollectHostDiskUsage) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostDiskUsage) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostDiskUsage) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	result := map[string][]byte{}

	if c.hostCollector == nil {
		return map[string][]byte{}, nil
	}

	pathExists, err := traverseFiletreeDirExists(c.hostCollector.Path)
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

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "diskUsage"
	}
	name := filepath.Join("host-collectors/diskUsage", collectorName+".json")

	result[name] = b

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return result, nil
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

func (c *CollectHostDiskUsage) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
