package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_doCompareHostDiskUsage(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		desired  string
		actual   uint64
		expected bool
	}{
		{
			name:     ">= 1Gi, when actual is 2Gi",
			operator: ">=",
			desired:  "1Gi",
			actual:   2147483648,
			expected: true,
		},
		{
			name:     "<= 1Gi, when actual is 1GB",
			operator: "<=",
			desired:  "1Gi",
			actual:   1000000000,
			expected: true,
		},
		{
			name:     "< 20Gi, when actual is 15Gi",
			operator: "<",
			desired:  "20Gi",
			actual:   15 * 1024 * 1024 * 1024,
			expected: true,
		},
		{
			name:     "< 20Gi, when actual is 20Gi",
			operator: "<",
			desired:  "20Gi",
			actual:   20 * 1024 * 1024 * 1024,
			expected: false,
		},
		{
			name:     "> 1073741824, when actual is 1024",
			operator: ">",
			desired:  "1073741824",
			actual:   1024,
			expected: false,
		},
		{
			name:     "= 4096, when actual is 4096",
			operator: "=",
			desired:  "4096",
			actual:   4096,
			expected: true,
		},
		{
			name:     "= 4096, when actual is 1024",
			operator: "=",
			desired:  "4096",
			actual:   1024,
			expected: false,
		},
		{
			name:     "= 4096, when actual is 5000",
			operator: "=",
			desired:  "4096",
			actual:   5000,
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := doCompareHostDiskUsage(test.operator, test.desired, test.actual)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)

		})
	}
}

func Test_doCompareHostDiskUsagePercent(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		desired  string
		actual   float64
		expected bool
	}{
		{
			name:     ">= .20, when actual is .30",
			operator: ">=",
			desired:  ".20",
			actual:   .30,
			expected: true,
		},
		{
			name:     ">= .20, when actual is .10",
			operator: ">=",
			desired:  ".20",
			actual:   .10,
			expected: false,
		},
		{
			name:     ">= .20, when actual is .20",
			operator: ">=",
			desired:  ".20",
			actual:   .20,
			expected: true,
		},
		{
			name:     "> .20, when actual is .30",
			operator: ">",
			desired:  ".20",
			actual:   .30,
			expected: true,
		},
		{
			name:     "> .20, when actual is .10",
			operator: ">",
			desired:  ".20",
			actual:   .10,
			expected: false,
		},
		{
			name:     "> .20, when actual is .20",
			operator: ">",
			desired:  ".20",
			actual:   .20,
			expected: false,
		},
		{
			name:     "<= .20, when actual is .30",
			operator: "<=",
			desired:  ".20",
			actual:   .30,
			expected: false,
		},
		{
			name:     "<= .20, when actual is .10",
			operator: "<=",
			desired:  ".20",
			actual:   .10,
			expected: true,
		},
		{
			name:     "<= .20, when actual is .20",
			operator: "<=",
			desired:  ".20",
			actual:   .20,
			expected: true,
		},
		{
			name:     "< .20, when actual is .30",
			operator: "<",
			desired:  ".20",
			actual:   .30,
			expected: false,
		},
		{
			name:     "< .20, when actual is .10",
			operator: "<",
			desired:  ".20",
			actual:   .10,
			expected: true,
		},
		{
			name:     "< .20, when actual is .20",
			operator: "<",
			desired:  ".20",
			actual:   .20,
			expected: false,
		},
		{
			name:     "= .20, when actual is .30",
			operator: "=",
			desired:  ".20",
			actual:   .30,
			expected: false,
		},
		{
			name:     "= .20, when actual is .10",
			operator: "=",
			desired:  ".20",
			actual:   .10,
			expected: false,
		},
		{
			name:     "= .20, when actual is .20",
			operator: "=",
			desired:  ".20",
			actual:   .20,
			expected: true,
		},
		{
			name:     "= 20%, when actual is .20",
			operator: "=",
			desired:  "20%",
			actual:   .20,
			expected: true,
		},
		{
			name:     "= 20%, when actual is 20",
			operator: "=",
			desired:  "20%",
			actual:   20,
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := doCompareHostDiskUsagePercent(test.operator, test.desired, test.actual)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestAnalyzeHostDiskUsage(t *testing.T) {
	tests := []struct {
		name          string
		diskUsageInfo *collect.DiskUsageInfo
		hostAnalyzer  *troubleshootv1beta2.DiskUsageAnalyze
		result        []*AnalyzeResult
		expectErr     bool
	}{
		{
			name: "Fail on insuffient total ephemeral disk space",
			diskUsageInfo: &collect.DiskUsageInfo{
				TotalBytes: 10 * 1024 * 1024 * 1024,
				UsedBytes:  5 * 1024 * 1024 * 1024,
			},
			hostAnalyzer: &troubleshootv1beta2.DiskUsageAnalyze{
				CollectorName: "ephemeral",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "used/total >= 80%",
							Message: "/var/lib/kubelet is more than 80% full",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "total <= 10Gi",
							Message: "/var/lib/kubelet requires at least 10Gi",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Disk Usage ephemeral",
					IsFail:  true,
					Message: "/var/lib/kubelet requires at least 10Gi",
				},
			},
		},
		{
			name: "Fail on insuffient available ephemeral disk space percentage",
			diskUsageInfo: &collect.DiskUsageInfo{
				TotalBytes: 10 * 1024 * 1024 * 1024,
				UsedBytes:  8 * 1024 * 1024 * 1024,
			},
			hostAnalyzer: &troubleshootv1beta2.DiskUsageAnalyze{
				CollectorName: "ephemeral",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "total < 10Gi",
							Message: "/var/lib/kubelet requires at least 10Gi",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "used/total >= 80%",
							Message: "/var/lib/kubelet is more than 80% full",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Disk Usage ephemeral",
					IsFail:  true,
					Message: "/var/lib/kubelet is more than 80% full",
				},
			},
		},
		{
			name: "Warn on high ephemeral disk space usage",
			diskUsageInfo: &collect.DiskUsageInfo{
				TotalBytes: 1024 * 1024 * 1024 * 1024,
				UsedBytes:  100 * 1024 * 1024 * 1024,
			},
			hostAnalyzer: &troubleshootv1beta2.DiskUsageAnalyze{
				CollectorName: "ephemeral",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "total < 10Gi",
							Message: "/var/lib/kubelet requires at least 10Gi",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "used/total >= 80%",
							Message: "/var/lib/kubelet is more than 80% full",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "used >= 100Gi",
							Message: "/var/lib/kubelet has more than 100Gi used",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Disk Usage ephemeral",
					IsWarn:  true,
					Message: "/var/lib/kubelet has more than 100Gi used",
				},
			},
		},
		{
			name: "Pass on ephemeral disk space available",
			diskUsageInfo: &collect.DiskUsageInfo{
				TotalBytes: 12 * 1024 * 1024 * 1024,
				UsedBytes:  1 * 1024 * 1024 * 1024,
			},
			hostAnalyzer: &troubleshootv1beta2.DiskUsageAnalyze{
				CollectorName: "ephemeral",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "available > 10Gi",
							Message: "/var/lib/kubelet has at least 10Gi available",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Disk Usage ephemeral",
					IsPass:  true,
					Message: "/var/lib/kubelet has at least 10Gi available",
				},
			},
		},
		{
			name: "Pass with empty When and warning",
			diskUsageInfo: &collect.DiskUsageInfo{
				TotalBytes: 9 * 1024 * 1024 * 1024,
				UsedBytes:  1 * 1024 * 1024 * 1024,
			},
			hostAnalyzer: &troubleshootv1beta2.DiskUsageAnalyze{
				CollectorName: "ephemeral",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "available < 10Gi",
							Message: "/var/lib/kubelet less than 10Gi available",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "available < 25Gi",
							Message: "/var/lib/kubelet less than 25Gi available",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "/var/lib/kubelet passed",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Disk Usage ephemeral",
					IsFail:  true,
					Message: "/var/lib/kubelet less than 10Gi available",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			b, err := json.Marshal(test.diskUsageInfo)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := (&AnalyzeHostDiskUsage{test.hostAnalyzer}).Analyze(getCollectedFileContents)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
