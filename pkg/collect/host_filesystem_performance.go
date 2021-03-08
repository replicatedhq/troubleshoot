package collect

import (
	"math"
	"math/rand"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type CollectHostFilesystemPerformance struct {
	hostCollector *troubleshootv1beta2.FilesystemPerformance
}

func (c *CollectHostFilesystemPerformance) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Filesystem Performance")
}

func (c *CollectHostFilesystemPerformance) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostFilesystemPerformance) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return collectHostFilesystemPerformance(c.hostCollector)
}

type FSPerfResults struct {
	Min     time.Duration
	Max     time.Duration
	Average time.Duration
	P1      time.Duration
	P5      time.Duration
	P10     time.Duration
	P20     time.Duration
	P30     time.Duration
	P40     time.Duration
	P50     time.Duration
	P60     time.Duration
	P70     time.Duration
	P80     time.Duration
	P90     time.Duration
	P95     time.Duration
	P99     time.Duration
	P995    time.Duration
	P999    time.Duration
	P9995   time.Duration
	P9999   time.Duration
	IOPS    int
}

func getPercentileIndex(p float64, items int) int {
	if p >= 1 {
		return items - 1
	}
	return int(math.Ceil(p*float64(items))) - 1
}
