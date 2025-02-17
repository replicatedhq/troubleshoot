package collect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
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
	Instances uint               `json:"instances"`
	Status    KernelModuleStatus `json:"status"`
}

const HostKernelModulesPath = `host-collectors/system/kernel_modules.json`

// kernelModuleCollector defines the interface used to collect modules from the
// underlying host.
type kernelModuleCollector interface {
	collect() (map[string]KernelModuleInfo, error)
}

// CollectHostKernelModules is responsible for collecting kernel module status
// from the host.
type CollectHostKernelModules struct {
	hostCollector *troubleshootv1beta2.HostKernelModules
	BundlePath    string
	loadable      kernelModuleCollector
	loaded        kernelModuleCollector
}

// Title is the name of the collector.
func (c *CollectHostKernelModules) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Kernel Modules")
}

// IsExcluded returns true if the collector has been excluded from the results.
func (c *CollectHostKernelModules) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

// Collect the kernel module status from the host.   Modules are returned as a
// map keyed on the module name used by the kernel, e.g:
//
//	{
//	  "system/kernel_modules.json": {
//	    ...
//	    "dm_snapshot": {
//	      "instances": 8,
//	      "size": 45056,
//	      "status": "loaded"
//	    },
//	    ...
//	  },
//	}
//
// Module status may be: loaded, loadable, loading, unloading or unknown.  When
// a module is loaded, it may have one or more instances.  The size represents
// the amount of memory (in bytes) that the module is using.
func (c *CollectHostKernelModules) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	modules, err := c.loadable.collect()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read loadable kernel modules")
	}
	if modules == nil {
		modules = map[string]KernelModuleInfo{}
	}
	loaded, err := c.loaded.collect()
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

	output := NewResult()
	output.SaveResult(c.BundlePath, HostKernelModulesPath, bytes.NewBuffer(b))

	return map[string][]byte{
		HostKernelModulesPath: b,
	}, nil
}

// kernelModulesLoadable retrieves the list of modules that can be loaded by
// the kernel.
type kernelModulesLoadable struct{}

// collect the list of modules that can be loaded by the kernel.
func (l kernelModulesLoadable) collect() (map[string]KernelModuleInfo, error) {
	modules := make(map[string]KernelModuleInfo)

	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine kernel release")
	}
	kernel := strings.TrimSpace(string(out))

	kernelPath := "/lib/modules/" + kernel

	if _, err := os.Stat(kernelPath); os.IsNotExist(err) {
		klog.V(2).Infof("modules are not loadable because kernel path %q does not exist, assuming we are in a container", kernelPath)
		return modules, nil
	}

	cmd := exec.Command("/usr/bin/find", kernelPath, "-type", "f", "-name", "*.ko*")
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

// kernelModulesLoaded retrieves the list of modules that the kernel is aware of.  The
// modules will either be in loaded, loading or unloading state.
type kernelModulesLoaded struct{}

// collect the list of modules that the kernel is aware of.
func (l kernelModulesLoaded) collect() (map[string]KernelModuleInfo, error) {
	modules, err := l.collectProc()
	if err != nil {
		return nil, err
	}

	builtin, err := l.collectBuiltin()
	if err != nil {
		return nil, err
	}

	for name, module := range builtin {
		if _, ok := modules[name]; !ok {
			modules[name] = module
		}
	}

	return modules, nil
}

func (l kernelModulesLoaded) collectProc() (map[string]KernelModuleInfo, error) {
	modules := make(map[string]KernelModuleInfo)

	file, err := os.Open("/proc/modules")
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		s := strings.Split(scanner.Text(), " ")
		name, size, instances, state := s[0], s[1], s[2], s[4]

		sizeInt, err := strconv.Atoi(size)
		if err != nil {
			sizeInt = 0
		}

		instancesInt, err := strconv.Atoi(instances)
		if err != nil {
			instancesInt = 0
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
			Instances: uint(instancesInt),
			Status:    status,
		}
	}
	return modules, nil
}

func (l kernelModulesLoaded) collectBuiltin() (map[string]KernelModuleInfo, error) {
	modules := make(map[string]KernelModuleInfo)

	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine kernel release")
	}
	kernel := strings.TrimSpace(string(out))

	file, err := os.Open(fmt.Sprintf("/usr/lib/modules/%s/modules.builtin", kernel))
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		_, file := filepath.Split(scanner.Text())
		name := strings.TrimSuffix(file, filepath.Ext(file))

		if name == "" {
			continue
		}

		modules[name] = KernelModuleInfo{
			Status: KernelModuleLoaded,
		}
	}
	return modules, nil
}

func (c *CollectHostKernelModules) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
