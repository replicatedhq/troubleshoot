package collect

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"periph.io/x/periph/host/distro"
)

type SystemPackageInfo struct {
	Name     string `json:"name"`
	Details  string `json:"details"`
	ExitCode string `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}

type CollectHostSystemPackages struct {
	hostCollector *troubleshootv1beta2.HostSystemPackages
}

func (c *CollectHostSystemPackages) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "System Packages")
}

func (c *CollectHostSystemPackages) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostSystemPackages) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	infos := []SystemPackageInfo{}

	var releaseID string

	osReleaseMap := distro.OSRelease()
	if id, ok := osReleaseMap["ID"]; ok {
		releaseID = id
	} else {
		b, err := ioutil.ReadFile("/etc/system-release")
		if err == nil && bytes.Contains(b, []byte("Amazon Linux")) {
			releaseID = "amzn"
		}
	}

	for _, p := range c.hostCollector.Packages {
		info := SystemPackageInfo{
			Name: p,
		}

		var cmd *exec.Cmd
		var stdout, stderr bytes.Buffer

		switch releaseID {
		case "ubuntu":
			cmd = exec.Command("dpkg", "-s", p)
		case "centos", "rhel", "amzn", "ol":
			cmd = exec.Command("yum", "list", "installed", p)
		default:
			return nil, errors.New("unsupported distribution")
		}

		err := cmd.Run()
		if err != nil {
			if werr, ok := err.(*exec.ExitError); ok {
				info.ExitCode = strings.TrimPrefix(werr.Error(), "exit status ")
			} else {
				return nil, errors.Wrap(err, "failed to run")
			}
		} else {
			info.ExitCode = "0"
		}

		info.Details = stdout.String()
		info.Error = stderr.String()

		infos = append(infos, info)
	}

	b, err := json.Marshal(infos)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal block device info")
	}

	return map[string][]byte{
		"system/packages.json": b,
	}, nil
}
