//go:build !linux

package collect

func collectHypervisorClock() (*HypervisorClockInfo, error) {
	return nil, nil
}
