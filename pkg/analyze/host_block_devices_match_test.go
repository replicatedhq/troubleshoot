package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHostBlockDevices_deviceTypeEligibility documents type rules in isolation (see blockDevicesMatchConfig in host_block_devices.go).
func TestHostBlockDevices_deviceTypeEligibility(t *testing.T) {
	tests := []struct {
		name    string
		cfg     blockDevicesMatchConfig
		devType string
		want    bool
	}{
		{name: "disk always", cfg: blockDevicesMatchConfig{}, devType: "disk", want: true},
		{name: "part without flag", cfg: blockDevicesMatchConfig{}, devType: "part", want: false},
		{name: "part with flag", cfg: blockDevicesMatchConfig{includeUnmountedPartitions: true}, devType: "part", want: true},
		{name: "loop without additional", cfg: blockDevicesMatchConfig{}, devType: "loop", want: false},
		{name: "loop with additional", cfg: blockDevicesMatchConfig{additionalDeviceTypes: []string{"loop"}}, devType: "loop", want: true},
		{name: "lvm with additional", cfg: blockDevicesMatchConfig{additionalDeviceTypes: []string{"lvm"}}, devType: "lvm", want: true},
		{name: "crypt with additional", cfg: blockDevicesMatchConfig{additionalDeviceTypes: []string{"crypt"}}, devType: "crypt", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isEligibleDeviceType(tt.devType, tt.cfg)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestHostBlockDevices_additionalDeviceTypes covers end-to-end analyze behavior for AdditionalDeviceTypes and representative preflights.
func TestHostBlockDevices_additionalDeviceTypes(t *testing.T) {
	const rawStoragePass = "At least one raw block device is available for storage."
	const rawStorageFail = "No raw block devices found. At least one unformatted, unmounted disk is required for storage. Attach a raw disk and ensure it has no filesystem or mount point."

	rawStorageOutcomes := []*troubleshootv1beta2.Outcome{
		{Fail: &troubleshootv1beta2.SingleOutcome{When: ".* == 0", Message: rawStorageFail}},
		{Pass: &troubleshootv1beta2.SingleOutcome{When: ".* >= 1", Message: rawStoragePass}},
	}

	tests := []struct {
		name         string
		devices      []collect.BlockDeviceInfo
		hostAnalyzer *troubleshootv1beta2.BlockDevicesAnalyze
		want         []*AnalyzeResult
	}{
		{
			name: "preflight-style 10GiB loop0 with includeUnmountedPartitions + additionalDeviceTypes loop",
			devices: []collect.BlockDeviceInfo{{
				Name: "loop0", KernelName: "loop0", Type: "loop", Major: 7, Minor: 0,
				Size: 10737418240, ReadOnly: false, Removable: false,
			}},
			hostAnalyzer: &troubleshootv1beta2.BlockDevicesAnalyze{
				IncludeUnmountedPartitions: true,
				MinimumAcceptableSize:      10737418240,
				AdditionalDeviceTypes:      []string{"loop"},
				Outcomes:                   rawStorageOutcomes,
			},
			want: []*AnalyzeResult{{Title: "Block Devices", IsPass: true, Message: rawStoragePass}},
		},
		{
			name: "preflight-style 10GiB LVM with includeUnmountedPartitions + additionalDeviceTypes lvm",
			devices: []collect.BlockDeviceInfo{{
				Name: "ceph--vg-lv--osd0", KernelName: "dm-0", Type: "lvm", Major: 252, Minor: 0,
				Size: 10737418240, ReadOnly: false, Removable: false,
			}},
			hostAnalyzer: &troubleshootv1beta2.BlockDevicesAnalyze{
				IncludeUnmountedPartitions: true,
				MinimumAcceptableSize:      10737418240,
				AdditionalDeviceTypes:      []string{"lvm"},
				Outcomes:                   rawStorageOutcomes,
			},
			want: []*AnalyzeResult{{Title: "Block Devices", IsPass: true, Message: rawStoragePass}},
		},
		{
			name: "loop counts with only additionalDeviceTypes (includeUnmountedPartitions false)",
			devices: []collect.BlockDeviceInfo{{
				Name: "loop0", KernelName: "loop0", Type: "loop", Major: 7,
			}},
			hostAnalyzer: &troubleshootv1beta2.BlockDevicesAnalyze{
				AdditionalDeviceTypes: []string{"loop"},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{Pass: &troubleshootv1beta2.SingleOutcome{When: ".* > 0", Message: "Block device available"}},
					{Fail: &troubleshootv1beta2.SingleOutcome{Message: "No block device available"}},
				},
			},
			want: []*AnalyzeResult{{Title: "Block Devices", IsPass: true, Message: "Block device available"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := analyzeHostBlockDevicesOutput(t, tt.devices, tt.hostAnalyzer)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
