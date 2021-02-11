package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeBlockDevices(t *testing.T) {
	tests := []struct {
		name         string
		devices      []collect.BlockDeviceInfo
		hostAnalyzer *troubleshootv1beta2.BlockDevicesAnalyze
		result       *AnalyzeResult
		expectErr    bool
	}{
		{
			name: "sdb == 1, pass when there is an empty /dev/sdb",
			devices: []collect.BlockDeviceInfo{
				{
					Name:       "sdb",
					KernelName: "sdb",
					Type:       "disk",
					Major:      8,
					Serial:     "disk1",
				},
			},
			hostAnalyzer: &troubleshootv1beta2.BlockDevicesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "sdb == 1",
							Message: "Block device available",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Block Devices",
				IsPass:  true,
				Message: "Block device available",
			},
		},
		{
			name: "sdb == 1, fail when partitioned",
			devices: []collect.BlockDeviceInfo{
				{
					Name:       "sdb",
					KernelName: "sdb",
					Type:       "disk",
					Major:      8,
					Serial:     "disk1",
				},
				{
					Name:             "sdb1",
					KernelName:       "sdb1",
					ParentKernelName: "sdb",
					Type:             "part",
					Major:            8,
					Minor:            1,
				},
			},
			hostAnalyzer: &troubleshootv1beta2.BlockDevicesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "sdb == 1",
							Message: "Block device available",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "No block device available",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Block Devices",
				IsFail:  true,
				Message: "No block device available",
			},
		},
		{
			name: "sdb == 1, fail when it has a filesystem",
			devices: []collect.BlockDeviceInfo{
				{
					Name:           "sdb",
					KernelName:     "sdb",
					Type:           "disk",
					Major:          8,
					Serial:         "disk1",
					FilesystemType: "ext4",
				},
			},
			hostAnalyzer: &troubleshootv1beta2.BlockDevicesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "sdb == 1",
							Message: "Block device available",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "No block device available",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Block Devices",
				IsFail:  true,
				Message: "No block device available",
			},
		},
		{
			name: ".* > 0, fail when only loop devices are found",
			devices: []collect.BlockDeviceInfo{
				{
					Name:       "loop0",
					KernelName: "loop0",
					Type:       "loop",
					Major:      7,
				},
			},
			hostAnalyzer: &troubleshootv1beta2.BlockDevicesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    ".* > 0",
							Message: "Block device available",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "No block device available",
						},
					},
				},
			},
			result: &AnalyzeResult{
				Title:   "Block Devices",
				IsFail:  true,
				Message: "No block device available",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			b, err := json.Marshal(test.devices)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := analyzeHostBlockDevices(test.hostAnalyzer, getCollectedFileContents)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}
