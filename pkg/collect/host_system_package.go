package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"periph.io/x/host/v3/distro"
)

type SystemPackagesInfo struct {
	OS        string          `json:"os"`
	OSVersion string          `json:"osVersion"`
	Packages  []SystemPackage `json:"packages"`
}

type SystemPackage struct {
	Name     string `json:"name"`
	Details  string `json:"details"`
	ExitCode string `json:"exitCode"`
	Error    string `json:"error,omitempty"`
}

type CollectHostSystemPackages struct {
	hostCollector *troubleshootv1beta2.HostSystemPackages
	BundlePath    string
}

func (c *CollectHostSystemPackages) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "System Packages")
}

func (c *CollectHostSystemPackages) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostSystemPackages) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
}

func (c *CollectHostSystemPackages) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	info := SystemPackagesInfo{}

	osReleaseMap := distro.OSRelease()
	info.OS = osReleaseMap["ID"]
	info.OSVersion = osReleaseMap["VERSION_ID"]

	if info.OS == "" {
		// special case for Amazon 2014.03
		b, err := ioutil.ReadFile("/etc/system-release")
		if err == nil && bytes.Contains(b, []byte("Amazon Linux")) {
			info.OS = "amzn"
			v, err := exec.Command("awk", "/Amazon Linux/{print $NF}", "/etc/system-release").Output()
			if err == nil {
				info.OSVersion = string(v)
			}
		}
	}

	if info.OS == "" {
		return nil, errors.New("distribution id could not be detected or is unsupported.")
	}
	if info.OSVersion == "" {
		return nil, errors.New("distribution version could not be detected or is unsupported.")
	}

	packages := []string{}

	switch info.OS {
	case "ubuntu":
		if len(c.hostCollector.Ubuntu) > 0 {
			packages = append(packages, c.hostCollector.Ubuntu...)
		}
		if len(c.hostCollector.Ubuntu16) > 0 && matchMajorVersion(info.OSVersion, "16") {
			packages = append(packages, c.hostCollector.Ubuntu16...)
		}
		if len(c.hostCollector.Ubuntu18) > 0 && matchMajorVersion(info.OSVersion, "18") {
			packages = append(packages, c.hostCollector.Ubuntu18...)
		}
		if len(c.hostCollector.Ubuntu20) > 0 && matchMajorVersion(info.OSVersion, "20") {
			packages = append(packages, c.hostCollector.Ubuntu20...)
		}
	case "centos":
		if len(c.hostCollector.CentOS) > 0 {
			packages = append(packages, c.hostCollector.CentOS...)
		}
		if len(c.hostCollector.CentOS7) > 0 && matchMajorVersion(info.OSVersion, "7") {
			packages = append(packages, c.hostCollector.CentOS7...)
		}
		if len(c.hostCollector.CentOS8) > 0 && matchMajorVersion(info.OSVersion, "8") {
			packages = append(packages, c.hostCollector.CentOS8...)
		}
		if len(c.hostCollector.CentOS9) > 0 && matchMajorVersion(info.OSVersion, "9") {
			packages = append(packages, c.hostCollector.CentOS8...)
		}
	case "rhel":
		if len(c.hostCollector.RHEL) > 0 {
			packages = append(packages, c.hostCollector.RHEL...)
		}
		if len(c.hostCollector.RHEL7) > 0 && matchMajorVersion(info.OSVersion, "7") {
			packages = append(packages, c.hostCollector.RHEL7...)
		}
		if len(c.hostCollector.RHEL8) > 0 && matchMajorVersion(info.OSVersion, "8") {
			packages = append(packages, c.hostCollector.RHEL8...)
		}
		if len(c.hostCollector.RHEL9) > 0 && matchMajorVersion(info.OSVersion, "9") {
			packages = append(packages, c.hostCollector.RHEL9...)
		}
	case "ol":
		if len(c.hostCollector.OracleLinux) > 0 {
			packages = append(packages, c.hostCollector.OracleLinux...)
		}
		if len(c.hostCollector.OracleLinux7) > 0 && matchMajorVersion(info.OSVersion, "7") {
			packages = append(packages, c.hostCollector.OracleLinux7...)
		}
		if len(c.hostCollector.OracleLinux8) > 0 && matchMajorVersion(info.OSVersion, "8") {
			packages = append(packages, c.hostCollector.OracleLinux8...)
		}
		if len(c.hostCollector.OracleLinux9) > 0 && matchMajorVersion(info.OSVersion, "9") {
			packages = append(packages, c.hostCollector.OracleLinux9...)
		}
	case "rocky":
		if len(c.hostCollector.RockyLinux) > 0 {
			packages = append(packages, c.hostCollector.RockyLinux...)
		}
		if len(c.hostCollector.RockyLinux8) > 0 && matchMajorVersion(info.OSVersion, "8") {
			packages = append(packages, c.hostCollector.RockyLinux8...)
		}
		if len(c.hostCollector.RockyLinux9) > 0 && matchMajorVersion(info.OSVersion, "9") {
			packages = append(packages, c.hostCollector.RockyLinux9...)
		}
	case "amzn":
		if len(c.hostCollector.AmazonLinux) > 0 {
			packages = append(packages, c.hostCollector.AmazonLinux...)
		}
		if len(c.hostCollector.AmazonLinux2) > 0 && matchMajorVersion(info.OSVersion, "2") {
			packages = append(packages, c.hostCollector.AmazonLinux2...)
		}
	default:
		return nil, errors.Errorf("unsupported distribution: %s", info.OS)
	}

	for _, p := range packages {
		sysPkg := SystemPackage{
			Name: p,
		}

		var cmd *exec.Cmd
		switch info.OS {
		case "ubuntu":
			cmd = exec.Command("dpkg", "-s", p)
		case "centos", "rhel", "amzn", "ol", "rocky":
			cmd = exec.Command("rpm", "-qi", p)
		default:
			return nil, errors.Errorf("unsupported distribution: %s", info.OS)
		}

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		err := cmd.Run()
		if err != nil {
			if werr, ok := err.(*exec.ExitError); ok {
				sysPkg.ExitCode = strings.TrimPrefix(werr.Error(), "exit status ")
			} else {
				return nil, errors.Wrap(err, "failed to run")
			}
		} else {
			sysPkg.ExitCode = "0"
		}

		sysPkg.Details = stdout.String()
		sysPkg.Error = stderr.String()

		info.Packages = append(info.Packages, sysPkg)
	}

	b, err := json.Marshal(info)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal system packages info")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "packages"
	}
	name := filepath.Join("host-collectors/system", collectorName+"-packages.json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return map[string][]byte{
		name: b,
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

func (c *CollectHostSystemPackages) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
