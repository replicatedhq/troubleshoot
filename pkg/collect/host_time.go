package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/godbus/dbus/v5"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type NTPStatus string

type TimeInfo struct {
	Timezone        string `json:"timezone"`
	NTPSynchronized bool   `json:"ntp_synchronized"`
	NTPActive       bool   `json:"ntp_active"`
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

func (c *CollectHostTime) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
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
