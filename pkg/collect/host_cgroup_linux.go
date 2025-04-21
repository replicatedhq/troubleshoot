// This Control Groups collector is heavily based on k0s'
// probes implementation https://github.com/k0sproject/k0s/blob/main/internal/pkg/sysinfo/probes/linux/cgroups.go

//go:build linux

package collect

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/cilium/ebpf/rlimit"
	"github.com/containerd/cgroups/v3/cgroup2"
	"github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"

	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

func discoverConfiguration(mountPoint string) (cgroupsResult, error) {
	results := cgroupsResult{}

	var st syscall.Statfs_t
	if err := syscall.Statfs(mountPoint, &st); err != nil {
		if os.IsNotExist(err) {
			klog.V(2).Infof("no file system mounted at %q", mountPoint)
			return results, nil
		}

		return results, fmt.Errorf("failed to stat %q: %w", mountPoint, err)
	}

	switch st.Type {
	case unix.CGROUP2_SUPER_MAGIC:
		klog.V(2).Infof("cgroup v2 mounted at %q", mountPoint)
		// Discover cgroup2 and controllers enabled
		// https://www.kernel.org/doc/html/v5.16/admin-guide/cgroup-v2.html#mounting
		v, err := discoverV2Configuration(mountPoint)
		if err != nil {
			return results, fmt.Errorf("failed to discover cgroup v2 configuration from %s mount point: %w", mountPoint, err)
		}
		results.CGroupV2 = v
	case unix.CGROUP_SUPER_MAGIC, unix.TMPFS_MAGIC:
		klog.V(2).Infof("cgroup v1 mounted at %q", mountPoint)
		// Discover cgroup1 and controllers enabled
		// https://git.kernel.org/pub/scm/docs/man-pages/man-pages.git/tree/man7/cgroups.7?h=man-pages-5.13#n159
		// https://www.kernel.org/doc/html/v5.16/admin-guide/cgroup-v1/cgroups.html#how-do-i-use-cgroups
		r, err := discoverV1Configuration(mountPoint)
		if err != nil {
			return results, fmt.Errorf("failed to discover cgroup v1 configuration from %s mount point: %w", mountPoint, err)
		}
		results.CGroupV1 = r
	default:
		return results, fmt.Errorf("unexpected file system type of %q: 0x%x", mountPoint, st.Type)
	}

	// If cgroup1 or cgroup2 is enabled
	results.CGroupEnabled = results.CGroupV1.Enabled || results.CGroupV2.Enabled

	// Sort controllers for consistent output
	if len(results.CGroupV1.Controllers) > 0 {
		sort.Strings(results.CGroupV1.Controllers)
	} else {
		results.CGroupV1.Controllers = []string{}
	}
	if len(results.CGroupV2.Controllers) > 0 {
		sort.Strings(results.CGroupV2.Controllers)
	} else {
		results.CGroupV2.Controllers = []string{}
	}

	// Combine all controllers
	set := make(map[string]struct{})
	for _, c := range results.CGroupV1.Controllers {
		set[c] = struct{}{}
	}

	for _, c := range results.CGroupV2.Controllers {
		set[c] = struct{}{}
	}

	for c := range set {
		results.AllControllers = append(results.AllControllers, c)
	}
	sort.Strings(results.AllControllers)

	return results, nil
}

func discoverV1Configuration(mountPoint string) (cgroupResult, error) {
	res := cgroupResult{}
	// Get the available controllers from /proc/cgroups.
	// See https://www.man7.org/linux/man-pages/man7/cgroups.7.html#NOTES

	f, err := os.Open("/proc/cgroups")
	if err != nil {
		return res, fmt.Errorf("failed to open /proc/cgroups: %w", err)
	}
	defer f.Close()

	names, err := parseV1ControllerNames(f)
	if err != nil {
		return res, err
	}

	res.Enabled = true
	res.Controllers = names
	res.MountPoint = mountPoint

	return res, nil
}

func discoverV2Configuration(mountPoint string) (cgroupResult, error) {
	res := cgroupResult{}

	// Detect all the listed root controllers.
	controllers, err := detectV2Controllers(mountPoint)
	if err != nil {
		return res, err
	}

	res.Enabled = true
	res.Controllers = controllers
	res.MountPoint = mountPoint
	return res, nil
}

// Detects all the listed root controllers.
//
// https://github.com/torvalds/linux/blob/v5.3/Documentation/admin-guide/cgroup-v2.rst#core-interface-files
func detectV2Controllers(mountPoint string) ([]string, error) {
	root, err := cgroup2.Load("/", cgroup2.WithMountpoint(mountPoint))
	if err != nil {
		return nil, fmt.Errorf("failed to load root cgroup: %w", err)
	}

	// Load root controllers
	controllerNames, err := root.RootControllers() // This reads cgroup.controllers
	if err != nil {
		return nil, fmt.Errorf("failed to list cgroup root controllers: %w", err)
	}

	for _, c := range controllerNames {
		if c == "cpu" {
			// If the cpu controller is enabled, the cpuacct controller is also enabled.
			// This controller succeeded v1's cpuacct and cpu controllers.
			// https://www.man7.org/linux/man-pages/man7/cgroups.7.html
			controllerNames = append(controllerNames, "cpuacct")
		}
	}

	// Detect freezer controller
	if detectV2FreezerController(mountPoint) {
		controllerNames = append(controllerNames, "freezer")
	}

	// Detect devices controller
	if detectV2DevicesController(mountPoint) {
		controllerNames = append(controllerNames, "devices")
	}

	return controllerNames, nil
}

// Detects the device controller by trying to attach a dummy program of type
// BPF_CGROUP_DEVICE to a cgroup. Since the controller has no interface files
// and is implemented purely on top of BPF, this is the only reliable way to
// detect it. A best-guess detection via the kernel version has the major
// drawback of not working with kernels that have a lot of backported features,
// such as RHEL and friends.
//
// https://github.com/torvalds/linux/blob/v5.3/Documentation/admin-guide/cgroup-v2.rst#device-controller
func detectV2DevicesController(mountPoint string) bool {
	err := attachDummyDeviceFilter(mountPoint)
	switch {
	case err == nil:
		klog.V(2).Info("eBPF device filter program successfully attached")
		return true
	// EACCES occurs when not allowed to create cgroups.
	// EPERM occurs when not allowed to load eBPF programs.
	case errors.Is(err, os.ErrPermission) && os.Geteuid() != 0:
		// Insufficient permissions. Loading the eBPF program requires elevated permissions
		return true
	case errors.Is(err, unix.EROFS):
		// Read-only file system detected when trying to create a temporary cgroup
		return true
	case eBPFProgramUnsupported(err):
		klog.V(2).Info("eBPF device filter program is unsupported by the kernel")
		return false
	}

	klog.V(2).Infof("failed to attach eBPF device filter program: %v", err)
	return false
}

// Attaches a dummy program of type BPF_CGROUP_DEVICE to a randomly created
// cgroup and removes the program and cgroup again.
func attachDummyDeviceFilter(mountPoint string) (err error) {
	insts, license, err := cgroup2.DeviceFilter([]specs.LinuxDeviceCgroup{{
		Allow:  true,
		Type:   "a",
		Major:  ptr.To(int64(-1)),
		Minor:  ptr.To(int64(-1)),
		Access: "rwm",
	}})
	if err != nil {
		return fmt.Errorf("failed to create eBPF device filter program: %w", err)
	}

	tmpCgroupPath, err := os.MkdirTemp(mountPoint, "troubleshoot-devices-detection-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary cgroup: %w", err)
	}
	defer func() { err = errors.Join(err, os.Remove(tmpCgroupPath)) }()

	dirFD, err := unix.Open(tmpCgroupPath, unix.O_DIRECTORY|unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return fmt.Errorf("failed to open temporary cgroup: %w", &fs.PathError{Op: "open", Path: tmpCgroupPath, Err: err})
	}
	defer func() {
		if closeErr := unix.Close(dirFD); closeErr != nil {
			err = errors.Join(err, &fs.PathError{Op: "close", Path: tmpCgroupPath, Err: closeErr})
		}
	}()

	close, err := cgroup2.LoadAttachCgroupDeviceFilter(insts, license, dirFD)
	if err != nil {
		// RemoveMemlock may be required on kernels < 5.11
		// observed on debian 11: 5.10.0-21-armmp-lpae #1 SMP Debian 5.10.162-1 (2023-01-21) armv7l
		// https://github.com/cilium/ebpf/blob/v0.11.0/prog.go#L356-L360
		if errors.Is(err, unix.EPERM) && strings.Contains(err.Error(), "RemoveMemlock") {
			if err2 := rlimit.RemoveMemlock(); err2 != nil {
				err = errors.Join(err, err2)
			} else {
				// Try again, MEMLOCK should be removed by now.
				close, err2 = cgroup2.LoadAttachCgroupDeviceFilter(insts, license, dirFD)
				if err2 != nil {
					err = errors.Join(err, err2)
				} else {
					err = nil
				}
			}
		}
	}
	if err != nil {
		if eBPFProgramUnsupported(err) {
			return err
		}
		return fmt.Errorf("failed to load/attach eBPF device filter program: %w", err)
	}

	return close()
}

// Returns true if the given error indicates that an eBPF program is unsupported
// by the kernel.
func eBPFProgramUnsupported(err error) bool {
	// https://github.com/cilium/ebpf/blob/v0.11.0/features/prog.go#L43-L49

	switch {
	// EINVAL occurs when attempting to create a program with an unknown type.
	case errors.Is(err, unix.EINVAL):
		return true

	// E2BIG occurs when ProgLoadAttr contains non-zero bytes past the end of
	// the struct known by the running kernel, meaning the kernel is too old to
	// support the given prog type.
	case errors.Is(err, unix.E2BIG):
		return true

	default:
		return false
	}
}

// Detect the freezer controller. It doesn't appear in the cgroup.controllers
// file. Check for the existence of the cgroup.freeze file in the troubleshoot cgroup
// instead, or try to create a dummy cgroup if troubleshoot runs in the root cgroup.
//
// https://github.com/torvalds/linux/blob/v5.3/Documentation/admin-guide/cgroup-v2.rst#core-interface-files
func detectV2FreezerController(mountPoint string) bool {

	// Detect the freezer controller by checking troubleshoot's cgroup for the existence
	// of the cgroup.freeze file.
	// https://github.com/torvalds/linux/blob/v5.3/Documentation/admin-guide/cgroup-v2.rst#processes
	cgroupPath, err := cgroup2.NestedGroupPath("")
	if err != nil {
		klog.V(2).Infof("failed to get troubleshoot cgroup: %v", err)
		return false
	}

	if cgroupPath != "/" {
		cgroupPath = filepath.Join(mountPoint, cgroupPath)
	} else { // The root cgroup cannot be frozen. Try to create a dummy cgroup.
		tmpCgroupPath, err := os.MkdirTemp(mountPoint, "troubleshoot-freezer-detection-*")
		if err != nil {
			if errors.Is(err, os.ErrPermission) && os.Geteuid() != 0 {
				// Insufficient permissions. Creating a cgroup requires elevated permissions
				klog.V(2).Info("insufficient permissions to create temporary cgroup")
			}
			if errors.Is(err, unix.EROFS) && os.Geteuid() != 0 {
				klog.V(2).Info("read-only file system detected when trying to create a temporary cgroup")
			}

			klog.V(2).Infof("failed to create temporary cgroup: %v", err)
			return false
		}
		defer func() { err = errors.Join(err, os.Remove(tmpCgroupPath)) }()
		cgroupPath = tmpCgroupPath
	}

	// Check if the cgroup.freeze exists
	if stat, err := os.Stat(filepath.Join(cgroupPath, "cgroup.freeze")); (err == nil && stat.IsDir()) || os.IsNotExist(err) {
		klog.V(2).Infof("cgroup.freeze exists at %q", cgroupPath)
		return false
	} else if err != nil {
		klog.V(2).Infof("failed to check for cgroup.freeze at %q: %v", cgroupPath, err)
		return false
	}

	klog.V(2).Infof("cgroup.freeze exists at %q", cgroupPath)
	return true
}
