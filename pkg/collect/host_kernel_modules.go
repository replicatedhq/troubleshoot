package collect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

const (
	KernelModuleUnknown   = "unknown"
	KernelModuleLoaded    = "loaded"
	KernelModuleLoadable  = "loadable"
	KernelModuleLoading   = "loading"
	KernelModuleUnloading = "unloading"
)

type KernelModuleStatus string

type KernelModuleInfo struct {
	Size      uint64             `json:"size"`
	Instances string             `json:"instances"`
	UsedBy    []string           `json:"usedBy"`
	Status    KernelModuleStatus `json:"status"`
}

type CollectHostKernelModules struct {
	hostCollector *troubleshootv1beta2.HostKernelModules
}

func (c *CollectHostKernelModules) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Kernel Modules")
}

func (c *CollectHostKernelModules) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostKernelModules) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	modules, err := collectLoadable()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read loadable kernel modules")
	}
	loaded, err := collectLoaded()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read loaded kernel modules")
	}

	// Overlay with loaded modules.
	for name, module := range loaded {
		modules[name] = module
	}

	b, err := json.Marshal(modules)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal kernel modules")
	}

	return map[string][]byte{
		"system/kernel_modules.json": b,
	}, nil
}

func collectLoadable() (map[string]KernelModuleInfo, error) {
	modules := make(map[string]KernelModuleInfo)

	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine kernel release")
	}
	kernel := strings.TrimSpace(string(out))

	cmd := exec.Command("/usr/bin/find", "/lib/modules/"+kernel, "-type", "f", "-name", "*.ko*")
	stdout, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	buf := bytes.NewBuffer(stdout)
	scanner := bufio.NewScanner(buf)

	for scanner.Scan() {
		_, file := filepath.Split(scanner.Text())
		name := strings.TrimSuffix(file, filepath.Ext(file))

		if name == "" {
			continue
		}

		modules[name] = KernelModuleInfo{
			Status: KernelModuleLoadable,
		}
	}
	return modules, nil
}

func collectLoaded() (map[string]KernelModuleInfo, error) {
	modules := make(map[string]KernelModuleInfo)

	file, err := os.Open("/proc/modules")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		s := strings.Split(scanner.Text(), " ")
		name, size, instances, usedBy, state := s[0], s[1], s[2], s[3], s[4]

		sizeInt, err := strconv.Atoi(size)
		if err != nil {
			sizeInt = 0
		}

		var status KernelModuleStatus = KernelModuleUnknown
		switch state {
		case "Live":
			status = KernelModuleLoaded
		case "Loading":
			status = KernelModuleLoading
		case "Unloading":
			status = KernelModuleUnloading
		}

		modules[name] = KernelModuleInfo{
			Size:      uint64(sizeInt),
			Instances: instances,
			UsedBy:    strings.Split(usedBy, ","),
			Status:    status,
		}
	}
	return modules, nil
}
