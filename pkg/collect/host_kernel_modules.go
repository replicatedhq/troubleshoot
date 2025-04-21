package collect

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
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
	collect(kernelRelease string) (map[string]KernelModuleInfo, error)
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

func (c *CollectHostKernelModules) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
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
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine kernel release")
	}
	kernelRelease := strings.TrimSpace(string(out))

	modules, err := c.loadable.collect(kernelRelease)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read loadable kernel modules")
	}
	if modules == nil {
		modules = map[string]KernelModuleInfo{}
	}
	loaded, err := c.loaded.collect(kernelRelease)
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
func (l kernelModulesLoadable) collect(kernelRelease string) (map[string]KernelModuleInfo, error) {
	modules := make(map[string]KernelModuleInfo)

	kernelPath := filepath.Join("/lib/modules", kernelRelease)
	if _, err := os.Stat(kernelPath); os.IsNotExist(err) {
		kernelPath = filepath.Join("/usr/lib/modules", kernelRelease)
		if _, err := os.Stat(kernelPath); os.IsNotExist(err) {
			kernelPath = filepath.Join("/lib/modules", kernelRelease)
			klog.V(2).Infof("kernel modules are not loadable because path %q does not exist, assuming we are in a container", kernelPath)
			return modules, nil
		}
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
type kernelModulesLoaded struct {
	fs fs.FS
}

// collect the list of modules that the kernel is aware of.
func (l kernelModulesLoaded) collect(kernelRelease string) (map[string]KernelModuleInfo, error) {
	modules, err := l.collectProc()
	if err != nil {
		return nil, fmt.Errorf("proc: %w", err)
	}

	builtin, err := l.collectBuiltin(kernelRelease)
	if err != nil {
		return nil, fmt.Errorf("builtin: %w", err)
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

	file, err := l.fs.Open("proc/modules")
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

func (l kernelModulesLoaded) collectBuiltin(kernelRelease string) (map[string]KernelModuleInfo, error) {
	builtinPath := filepath.Join("lib/modules", kernelRelease, "modules.builtin")
	if _, err := fs.Stat(l.fs, builtinPath); os.IsNotExist(err) {
		builtinPath = filepath.Join("usr/lib/modules", kernelRelease, "modules.builtin")
		if _, err := fs.Stat(l.fs, builtinPath); os.IsNotExist(err) {
			builtinPath = filepath.Join("lib/modules", kernelRelease, "modules.builtin")
			klog.V(2).Infof("kernel builtin modules path %q does not exist, assuming we are in a container", builtinPath)
			return nil, nil
		}
	}

	file, err := l.fs.Open(builtinPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	return l.parseBuiltin(file)
}

func (l kernelModulesLoaded) parseBuiltin(r io.Reader) (map[string]KernelModuleInfo, error) {
	modules := make(map[string]KernelModuleInfo)

	scanner := bufio.NewScanner(r)
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
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return modules, nil
}

func (c *CollectHostKernelModules) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
