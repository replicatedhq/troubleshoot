package collect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type ServiceInfo struct {
	Unit   string `json:"Unit"`
	Load   string `json:"Load"`
	Active string `json:"Active"`
	Sub    string `json:"Sub"`
}

const systemctlFormat = `%s %s %s %s` // this leaves off the description
const HostServicesPath = `host-collectors/system/systemctl_services.json`
const HostServicesFileName = `systemctl_services.json`

type CollectHostServices struct {
	hostCollector *troubleshootv1beta2.HostServices
	BundlePath    string
}

func (c *CollectHostServices) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Block Devices")
}

func (c *CollectHostServices) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostServices) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostServices) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	var devices []ServiceInfo

	cmd := exec.Command("systemctl", "list-units", "--type=service", "--no-legend", "--all")
	stdout, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to execute systemctl")
	}
	buf := bytes.NewBuffer(stdout)
	scanner := bufio.NewScanner(buf)

	for scanner.Scan() {
		bdi := ServiceInfo{}
		fmt.Sscanf(
			scanner.Text(),
			systemctlFormat,
			&bdi.Unit,
			&bdi.Load,
			&bdi.Active,
			&bdi.Sub,
		)

		devices = append(devices, bdi)
	}

	b, err := json.Marshal(devices)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal systemctl service info")
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, HostServicesPath, bytes.NewBuffer(b))

	return map[string][]byte{
		HostServicesPath: b,
	}, nil
}

func (c *CollectHostServices) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
