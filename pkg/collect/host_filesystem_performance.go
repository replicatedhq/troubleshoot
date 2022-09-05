package collect

import (
	"bytes"
	"math"
	"math/rand"
	"text/template"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type CollectHostFilesystemPerformance struct {
	hostCollector *troubleshootv1beta2.FilesystemPerformance
	BundlePath    string
}

func (c *CollectHostFilesystemPerformance) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Filesystem Performance")
}

func (c *CollectHostFilesystemPerformance) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostFilesystemPerformance) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return collectHostFilesystemPerformance(c.hostCollector, c.BundlePath)
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
}

func getPercentileIndex(p float64, items int) int {
	if p >= 1 {
		return items - 1
	}
	return int(math.Ceil(p*float64(items))) - 1
}

var fsPerfTmpl = template.Must(template.New("").Parse(`
   Min: {{ .Min }}
   Max: {{ .Max }}
   Avg: {{ .Average }}
    p1: {{ .P1 }}
    p5: {{ .P5 }}
   p10: {{ .P10 }}
   p20: {{ .P20 }}
   p30: {{ .P30 }}
   p40: {{ .P40 }}
   p50: {{ .P50 }}
   p60: {{ .P60 }}
   p70: {{ .P70 }}
   p80: {{ .P80 }}
   p90: {{ .P90 }}
   p95: {{ .P95 }}
   p99: {{ .P99 }}
 p99.5: {{ .P995 }}
 p99.9: {{ .P999 }}
p99.95: {{ .P9995 }}
p99.99: {{ .P9999 }}`))

func (f FSPerfResults) String() string {
	var buf bytes.Buffer

	fsPerfTmpl.Execute(&buf, f)

	return buf.String()
}
