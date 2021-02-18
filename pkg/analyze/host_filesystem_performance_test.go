package analyzer

import (
	"encoding/json"
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeHostFilesystemPerformance(t *testing.T) {
	tests := []struct {
		name         string
		fsPerf       *collect.FSPerfResults
		hostAnalyzer *troubleshootv1beta2.FilesystemPerformanceAnalyze
		result       *AnalyzeResult
		expectErr    bool
	}{
		{
			name: "Cover",
			fsPerf: &collect.FSPerfResults{
				Min:   200 * time.Nanosecond,
				Max:   time.Second,
				P1:    1 * time.Microsecond,
				P5:    5 * time.Microsecond,
				P10:   10 * time.Microsecond,
				P20:   20 * time.Microsecond,
				P30:   30 * time.Microsecond,
				P40:   40 * time.Microsecond,
				P50:   50 * time.Microsecond,
				P60:   60 * time.Microsecond,
				P70:   70 * time.Microsecond,
				P80:   80 * time.Microsecond,
				P90:   90 * time.Microsecond,
				P95:   95 * time.Microsecond,
				P99:   99 * time.Microsecond,
				P995:  995 * time.Microsecond,
				P999:  999 * time.Microsecond,
				P9995: 5 * time.Millisecond,
				P9999: 9 * time.Millisecond,
			},
			hostAnalyzer: &troubleshootv1beta2.FilesystemPerformanceAnalyze{
				CollectorName: "etcd",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "min == 0",
							Message: "min not working",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "min <= 50ns",
							Message: "lte operator not working",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "max == 0",
							Message: "max not working",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "max >= 1m",
							Message: "gte operator not working",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p1 < 1us",
							Message: "P1 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p1 > 1us",
							Message: "P1 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p5 < 5us",
							Message: "P5 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p5 > 5us",
							Message: "P5 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p10 < 10us",
							Message: "P10 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p10 > 10us",
							Message: "P10 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p20 < 20us",
							Message: "P20 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p20 > 20us",
							Message: "P20 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p30 < 30us",
							Message: "P30 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p30 > 30us",
							Message: "P30 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p40 < 40us",
							Message: "P40 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p40 > 40us",
							Message: "P40 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p50 < 50us",
							Message: "P50 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p50 > 50us",
							Message: "P50 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p60 < 60us",
							Message: "P60 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p60 > 60us",
							Message: "P60 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p70 < 70us",
							Message: "P70 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p70 > 70us",
							Message: "P70 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p80 < 80us",
							Message: "P80 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p80 > 80us",
							Message: "P80 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p90 < 90us",
							Message: "P90 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p90 > 90us",
							Message: "P90 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p95 < 95us",
							Message: "P95 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p95 > 95us",
							Message: "P95 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p99 < 99us",
							Message: "P99 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p99 > 99us",
							Message: "P99 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p995 < 995us",
							Message: "P995 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p995 > 995us",
							Message: "P995 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p999 < 999us",
							Message: "P999 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p999 > 999us",
							Message: "P999 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p9995 < 5ms",
							Message: "P9995 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p9995 > 5ms",
							Message: "P9995 too high",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p9999 < 9ms",
							Message: "P9999 too low",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p9999 > 9ms",
							Message: "P9999 too high",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "p9999 < 10ms",
							Message: "Acceptable write latency",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Filesystem Performance",
				IsPass:  true,
				Message: "Acceptable write latency",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			b, err := json.Marshal(test.fsPerf)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := analyzeHostFilesystemPerformance(test.hostAnalyzer, getCollectedFileContents)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
