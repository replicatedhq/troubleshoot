package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestWeaveReport(t *testing.T) {
	var tests = []struct {
		name   string
		report string
		expect []*AnalyzeResult
	}{
		{
			name: "sufficient IPs",
			expect: []*AnalyzeResult{
				{
					Title:   "Weave Report",
					IsPass:  true,
					Message: "No issues detected in weave report",
				},
			},
			report: `
{
	"Router": {
		"NickName": "areed-aka-k8ms"
	},
	"IPAM": {
		"RangeNumIPs": 4096,
		"ActiveIPs": 21,
		"PendingAllocates": null
	}
}`,
		},
		{
			name: "insufficient IPs",
			expect: []*AnalyzeResult{
				{
					Title:   "Available Pod IPs",
					IsWarn:  true,
					Message: "3900 of 4096 total available IPs have been assigned",
				},
			},
			report: `
{
	"Router": {
		"NickName": "areed-aka-k8ms"
	},
	"IPAM": {
		"RangeNumIPs": 4096,
		"ActiveIPs": 3900,
		"PendingAllocates": null
	}
}`,
		},
		{
			name: "IPs pending allocation",
			expect: []*AnalyzeResult{
				{
					Title:   "Pending IP Allocation",
					IsWarn:  true,
					Message: "Waiting for IPs to become available",
				},
			},
			report: `
{
	"Router": {
		"NickName": "areed-aka-k8ms"
	},
	"IPAM": {
		"RangeNumIPs": 4096,
		"ActiveIPs": 21,
		"PendingAllocates": ["10.32.1.15"]
	}
}`,
		},
		{
			name: "remote connections established",
			expect: []*AnalyzeResult{
				{
					Title:   "Weave Report",
					IsPass:  true,
					Message: "No issues detected in weave report",
				},
			},
			report: `
{
	"Router": {
		"NickName": "areed-aka-k8ms",
		"Connections": [
			{
				"State": "failed",
				"Info": "cannot connect to ourself, retry: never",
				"Attrs": {
					"name": "fastdp"
				}
			},
			{
				"State": "established",
				"Info": "encrypted   fastdp 1a:5b:a9:53:2b:11(areed-aka-kkz0)",
				"Attrs": {
					"name": "fastdp"
				}
			}
		]
	}
}`,
		},
		{
			name: "pending connection",
			expect: []*AnalyzeResult{
				{
					Title:   "Weave Inter-Node Connections",
					IsWarn:  true,
					Message: "Connection from areed-aka-k8ms to areed-aka-kkz0 is pending",
				},
			},
			report: `
{
	"Router": {
		"NickName": "areed-aka-k8ms",
		"Connections": [
			{
				"State": "pending",
				"Info": "encrypted   fastdp 1a:5b:a9:53:2b:11(areed-aka-kkz0)",
				"Attrs": {
					"name": "fastdp"
				}
			}
		]
	}
}`,
		},
		{
			name: "sleeve connection",
			expect: []*AnalyzeResult{
				{
					Title:   "Weave Inter-Node Connections",
					IsWarn:  true,
					Message: `Connection from areed-aka-k8ms to areed-aka-kkz0 protocol is "sleeve", not fastdp`,
				},
			},
			report: `
{
	"Router": {
		"NickName": "areed-aka-k8ms",
		"Connections": [
			{
				"State": "established",
				"Info": "encrypted   sleeve 1a:5b:a9:53:2b:11(areed-aka-kkz0)",
				"Attrs": {
                    "name": "sleeve"
                }
			}
		]
	}
}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var findFiles = func(glob string, _ []string) (map[string][]byte, error) {
				return map[string][]byte{
					"report1": []byte(test.report),
				}, nil
			}
			analyzer := &troubleshootv1beta2.WeaveReportAnalyze{}
			results, err := analyzeWeaveReport(analyzer, findFiles)

			assert.NoError(t, err)
			assert.Equal(t, test.expect, results)
		})
	}
}

func TestParseWeaveConnectionInfoHostname(t *testing.T) {
	info := "encrypted   fastdp 1a:5b:a9:53:2b:11(areed-aka-kkz0)"
	got := parseWeaveConnectionInfoHostname(info)

	expect := "areed-aka-kkz0"

	assert.Equal(t, expect, got)

}
