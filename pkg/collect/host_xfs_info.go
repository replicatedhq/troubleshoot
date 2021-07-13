package collect

import (
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type XFSInfo struct {
	IsXFS          bool `yaml:"isXFS"`
	IsFtypeEnabled bool `yaml:"isFtypeEnabled"`
}

type CollectHostXFSInfo struct {
	hostCollector *troubleshootv1beta2.XFSInfo
}

func (c *CollectHostXFSInfo) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Host XFS Info")
}

func (c *CollectHostXFSInfo) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostXFSInfo) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return collectXFSInfo(c.hostCollector)
}

func GetXFSPath(collectorName string) string {
	if collectorName == "" {
		return "xfs/xfs.yaml"
	}
	return fmt.Sprintf("xfs/%s.yaml", collectorName)
}
