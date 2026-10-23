//go:build linux

package collect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHypervisorClockDeviceFailures(t *testing.T) {
	for _, tc := range []struct {
		name      string
		clockName string
		device    bool
		wantError bool
	}{
		{name: "no reference"},
		{name: "NIC clock is not a reference", clockName: "unverified NIC PTP"},
		{name: "unavailable KVM device", clockName: kvmPTPClockName, wantError: true},
		{name: "unavailable Hyper-V device", clockName: "hyperv", wantError: true},
		{name: "unavailable VMware device", clockName: "ptp_vmw", wantError: true},
		{name: "unreadable Hyper-V clock", clockName: "hyperv", device: true, wantError: true},
		{name: "unreadable VMware clock", clockName: "ptp_vmw", device: true, wantError: true},
		{name: "unreadable clock", clockName: kvmPTPClockName, device: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			sysDirectory := filepath.Join(root, "sys")
			deviceDirectory := filepath.Join(root, "dev")
			if tc.clockName != "" {
				require.NoError(t, os.MkdirAll(filepath.Join(sysDirectory, "ptp0"), 0755))
				require.NoError(t, os.WriteFile(filepath.Join(sysDirectory, "ptp0", "clock_name"), []byte(tc.clockName+"\n"), 0644))
			}
			if tc.device {
				require.NoError(t, os.MkdirAll(deviceDirectory, 0755))
				// A regular file cannot provide a POSIX dynamic clock.
				require.NoError(t, os.WriteFile(filepath.Join(deviceDirectory, "ptp0"), nil, 0644))
			}
			ref, err := collectHypervisorClockFrom(sysDirectory, deviceDirectory)
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Nil(t, ref)
		})
	}
}
