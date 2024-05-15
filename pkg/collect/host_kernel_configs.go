package collect

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

type CollectHostKernelConfigs struct {
	hostCollector *troubleshootv1beta2.HostKernelConfigs
	BundlePath    string
}

type kConfigs map[string]kConfigOption
type kConfigOption string

const HostKernelConfigsPath = `host-collectors/system/kernel-configs.json`

const (
	kConfigUnknown  kConfigOption = ""
	kConfigBuiltIn  kConfigOption = "y"
	kConfigAsModule kConfigOption = "m"
	kConfigLeftOut  kConfigOption = "n"
)

func (c *CollectHostKernelConfigs) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "kernel-configs")
}

func (c *CollectHostKernelConfigs) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostKernelConfigs) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {

	kernelRelease, err := getKernelRelease()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get kernel release")
	}

	var kConfigs kConfigs
	kConfigs, err = loadKConfigs(kernelRelease)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load kernel configs")
	}

	b, err := json.Marshal(kConfigs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal kernel configs")
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, HostKernelConfigsPath, bytes.NewBuffer(b))

	return output, nil
}

func getKernelRelease() (string, error) {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to determine kernel release")
	}
	release := strings.TrimSpace(string(out))
	return release, nil
}

// https://github.com/k0sproject/k0s/blob/ddee3f980443e19620e678a6e1dc136ff053bff9/internal/pkg/sysinfo/probes/linux/kernel.go#L282
// loadKConfigs checks a list of well-known file system paths for kernel
// configuration files and tries to parse them.
func loadKConfigs(kernelRelease string) (kConfigs, error) {
	// At least some references to those paths may be fond here:
	// https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L794
	// https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L9
	possiblePaths := []string{
		"/proc/config.gz",
		"/boot/config-" + kernelRelease,
		"/usr/src/linux-" + kernelRelease + "/.config",
		"/usr/src/linux/.config",
		"/usr/lib/modules/" + kernelRelease + "/config",
		"/usr/lib/ostree-boot/config-" + kernelRelease,
		"/usr/lib/kernel/config-" + kernelRelease,
		"/usr/src/linux-headers-" + kernelRelease + "/.config",
		"/lib/modules/" + kernelRelease + "/build/.config",
	}

	for _, path := range possiblePaths {
		// open file for reading
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		defer f.Close()

		r := io.Reader(bufio.NewReader(f))

		// This is a gzip file (config.gz), unzip it.
		if filepath.Ext(path) == ".gz" {
			gr, err := gzip.NewReader(r)
			if err != nil {
				return nil, err
			}
			defer gr.Close()
			r = gr
		}

		return parseKConfigs(r)
	}

	return nil, errors.Errorf("no kernel config files found for kernel release %q", kernelRelease)
}

// parseKConfigs parses `r` line by line, extracting all kernel config options.
func parseKConfigs(r io.Reader) (kConfigs, error) {
	configs := kConfigs{}
	kConfigLineRegex := regexp.MustCompile(fmt.Sprintf(
		"^(CONFIG_[A-Z0-9_]+)=([%s%s%s])$",
		string(kConfigBuiltIn), string(kConfigLeftOut), string(kConfigAsModule),
	))
	s := bufio.NewScanner(r)
	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}

		if matches := kConfigLineRegex.FindStringSubmatch(s.Text()); matches != nil {
			configs[matches[1]] = kConfigOption(matches[2])
		}
	}
	return configs, nil
}
