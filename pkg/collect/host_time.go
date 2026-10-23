package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type NTPStatus string

type TimeInfo struct {
	Timezone             string               `json:"timezone"`
	NTPSynchronized      bool                 `json:"ntp_synchronized"`
	NTPActive            bool                 `json:"ntp_active"`
	HypervisorClock      *HypervisorClockInfo `json:"hypervisor_clock,omitempty"`
	HypervisorClockError string               `json:"hypervisor_clock_error,omitempty"`
}

// HypervisorClockInfo records a comparison against an independently readable
// hypervisor wall clock. Ordinary network-card PTP clocks are not references.
type HypervisorClockInfo struct {
	Source                  string `json:"source"`
	OffsetNanoseconds       int64  `json:"offset_nanoseconds"`
	ReadDurationNanoseconds int64  `json:"read_duration_nanoseconds"`
}

const kvmPTPClockName = "KVM virtual PTP"
const maxHypervisorClockReadDuration = 100 * time.Millisecond

func isHypervisorClock(source string) bool {
	switch source {
	case kvmPTPClockName, "hyperv", "ptp_vmw":
		return true
	default:
		return false
	}
}

// ClockSynchronized retains the reported NTP flag, while also recognizing a
// clock measured within one second of the hypervisor without network access.
func (info TimeInfo) ClockSynchronized() bool {
	if info.NTPSynchronized {
		return true
	}
	ref := info.HypervisorClock
	if info.HypervisorClockError != "" || ref == nil || !isHypervisorClock(ref.Source) || ref.ReadDurationNanoseconds < 0 || ref.ReadDurationNanoseconds > int64(maxHypervisorClockReadDuration) {
		return false
	}
	// Include sampling uncertainty rather than passing a borderline measurement.
	limit := int64(time.Second) - (ref.ReadDurationNanoseconds+1)/2
	return ref.OffsetNanoseconds >= -limit && ref.OffsetNanoseconds <= limit
}

func observeHypervisorClock(source string, before, reference, after time.Time) (*HypervisorClockInfo, error) {
	return observeHypervisorClockSample(source, before, reference, after, after.Sub(before))
}

func observeHypervisorClockSample(source string, before, reference, after time.Time, elapsed time.Duration) (*HypervisorClockInfo, error) {
	// Strip monotonic readings to also detect wall-clock jumps during sampling.
	// Sub saturates for extreme dates, unlike UnixNano arithmetic which can wrap.
	wallElapsed := after.Round(0).Sub(before.Round(0))
	if elapsed < 0 || elapsed > maxHypervisorClockReadDuration || wallElapsed < 0 || wallElapsed > maxHypervisorClockReadDuration {
		return nil, fmt.Errorf("hypervisor clock read took too long or the system clock changed")
	}
	// Allow small sampling differences, but reject clock steps during the read.
	if difference := wallElapsed - elapsed; difference < -time.Millisecond || difference > time.Millisecond {
		return nil, fmt.Errorf("system clock changed during hypervisor clock read")
	}
	return &HypervisorClockInfo{Source: source, OffsetNanoseconds: int64(before.Add(wallElapsed / 2).Sub(reference)), ReadDurationNanoseconds: int64(wallElapsed)}, nil
}

const HostTimePath = `host-collectors/system/time.json`
const HostTimeFileName = `time.json`

type CollectHostTime struct {
	hostCollector *troubleshootv1beta2.HostTime
	BundlePath    string
}

func (c *CollectHostTime) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "TCP Port Status")
}

func (c *CollectHostTime) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostTime) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	timeInfo := TimeInfo{}

	conn, err := dbus.SystemBus()
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to dbus")
	}
	defer func() {
		if err := conn.Close(); err != nil {
			log.Printf("Failed to close dbus connection: %v\n", err)
		}
	}()

	prop := "org.freedesktop.timedate1.Timezone"
	variant, err := conn.Object("org.freedesktop.timedate1", "/org/freedesktop/timedate1").GetProperty(prop)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read property %s", prop)
	}
	timeInfo.Timezone = strings.Trim(variant.String(), `"`)
	// UTC is reported as Etc/UTC on Ubuntu
	if strings.ToLower(timeInfo.Timezone) == "etc/utc" {
		timeInfo.Timezone = "UTC"
	}

	timeInfo.Timezone = strings.ToUpper(timeInfo.Timezone)

	prop = "org.freedesktop.timedate1.NTPSynchronized"
	variant, err = conn.Object("org.freedesktop.timedate1", "/org/freedesktop/timedate1").GetProperty(prop)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read property %s", prop)
	}
	switch variant.String() {
	case "true":
		timeInfo.NTPSynchronized = true
	case "false":
		timeInfo.NTPSynchronized = false
	default:
		return nil, fmt.Errorf("Unexpected value for property %s: %s", prop, variant.String())
	}

	prop = "org.freedesktop.timedate1.NTP"
	variant, err = conn.Object("org.freedesktop.timedate1", "/org/freedesktop/timedate1").GetProperty(prop)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read property %s", prop)
	}
	switch variant.String() {
	case "true":
		timeInfo.NTPActive = true
	case "false":
		timeInfo.NTPActive = false
	default:
		return nil, fmt.Errorf("Unexpected value for property %s: %s", prop, variant.String())
	}

	timeInfo.HypervisorClock, err = collectHypervisorClock()
	if err != nil {
		timeInfo.HypervisorClockError = err.Error()
	}

	b, err := json.Marshal(timeInfo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal time info")
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, HostTimePath, bytes.NewBuffer(b))

	return map[string][]byte{
		HostTimePath: b,
	}, nil
}

func (c *CollectHostTime) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
