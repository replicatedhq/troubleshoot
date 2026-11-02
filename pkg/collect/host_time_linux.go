//go:build linux

package collect

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

func collectHypervisorClock() (*HypervisorClockInfo, error) {
	return collectHypervisorClockFrom("/sys/class/ptp", "/dev")
}

func collectHypervisorClockFrom(sysDirectory, deviceDirectory string) (*HypervisorClockInfo, error) {
	paths, err := filepath.Glob(filepath.Join(sysDirectory, "ptp*", "clock_name"))
	if err != nil {
		return nil, fmt.Errorf("find hypervisor clock: %w", err)
	}
	var failures []error
	for _, path := range paths {
		name, err := os.ReadFile(path)
		if err != nil {
			failures = append(failures, fmt.Errorf("read PTP clock name: %w", err))
			continue
		}
		source := strings.TrimSpace(string(name))
		if !isHypervisorClock(source) {
			continue
		}
		device := filepath.Join(deviceDirectory, filepath.Base(filepath.Dir(path)))
		file, err := os.Open(device)
		if err != nil {
			failures = append(failures, fmt.Errorf("open hypervisor clock: %w", err))
			continue
		}
		before := time.Now()
		var timestamp unix.Timespec
		// Linux's FD_TO_CLOCKID converts a PTP device into a POSIX dynamic clock.
		clockID := (^int32(file.Fd()) << 3) | 3
		err = unix.ClockGettime(clockID, &timestamp)
		after := time.Now()
		closeErr := file.Close()
		if err != nil {
			failures = append(failures, fmt.Errorf("read hypervisor clock %s: %w", device, err))
		}
		if closeErr != nil {
			failures = append(failures, fmt.Errorf("close hypervisor clock %s: %w", device, closeErr))
		}
		if err != nil || closeErr != nil {
			continue
		}
		ref, err := observeHypervisorClock(source, before, time.Unix(int64(timestamp.Sec), int64(timestamp.Nsec)), after)
		if err != nil {
			failures = append(failures, fmt.Errorf("sample hypervisor clock %s: %w", device, err))
			continue
		}
		return ref, nil
	}
	return nil, errors.Join(failures...)
}
