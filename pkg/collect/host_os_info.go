package collect

import (
	"encoding/json"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	osutils "github.com/shirou/gopsutil/host"
)

type HostOSInfo struct {
	Name            string `json:"name"`
	KernelVersion   string `json:"kernelVersion"`
	PlatformVersion string `json:"platformVersion"`
	Platform        string `json:"platform"`
}

type CollectHostOS struct {
	hostCollector *troubleshootv1beta2.HostOS
}

func (c *CollectHostOS) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Host OS Info")
}

func (c *CollectHostOS) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostOS) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	infoStat, err := osutils.Info()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get os info")
	}
	hostInfo := HostOSInfo{}
	hostInfo.Platform = infoStat.Platform
	hostInfo.KernelVersion = infoStat.KernelVersion
	hostInfo.Name = infoStat.Hostname
	hostInfo.PlatformVersion = infoStat.PlatformVersion

	b, err := json.MarshalIndent(hostInfo, "", " ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal host os info")
	}

	return map[string][]byte{
		"system/hostos_info.json": b,
	}, nil
}
