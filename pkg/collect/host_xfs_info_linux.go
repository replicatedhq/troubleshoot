package collect

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"golang.org/x/sys/unix"
	"gopkg.in/yaml.v2"
)

// https://github.com/torvalds/linux/blob/7fef2edf7cc753b51f7ccc74993971b0a9c81eca/fs/xfs/libxfs/xfs_fs.h#L247
const XFS_FSOP_GEOM_FLAGS_FTYPE = 1 << 16

// https://github.com/torvalds/linux/blob/7fef2edf7cc753b51f7ccc74993971b0a9c81eca/fs/xfs/libxfs/xfs_fs.h#L127
type xfs_fsop_geom struct {
	blocksize    uint32
	rtextsize    uint32
	agblocks     uint32
	agcount      uint32
	logblocks    uint32
	sectsize     uint32
	inodesize    uint32
	imaxpct      uint32
	datablocks   uint64
	rtblocks     uint64
	rtextents    uint64
	logstart     uint64
	uuid         [16]byte
	sunit        uint32
	swidth       uint32
	version      int32
	flags        uint32
	logsectsize  uint32
	rtsectsize   uint32
	dirblocksize uint32
}

func init() {
	var geom xfs_fsop_geom
	if int(unsafe.Offsetof(geom.flags)) != 92 {
		println(int(unsafe.Offsetof(geom.flags)))
		log.Fatal("Memory layout of xfs_fsop_geom struct has changed")
	}
}

func collectXFSInfo(hostCollector *troubleshootv1beta2.XFSInfo) (map[string][]byte, error) {
	ret := XFSInfo{}

	// 1. Find the deepest file on the path that exists. It does not have to be the mountpoint.
	filename := hostCollector.Path
	for i := 20; i >= 0; i-- {
		if _, err := os.Stat(filename); os.IsNotExist(err) {
			next := filepath.Dir(filename)
			if filename == next || filename == "." || i == 0 {
				return nil, fmt.Errorf("Path %q not found", hostCollector.Path)
			}
			filename = next
		} else if err != nil {
			return nil, err
		}
	}

	// 2. Check if an XFS filesystem is mounted at the path
	statfs_t := &syscall.Statfs_t{}
	if err := syscall.Statfs(filename, statfs_t); err != nil {
		return nil, err
	}
	ret.IsXFS = statfs_t.Type == unix.XFS_SUPER_MAGIC

	// 3. If it's an XFS filesystem check if XFS_FSOP_GEOM_FLAGS_FTYPE is enabled
	if ret.IsXFS {
		f, err := os.Open(filename)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		// https://github.com/torvalds/linux/blob/7fef2edf7cc753b51f7ccc74993971b0a9c81eca/fs/xfs/libxfs/xfs_fs.h#L809
		// https://github.com/torvalds/linux/blob/5bfc75d92efd494db37f5c4c173d3639d4772966/include/uapi/asm-generic/ioctl.h
		opcode := 100
		opcode |= 'X' << 8
		opcode |= int(unsafe.Sizeof(xfs_fsop_geom{})) << 16
		opcode |= 2 << 30

		geom := &xfs_fsop_geom{}

		_, _, errno := unix.Syscall(
			unix.SYS_IOCTL,
			f.Fd(),
			uintptr(opcode),
			uintptr(unsafe.Pointer(geom)),
		)
		if errno != 0 {
			return nil, os.NewSyscallError("ioctl", fmt.Errorf("%d", int(errno)))
		}

		ret.IsFtypeEnabled = (geom.flags & XFS_FSOP_GEOM_FLAGS_FTYPE) != 0
	}

	b, err := yaml.Marshal(ret)
	if err != nil {
		return nil, err
	}

	key := GetXFSPath(hostCollector.CollectorName)

	return map[string][]byte{key: b}, nil
}
