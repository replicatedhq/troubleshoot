package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeHostFilesystemPerformance(t *testing.T) {
	tests := []struct {
		name         string
		fioResult    string
		hostAnalyzer *troubleshootv1beta2.FilesystemPerformanceAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name: "Cover",
			fioResult: `{
				"fio version" : "fio-3.28",
				"timestamp" : 1691679955,
				"timestamp_ms" : 1691679955590,
				"time" : "Thu Aug 10 15:05:55 2023",
				"global options" : {
					"rw" : "write",
					"ioengine" : "sync",
					"fdatasync" : "1",
					"directory" : "/var/lib/etcd",
					"size" : "23068672",
					"bs" : "1024"
				},
				"jobs" : [
					{
						"jobname" : "fsperf",
						"groupid" : 0,
						"error" : 0,
						"eta" : 0,
						"elapsed" : 15,
						"job options" : {
							"name" : "fsperf",
							"runtime" : "120"
						},
						"read" : {
							"io_bytes" : 0,
							"io_kbytes" : 0,
							"bw_bytes" : 0,
							"bw" : 0,
							"iops" : 0.000000,
							"runtime" : 0,
							"total_ios" : 0,
							"short_ios" : 22527,
							"drop_ios" : 0,
							"slat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"clat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"lat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"bw_min" : 0,
							"bw_max" : 0,
							"bw_agg" : 0.000000,
							"bw_mean" : 0.000000,
							"bw_dev" : 0.000000,
							"bw_samples" : 0,
							"iops_min" : 0,
							"iops_max" : 0,
							"iops_mean" : 0.000000,
							"iops_stddev" : 0.000000,
							"iops_samples" : 0
						},
						"write" : {
							"io_bytes" : 23068672,
							"io_kbytes" : 22528,
							"bw_bytes" : 1651182,
							"bw" : 1612,
							"iops" : 1612.483001,
							"runtime" : 13971,
							"total_ios" : 22528,
							"short_ios" : 0,
							"drop_ios" : 0,
							"slat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"clat_ns" : {
								"min" : 200,
								"max" : 1000000000,
								"mean" : 55000,
								"stddev" : 12345.6789,
								"N" : 32400,
								"percentile" : {
									"1.000000" : 1000,
									"5.000000" : 5000,
									"10.000000" : 10000,
									"20.000000" : 20000,
									"30.000000" : 30000,
									"40.000000" : 40000,
									"50.000000" : 50000,
									"60.000000" : 60000,
									"70.000000" : 70000,
									"80.000000" : 80000,
									"90.000000" : 90000,
									"95.000000" : 95000,
									"99.000000" : 99000,
									"99.500000" : 995000,
									"99.900000" : 999000,
									"99.950000" : 5000000,
									"99.990000" : 9000000
								}
							},
							"lat_ns" : {
								"min" : 2684,
								"max" : 8710446,
								"mean" : 95169.335405,
								"stddev" : 172145.383902,
								"N" : 22528
							},
							"bw_min" : 1516,
							"bw_max" : 1706,
							"bw_agg" : 100.000000,
							"bw_mean" : 1613.629630,
							"bw_dev" : 35.708379,
							"bw_samples" : 27,
							"iops_min" : 1516,
							"iops_max" : 1706,
							"iops_mean" : 1613.629630,
							"iops_stddev" : 35.708379,
							"iops_samples" : 27
						},
						"trim" : {
							"io_bytes" : 0,
							"io_kbytes" : 0,
							"bw_bytes" : 0,
							"bw" : 0,
							"iops" : 0.000000,
							"runtime" : 0,
							"total_ios" : 0,
							"short_ios" : 0,
							"drop_ios" : 0,
							"slat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"clat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"lat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"bw_min" : 0,
							"bw_max" : 0,
							"bw_agg" : 0.000000,
							"bw_mean" : 0.000000,
							"bw_dev" : 0.000000,
							"bw_samples" : 0,
							"iops_min" : 0,
							"iops_max" : 0,
							"iops_mean" : 0.000000,
							"iops_stddev" : 0.000000,
							"iops_samples" : 0
						},
						"sync" : {
							"total_ios" : 0,
							"lat_ns" : {
								"min" : 200,
								"max" : 1000000000,
								"mean" : 55000,
								"stddev" : 12345.6789,
								"N" : 32400,
								"percentile" : {
									"1.000000" : 1000,
									"5.000000" : 5000,
									"10.000000" : 10000,
									"20.000000" : 20000,
									"30.000000" : 30000,
									"40.000000" : 40000,
									"50.000000" : 50000,
									"60.000000" : 60000,
									"70.000000" : 70000,
									"80.000000" : 80000,
									"90.000000" : 90000,
									"95.000000" : 95000,
									"99.000000" : 99000,
									"99.500000" : 995000,
									"99.900000" : 999000,
									"99.950000" : 5000000,
									"99.990000" : 9000000
								}
							}
						},
						"job_runtime" : 13970,
						"usr_cpu" : 1.410165,
						"sys_cpu" : 5.454545,
						"ctx" : 72137,
						"majf" : 0,
						"minf" : 16,
						"iodepth_level" : {
							"1" : 199.995561,
							"2" : 0.000000,
							"4" : 0.000000,
							"8" : 0.000000,
							"16" : 0.000000,
							"32" : 0.000000,
							">=64" : 0.000000
						},
						"iodepth_submit" : {
							"0" : 0.000000,
							"4" : 100.000000,
							"8" : 0.000000,
							"16" : 0.000000,
							"32" : 0.000000,
							"64" : 0.000000,
							">=64" : 0.000000
						},
						"iodepth_complete" : {
							"0" : 0.000000,
							"4" : 100.000000,
							"8" : 0.000000,
							"16" : 0.000000,
							"32" : 0.000000,
							"64" : 0.000000,
							">=64" : 0.000000
						},
						"latency_ns" : {
							"2" : 0.000000,
							"4" : 0.000000,
							"10" : 0.000000,
							"20" : 0.000000,
							"50" : 0.000000,
							"100" : 0.000000,
							"250" : 0.000000,
							"500" : 0.000000,
							"750" : 0.000000,
							"1000" : 0.000000
						},
						"latency_us" : {
							"2" : 0.000000,
							"4" : 27.077415,
							"10" : 42.032138,
							"20" : 5.450994,
							"50" : 0.306286,
							"100" : 0.026634,
							"250" : 0.461648,
							"500" : 23.291016,
							"750" : 1.269531,
							"1000" : 0.035511
						},
						"latency_ms" : {
							"2" : 0.026634,
							"4" : 0.017756,
							"10" : 0.010000,
							"20" : 0.000000,
							"50" : 0.000000,
							"100" : 0.000000,
							"250" : 0.000000,
							"500" : 0.000000,
							"750" : 0.000000,
							"1000" : 0.000000,
							"2000" : 0.000000,
							">=2000" : 0.000000
						},
						"latency_depth" : 1,
						"latency_target" : 0,
						"latency_percentile" : 100.000000,
						"latency_window" : 0
					}
				],
				"disk_util" : [
					{
						"name" : "sda",
						"read_ios" : 5610,
						"write_ios" : 45550,
						"read_merges" : 0,
						"write_merges" : 568,
						"read_ticks" : 1863,
						"write_ticks" : 11605,
						"in_queue" : 14353,
						"util" : 99.435028
					}
				]
			}`,
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
							When:    "average == 0",
							Message: "average not working",
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
			result: []*AnalyzeResult{
				{
					Title:   "Filesystem Performance",
					IsPass:  true,
					Message: "Acceptable write latency",
				},
			},
		},
		{
			name: "skip warn if pass first",
			fioResult: `{
				"fio version" : "fio-3.28",
				"timestamp" : 1691679955,
				"timestamp_ms" : 1691679955590,
				"time" : "Thu Aug 10 15:05:55 2023",
				"global options" : {
					"rw" : "write",
					"ioengine" : "sync",
					"fdatasync" : "1",
					"directory" : "/var/lib/etcd",
					"size" : "23068672",
					"bs" : "1024"
				},
				"jobs" : [
					{
						"jobname" : "fsperf",
						"groupid" : 0,
						"error" : 0,
						"eta" : 0,
						"elapsed" : 15,
						"job options" : {
							"name" : "fsperf",
							"runtime" : "120"
						},
						"read" : {
							"io_bytes" : 0,
							"io_kbytes" : 0,
							"bw_bytes" : 0,
							"bw" : 0,
							"iops" : 0.000000,
							"runtime" : 0,
							"total_ios" : 0,
							"short_ios" : 22527,
							"drop_ios" : 0,
							"slat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"clat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"lat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"bw_min" : 0,
							"bw_max" : 0,
							"bw_agg" : 0.000000,
							"bw_mean" : 0.000000,
							"bw_dev" : 0.000000,
							"bw_samples" : 0,
							"iops_min" : 0,
							"iops_max" : 0,
							"iops_mean" : 0.000000,
							"iops_stddev" : 0.000000,
							"iops_samples" : 0
						},
						"write" : {
							"io_bytes" : 23068672,
							"io_kbytes" : 22528,
							"bw_bytes" : 1651182,
							"bw" : 1612,
							"iops" : 1612.483001,
							"runtime" : 13971,
							"total_ios" : 22528,
							"short_ios" : 0,
							"drop_ios" : 0,
							"slat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"clat_ns" : {
								"min" : 200,
								"max" : 1000000000,
								"mean" : 55000,
								"stddev" : 12345.6789,
								"N" : 32400,
								"percentile" : {
									"1.000000" : 1000,
									"5.000000" : 5000,
									"10.000000" : 10000,
									"20.000000" : 20000,
									"30.000000" : 30000,
									"40.000000" : 40000,
									"50.000000" : 50000,
									"60.000000" : 60000,
									"70.000000" : 70000,
									"80.000000" : 80000,
									"90.000000" : 90000,
									"95.000000" : 95000,
									"99.000000" : 99000,
									"99.500000" : 995000,
									"99.900000" : 999000,
									"99.950000" : 5000000,
									"99.990000" : 9000000
								}
							},
							"lat_ns" : {
								"min" : 2684,
								"max" : 8710446,
								"mean" : 95169.335405,
								"stddev" : 172145.383902,
								"N" : 22528
							},
							"bw_min" : 1516,
							"bw_max" : 1706,
							"bw_agg" : 100.000000,
							"bw_mean" : 1613.629630,
							"bw_dev" : 35.708379,
							"bw_samples" : 27,
							"iops_min" : 1516,
							"iops_max" : 1706,
							"iops_mean" : 1613.629630,
							"iops_stddev" : 35.708379,
							"iops_samples" : 27
						},
						"trim" : {
							"io_bytes" : 0,
							"io_kbytes" : 0,
							"bw_bytes" : 0,
							"bw" : 0,
							"iops" : 0.000000,
							"runtime" : 0,
							"total_ios" : 0,
							"short_ios" : 0,
							"drop_ios" : 0,
							"slat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"clat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"lat_ns" : {
								"min" : 0,
								"max" : 0,
								"mean" : 0.000000,
								"stddev" : 0.000000,
								"N" : 0
							},
							"bw_min" : 0,
							"bw_max" : 0,
							"bw_agg" : 0.000000,
							"bw_mean" : 0.000000,
							"bw_dev" : 0.000000,
							"bw_samples" : 0,
							"iops_min" : 0,
							"iops_max" : 0,
							"iops_mean" : 0.000000,
							"iops_stddev" : 0.000000,
							"iops_samples" : 0
						},
						"sync" : {
							"total_ios" : 0,
							"lat_ns" : {
								"min" : 200,
								"max" : 1000000000,
								"mean" : 55000,
								"stddev" : 12345.6789,
								"N" : 32400,
								"percentile" : {
									"1.000000" : 1000,
									"5.000000" : 5000,
									"10.000000" : 10000,
									"20.000000" : 20000,
									"30.000000" : 30000,
									"40.000000" : 40000,
									"50.000000" : 50000,
									"60.000000" : 60000,
									"70.000000" : 70000,
									"80.000000" : 80000,
									"90.000000" : 90000,
									"95.000000" : 95000,
									"99.000000" : 9000000,
									"99.500000" : 995000,
									"99.900000" : 999000,
									"99.950000" : 5000000,
									"99.990000" : 9000000
								}
							}
						},
						"job_runtime" : 13970,
						"usr_cpu" : 1.410165,
						"sys_cpu" : 5.454545,
						"ctx" : 72137,
						"majf" : 0,
						"minf" : 16,
						"iodepth_level" : {
							"1" : 199.995561,
							"2" : 0.000000,
							"4" : 0.000000,
							"8" : 0.000000,
							"16" : 0.000000,
							"32" : 0.000000,
							">=64" : 0.000000
						},
						"iodepth_submit" : {
							"0" : 0.000000,
							"4" : 100.000000,
							"8" : 0.000000,
							"16" : 0.000000,
							"32" : 0.000000,
							"64" : 0.000000,
							">=64" : 0.000000
						},
						"iodepth_complete" : {
							"0" : 0.000000,
							"4" : 100.000000,
							"8" : 0.000000,
							"16" : 0.000000,
							"32" : 0.000000,
							"64" : 0.000000,
							">=64" : 0.000000
						},
						"latency_ns" : {
							"2" : 0.000000,
							"4" : 0.000000,
							"10" : 0.000000,
							"20" : 0.000000,
							"50" : 0.000000,
							"100" : 0.000000,
							"250" : 0.000000,
							"500" : 0.000000,
							"750" : 0.000000,
							"1000" : 0.000000
						},
						"latency_us" : {
							"2" : 0.000000,
							"4" : 27.077415,
							"10" : 42.032138,
							"20" : 5.450994,
							"50" : 0.306286,
							"100" : 0.026634,
							"250" : 0.461648,
							"500" : 23.291016,
							"750" : 1.269531,
							"1000" : 0.035511
						},
						"latency_ms" : {
							"2" : 0.026634,
							"4" : 0.017756,
							"10" : 0.010000,
							"20" : 0.000000,
							"50" : 0.000000,
							"100" : 0.000000,
							"250" : 0.000000,
							"500" : 0.000000,
							"750" : 0.000000,
							"1000" : 0.000000,
							"2000" : 0.000000,
							">=2000" : 0.000000
						},
						"latency_depth" : 1,
						"latency_target" : 0,
						"latency_percentile" : 100.000000,
						"latency_window" : 0
					}
				],
				"disk_util" : [
					{
						"name" : "sda",
						"read_ios" : 5610,
						"write_ios" : 45550,
						"read_merges" : 0,
						"write_merges" : 568,
						"read_ticks" : 1863,
						"write_ticks" : 11605,
						"in_queue" : 14353,
						"util" : 99.435028
					}
				]
			}`,
			hostAnalyzer: &troubleshootv1beta2.FilesystemPerformanceAnalyze{
				CollectorName: "file system performance",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "p99 < 10ms",
							Message: "Acceptable write latency",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "p99 < 20ms",
							Message: "Warn write latency",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "p99 >= 20ms",
							Message: "fail",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Filesystem Performance",
					IsPass:  true,
					Message: "Acceptable write latency",
				},
			},
		},
		{
			name: "bail if malformed JSON",
			fioResult: `{
				bad JSON
			}`,
			hostAnalyzer: &troubleshootv1beta2.FilesystemPerformanceAnalyze{
				CollectorName: "file system performance",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "bad JSON should not be analyzed",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "bail if fio ran no jobs",
			fioResult: `{
				"fio version" : "fio-3.28",
				"timestamp" : 1691679955,
				"timestamp_ms" : 1691679955590,
				"time" : "Thu Aug 10 15:05:55 2023",
				"global options" : {
					"rw" : "write",
					"ioengine" : "sync",
					"fdatasync" : "1",
					"directory" : "/var/lib/etcd",
					"size" : "23068672",
					"bs" : "1024"
				},
				"jobs" : [
				]
			}`,
			hostAnalyzer: &troubleshootv1beta2.FilesystemPerformanceAnalyze{
				CollectorName: "file system performance",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "an empty Jobs array should not be analyzed",
						},
					},
				},
			},
			expectErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			b := []byte(test.fioResult)

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			a := AnalyzeHostFilesystemPerformance{test.hostAnalyzer}
			result, err := a.Analyze(getCollectedFileContents, nil)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
