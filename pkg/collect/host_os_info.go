package collect

import (
	"bytes"
	"encoding/json"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	osutils "github.com/shirou/gopsutil/v4/host"
)

type HostOSInfo struct {
	Name            string `json:"name"`
	KernelVersion   string `json:"kernelVersion"`
	PlatformVersion string `json:"platformVersion"`
	Platform        string `json:"platform"`
	PlatformFamily  string `json:"platformFamily"`
}

type HostOSInfoNodes struct {
	Nodes []string `json:"nodes"`
}

const HostOSInfoPath = `host-collectors/system/hostos_info.json`
const NodeInfoBaseDir = `host-collectors/system`
const HostInfoFileName = `hostos_info.json`

type CollectHostOS struct {
	hostCollector *troubleshootv1beta2.HostOS
	BundlePath    string
}

func (c *CollectHostOS) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Host OS Info")
}

func (c *CollectHostOS) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostOS) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostOS) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	infoStat, err := osutils.Info()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get os info")
	}
	hostInfo := HostOSInfo{}
	hostInfo.Platform = infoStat.Platform
	hostInfo.PlatformFamily = infoStat.PlatformFamily
	hostInfo.KernelVersion = infoStat.KernelVersion
	hostInfo.Name = infoStat.Hostname
	hostInfo.PlatformVersion = infoStat.PlatformVersion

	b, err := json.MarshalIndent(hostInfo, "", " ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal host os info")
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, HostOSInfoPath, bytes.NewBuffer(b))

	return output, nil
}
