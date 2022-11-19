package collect

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/blang/semver"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	apt "github.com/arduino/go-apt-client"
	osutils "github.com/shirou/gopsutil/host"
)

// HostInstalledPackage represents a given package (rpm, deb, etc) installed in the host. We only
// care about its name and version. notice that Version is only a string and it can or not comply
// with the semantic version schema.
type HostInstalledPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// InRange returns true if the package is within the provided semantic version range.
func (v *HostInstalledPackage) InRange(semverRange string) (bool, error) {
	version, err := semver.Make(v.Version)
	if err != nil {
		return false, fmt.Errorf("package %s does not use semver: %w", v.Name, err)
	}

	vrange, err := semver.ParseRange(semverRange)
	if err != nil {
		return false, fmt.Errorf("invalid semver range %s: %w", semverRange, err)
	}

	return vrange(version), nil
}

// MatchesRegex returns true if the installed package version matches the provided regex.
func (v *HostInstalledPackage) MatchesRegex(expression string) (bool, error) {
	re, err := regexp.Compile(expression)
	if err != nil {
		return false, fmt.Errorf("invalid regex %s: %w", expression, err)
	}
	return re.Match([]byte(v.Version)), nil
}

// HostInfo holds a list of all installed packages and the operating system
// information.
type HostInfo struct {
	OSInfo            *osutils.InfoStat      `json:"os_info"`
	InstalledPackages []HostInstalledPackage `json:"installed_packages"`
}

// PackageByName return a package by its name. returns nil if not found.
func (h *HostInfo) PackageByName(pkgname string) *HostInstalledPackage {
	for _, pkg := range h.InstalledPackages {
		if pkg.Name == pkgname {
			return &pkg
		}
	}
	return nil
}

type CollectInstalledPackages struct {
	collector  *troubleshootv1beta2.InstalledPackages
	BundlePath string
}

func (c *CollectInstalledPackages) Title() string {
	return hostCollectorTitleOrDefault(c.collector.HostCollectorMeta, "Host Packages")
}

func (c *CollectInstalledPackages) IsExcluded() (bool, error) {
	return isExcluded(c.collector.Exclude)
}

func (c *CollectInstalledPackages) Collect(progress chan<- interface{}) (map[string][]byte, error) {
	info, err := osutils.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get os info: %w", err)
	} else if info.OS != "linux" {
		return nil, fmt.Errorf("failed to get host packages: unknown os %s", info.OS)
	}

	var packages []HostInstalledPackage
	switch info.PlatformFamily {
	case "debian":
		packages, err = c.collectDebianPackages()
	case "redhat", "fedora":
		packages, err = c.collectRedHatPackages()
	default:
		return nil, fmt.Errorf("unknown platform family: %s", info.PlatformFamily)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to collect packages: %w", err)
	}

	result := HostInfo{OSInfo: info, InstalledPackages: packages}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal installed packages: %w", err)
	}

	fname := "hostPackages.json"
	if c.collector.CollectorName != "" {
		fname = fmt.Sprintf("%s.json", c.collector.CollectorName)
	}

	name := filepath.Join("host-collectors", "hostPackages", fname)
	output := NewResult()
	if err := output.SaveResult(c.BundlePath, name, bytes.NewBuffer(data)); err != nil {
		return nil, fmt.Errorf("failed to save result: %w", err)
	}

	return map[string][]byte{name: data}, nil
}

func (c *CollectInstalledPackages) collectRedHatPackages() ([]HostInstalledPackage, error) {
	cmdout := bytes.NewBuffer(nil)
	cmd := exec.Command("rpm", "-qa", "--queryformat", `%{NAME},%{VERSION}\n`)
	cmd.Stdout = cmdout
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to run rpm command: %w", err)
	}

	var installed []HostInstalledPackage
	reader := csv.NewReader(cmdout)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to read line from rpm output: %w", err)
		}

		if len(record) != 2 {
			return nil, fmt.Errorf("failed to process rpm output: %+v", record)
		}

		installed = append(installed, HostInstalledPackage{
			Name:    record[0],
			Version: record[1],
		})
	}
	return installed, nil
}

func (c *CollectInstalledPackages) collectDebianPackages() ([]HostInstalledPackage, error) {
	pkgs, err := apt.List()
	if err != nil {
		return nil, fmt.Errorf("failed to list installed packages: %w", err)
	}

	var installed []HostInstalledPackage
	for _, pkg := range pkgs {
		if pkg.Status != "installed" {
			continue
		}

		installed = append(installed, HostInstalledPackage{
			Name:    pkg.Name,
			Version: pkg.Version,
		})
	}
	return installed, nil
}
