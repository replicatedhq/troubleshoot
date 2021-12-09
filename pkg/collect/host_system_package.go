package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
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

	osReleaseMap := distro.OSRelease()
	detectedDistroID := osReleaseMap["ID"]
	detectedDistroVersion := osReleaseMap["VERSION_ID"]

	if detectedDistroID == "" {
		// special case for Amazon 2014.03
		b, err := ioutil.ReadFile("/etc/system-release")
		if err == nil && bytes.Contains(b, []byte("Amazon Linux")) {
			detectedDistroID = "amzn"
			v, err := exec.Command("awk", "/Amazon Linux/{print $NF}", "/etc/system-release").Output()
			if err == nil {
				detectedDistroVersion = string(v)
			}
		}
	}

	if detectedDistroID == "" {
		return nil, errors.New("distribution id could not be detected or is unsupported.")
	}
	if detectedDistroVersion == "" {
		return nil, errors.New("distribution version could not be detected or is unsupported.")
	}

	packages := []string{}

	switch detectedDistroID {
	case "ubuntu":
		if len(c.hostCollector.Ubuntu) > 0 {
			packages = append(packages, c.hostCollector.Ubuntu...)
		}
		if len(c.hostCollector.Ubuntu16) > 0 && matchMajorVersion(detectedDistroVersion, "16") {
			packages = append(packages, c.hostCollector.Ubuntu16...)
		}
		if len(c.hostCollector.Ubuntu18) > 0 && matchMajorVersion(detectedDistroVersion, "18") {
			packages = append(packages, c.hostCollector.Ubuntu18...)
		}
		if len(c.hostCollector.Ubuntu20) > 0 && matchMajorVersion(detectedDistroVersion, "20") {
			packages = append(packages, c.hostCollector.Ubuntu20...)
		}
	case "centos":
		if len(c.hostCollector.CentOS) > 0 {
			packages = append(packages, c.hostCollector.CentOS...)
		}
		if len(c.hostCollector.CentOS7) > 0 && matchMajorVersion(detectedDistroVersion, "7") {
			packages = append(packages, c.hostCollector.CentOS7...)
		}
		if len(c.hostCollector.CentOS8) > 0 && matchMajorVersion(detectedDistroVersion, "8") {
			packages = append(packages, c.hostCollector.CentOS8...)
		}
	case "rhel":
		if len(c.hostCollector.RHEL) > 0 {
			packages = append(packages, c.hostCollector.RHEL...)
		}
		if len(c.hostCollector.RHEL7) > 0 && matchMajorVersion(detectedDistroVersion, "7") {
			packages = append(packages, c.hostCollector.RHEL7...)
		}
		if len(c.hostCollector.RHEL8) > 0 && matchMajorVersion(detectedDistroVersion, "8") {
			packages = append(packages, c.hostCollector.RHEL8...)
		}
	case "ol":
		if len(c.hostCollector.OracleLinux) > 0 {
			packages = append(packages, c.hostCollector.OracleLinux...)
		}
		if len(c.hostCollector.OracleLinux7) > 0 && matchMajorVersion(detectedDistroVersion, "7") {
			packages = append(packages, c.hostCollector.OracleLinux7...)
		}
		if len(c.hostCollector.OracleLinux8) > 0 && matchMajorVersion(detectedDistroVersion, "8") {
			packages = append(packages, c.hostCollector.OracleLinux8...)
		}
	case "amzn":
		if len(c.hostCollector.AmazonLinux) > 0 {
			packages = append(packages, c.hostCollector.AmazonLinux...)
		}
		if len(c.hostCollector.AmazonLinux2) > 0 && matchMajorVersion(detectedDistroVersion, "2") {
			packages = append(packages, c.hostCollector.AmazonLinux2...)
		}
	default:
		return nil, errors.Errorf("unsupported distribution: %s", detectedDistroID)
	}

	for _, p := range packages {
		info := SystemPackageInfo{
			Name: p,
		}

		var cmd *exec.Cmd
		var stdout, stderr bytes.Buffer

		switch detectedDistroID {
		case "ubuntu":
			cmd = exec.Command("dpkg", "-s", p)
		case "centos", "rhel", "amzn", "ol":
			cmd = exec.Command("yum", "list", "installed", p)
		default:
			return nil, errors.Errorf("unsupported distribution: %s", detectedDistroID)
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
		return nil, errors.Wrap(err, "failed to marshal system packages info")
	}

	outputFileName := "system/packages.json"
	if c.hostCollector.CollectorName != "" {
		outputFileName = fmt.Sprintf("system/%s-packages.json", c.hostCollector.CollectorName)
	}

	return map[string][]byte{
		outputFileName: b,
	}, nil
}

func matchMajorVersion(version string, major string) bool {
	if version == major {
		return true
	}
	if strings.HasPrefix(version, fmt.Sprintf("%s.", major)) {
		return true
	}
	return false
}
