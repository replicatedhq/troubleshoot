package collect

import (
	"bytes"
	"encoding/json"

	"github.com/lorenzosaino/go-sysctl"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// Ensure `CollectHostSysctl` implements `HostCollector` interface at compile time.
var _ HostCollector = (*CollectHostSysctl)(nil)

// Path to the kernel virtual files, defaults to /proc/sys
var sysctlVirtualFiles = sysctl.DefaultPath

const HostSysctlPath = `host-collectors/system/sysctl.json`

type CollectHostSysctl struct {
	hostCollector *troubleshootv1beta2.HostSysctl
	BundlePath    string
}

func (c *CollectHostSysctl) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Sysctl")
}

func (c *CollectHostSysctl) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostSysctl) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	client, err := sysctl.NewClient(sysctlVirtualFiles)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialize sysctl client")
	}

	values, err := client.GetAll()
	if err != nil {
		return nil, errors.Wrap(err, "failed to run sysctl client")
	}

	payload, err := json.Marshal(values)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal data to json")
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, HostSysctlPath, bytes.NewBuffer(payload))
	return output, nil
}
