// +build !windows

package util

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	iscsi_util "github.com/longhorn/go-iscsi-helper/util"
)

func GetDiskInfo(directory string) (info *DiskInfo, err error) {
	defer func() {
		err = errors.Wrapf(err, "cannot get disk info of directory %v", directory)
	}()
	initiatorNSPath := iscsi_util.GetHostNamespacePath(HostProcPath)
	mountPath := fmt.Sprintf("--mount=%s/mnt", initiatorNSPath)
	output, err := Execute([]string{}, "nsenter", mountPath, "stat", "-fc", "{\"path\":\"%n\",\"fsid\":\"%i\",\"type\":\"%T\",\"freeBlock\":%f,\"totalBlock\":%b,\"blockSize\":%S}", directory)
	if err != nil {
		return nil, err
	}
	output = strings.Replace(output, "\n", "", -1)

	diskInfo := &DiskInfo{}
	err = json.Unmarshal([]byte(output), diskInfo)
	if err != nil {
		return nil, err
	}

	diskInfo.StorageMaximum = diskInfo.TotalBlock * diskInfo.BlockSize
	diskInfo.StorageAvailable = diskInfo.FreeBlock * diskInfo.BlockSize

	return diskInfo, nil
}

func RemoveHostDirectoryContent(directory string) (err error) {
	defer func() {
		err = errors.Wrapf(err, "failed to remove host directory %v", directory)
	}()

	dir, err := filepath.Abs(filepath.Clean(directory))
	if err != nil {
		return err
	}
	if strings.Count(dir, "/") < 2 {
		return fmt.Errorf("prohibit removing the top level of directory %v", dir)
	}
	initiatorNSPath := iscsi_util.GetHostNamespacePath(HostProcPath)
	nsExec, err := iscsi_util.NewNamespaceExecutor(initiatorNSPath)
	if err != nil {
		return err
	}
	// check if the directory already deleted
	if _, err := nsExec.Execute("ls", []string{dir}); err != nil {
		logrus.Warnf("cannot find host directory %v for removal", dir)
		return nil
	}
	if _, err := nsExec.Execute("rm", []string{"-rf", dir}); err != nil {
		return err
	}
	return nil
}

func CopyHostDirectoryContent(src, dest string) (err error) {
	defer func() {
		err = errors.Wrapf(err, "failed to copy the content from %v to %v for the host", src, dest)
	}()

	srcDir, err := filepath.Abs(filepath.Clean(src))
	if err != nil {
		return err
	}
	destDir, err := filepath.Abs(filepath.Clean(dest))
	if err != nil {
		return err
	}
	if strings.Count(srcDir, "/") < 2 || strings.Count(destDir, "/") < 2 {
		return fmt.Errorf("prohibit copying the content for the top level of directory %v or %v", srcDir, destDir)
	}

	initiatorNSPath := iscsi_util.GetHostNamespacePath(HostProcPath)
	nsExec, err := iscsi_util.NewNamespaceExecutor(initiatorNSPath)
	if err != nil {
		return err
	}

	// There can be no src directory, hence returning nil is fine.
	if _, err := nsExec.Execute("bash", []string{"-c", fmt.Sprintf("ls %s", filepath.Join(srcDir, "*"))}); err != nil {
		logrus.Infof("cannot list the content of the src directory %v for the copy, will do nothing: %v", srcDir, err)
		return nil
	}
	// Check if the dest directory exists.
	if _, err := nsExec.Execute("mkdir", []string{"-p", destDir}); err != nil {
		return err
	}
	// The flag `-n` means not overwriting an existing file.
	if _, err := nsExec.Execute("bash", []string{"-c", fmt.Sprintf("cp -an %s %s", filepath.Join(srcDir, "*"), destDir)}); err != nil {
		return err
	}
	return nil
}

func CreateDiskPathReplicaSubdirectory(path string) error {
	nsPath := iscsi_util.GetHostNamespacePath(HostProcPath)
	nsExec, err := iscsi_util.NewNamespaceExecutor(nsPath)
	if err != nil {
		return err
	}
	if _, err := nsExec.Execute("mkdir", []string{"-p", filepath.Join(path, ReplicaDirectory)}); err != nil {
		return errors.Wrapf(err, "error creating data path %v on host", path)
	}

	return nil
}

func DeleteDiskPathReplicaSubdirectoryAndDiskCfgFile(
	nsExec *iscsi_util.NamespaceExecutor, path string) error {

	var err error
	dirPath := filepath.Join(path, ReplicaDirectory)
	filePath := filepath.Join(path, DiskConfigFile)

	// Check if the replica directory exist, delete it
	if _, err := nsExec.Execute("ls", []string{dirPath}); err == nil {
		if _, err := nsExec.Execute("rmdir", []string{dirPath}); err != nil {
			return errors.Wrapf(err, "error deleting data path %v on host", path)
		}
	}

	// Check if the disk cfg file exist, delete it
	if _, err := nsExec.Execute("ls", []string{filePath}); err == nil {
		if _, err := nsExec.Execute("rm", []string{filePath}); err != nil {
			err = errors.Wrapf(err, "error deleting disk cfg file %v on host", filePath)
		}
	}

	return err
}

func ExpandFileSystem(volumeName string) (err error) {
	devicePath := filepath.Join(DeviceDirectory, volumeName)
	nsPath := iscsi_util.GetHostNamespacePath(HostProcPath)
	nsExec, err := iscsi_util.NewNamespaceExecutor(nsPath)
	if err != nil {
		return err
	}

	fsType, err := DetectFileSystem(volumeName)
	if err != nil {
		return err
	}
	if !IsSupportedFileSystem(fsType) {
		return fmt.Errorf("volume %v is using unsupported file system %v", volumeName, fsType)
	}

	// make sure there is a mount point for the volume before file system expansion
	tmpMountNeeded := true
	mountPoint := ""
	mountRes, err := nsExec.Execute("bash", []string{"-c", "mount | grep \"/" + volumeName + " \" | awk '{print $3}'"})
	if err != nil {
		logrus.Warnf("failed to use command mount to get the mount info of volume %v, consider the volume as unmounted: %v", volumeName, err)
	} else {
		// For empty `mountRes`, `mountPoints` is [""]
		mountPoints := strings.Split(strings.TrimSpace(mountRes), "\n")
		if !(len(mountPoints) == 1 && strings.TrimSpace(mountPoints[0]) == "") {
			// pick up a random mount point
			for _, m := range mountPoints {
				mountPoint = strings.TrimSpace(m)
				if mountPoint != "" {
					tmpMountNeeded = false
					break
				}
			}
			if tmpMountNeeded {
				logrus.Errorf("BUG: Found mount point records %v for volume %v but there is no valid(non-empty) mount point", mountRes, volumeName)
			}
		}
	}
	if tmpMountNeeded {
		mountPoint = filepath.Join(TemporaryMountPointDirectory, volumeName)
		logrus.Infof("The volume %v is unmounted, hence it will be temporarily mounted on %v for file system expansion", volumeName, mountPoint)
		if _, err := nsExec.Execute("mkdir", []string{"-p", mountPoint}); err != nil {
			return errors.Wrapf(err, "failed to create a temporary mount point %v before file system expansion", mountPoint)
		}
		if _, err := nsExec.Execute("mount", []string{devicePath, mountPoint}); err != nil {
			return errors.Wrapf(err, "failed to temporarily mount volume %v on %v before file system expansion", volumeName, mountPoint)
		}
	}

	switch fsType {
	case "ext2":
		fallthrough
	case "ext3":
		fallthrough
	case "ext4":
		if _, err = nsExec.Execute("resize2fs", []string{devicePath}); err != nil {
			return err
		}
	case "xfs":
		if _, err = nsExec.Execute("xfs_growfs", []string{mountPoint}); err != nil {
			return err
		}
	default:
		return fmt.Errorf("volume %v is using unsupported file system %v", volumeName, fsType)
	}

	// cleanup
	if tmpMountNeeded {
		if _, err := nsExec.Execute("umount", []string{mountPoint}); err != nil {
			return errors.Wrapf(err, "failed to unmount volume %v on the temporary mount point %v after file system expansion", volumeName, mountPoint)
		}
		if _, err := nsExec.Execute("rm", []string{"-r", mountPoint}); err != nil {
			return errors.Wrapf(err, "failed to remove the temporary mount point %v after file system expansion", mountPoint)
		}
	}

	return nil
}

func DetectFileSystem(volumeName string) (string, error) {
	devicePath := filepath.Join(DeviceDirectory, volumeName)
	nsPath := iscsi_util.GetHostNamespacePath(HostProcPath)
	nsExec, err := iscsi_util.NewNamespaceExecutor(nsPath)
	if err != nil {
		return "", err
	}

	// The output schema of `blkid` can be different.
	// For filesystem `btrfs`, the schema is: `<device path>: UUID="<filesystem UUID>" UUID_SUB="<filesystem UUID_SUB>" TYPE="<filesystem type>"`
	// For filesystem `ext4` or `xfs`, the schema is: `<device path>: UUID="<filesystem UUID>" TYPE="<filesystem type>"`
	cmd := fmt.Sprintf("blkid %s | sed 's/.*TYPE=//g'", devicePath)
	output, err := nsExec.Execute("bash", []string{"-c", cmd})
	if err != nil {
		return "", errors.Wrapf(err, "failed to get the file system info for volume %v, maybe there is no Linux file system on the volume", volumeName)
	}
	fsType := strings.Trim(strings.TrimSpace(output), "\"")
	if fsType == "" {
		return "", fmt.Errorf("cannot get the filesystem type by using the command %v", cmd)
	}
	return fsType, nil
}

func GetDiskConfig(path string) (*DiskConfig, error) {
	nsPath := iscsi_util.GetHostNamespacePath(HostProcPath)
	nsExec, err := iscsi_util.NewNamespaceExecutor(nsPath)
	if err != nil {
		return nil, err
	}
	filePath := filepath.Join(path, DiskConfigFile)
	output, err := nsExec.Execute("cat", []string{filePath})
	if err != nil {
		return nil, fmt.Errorf("cannot find config file %v on host: %v", filePath, err)
	}

	cfg := &DiskConfig{}
	if err := json.Unmarshal([]byte(output), cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %v content %v on host: %v", filePath, output, err)
	}
	return cfg, nil
}

func GenerateDiskConfig(path string) (*DiskConfig, error) {
	cfg := &DiskConfig{
		DiskUUID: UUID(),
	}
	encoded, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("BUG: Cannot marshal %+v: %v", cfg, err)
	}

	nsPath := iscsi_util.GetHostNamespacePath(HostProcPath)
	nsExec, err := iscsi_util.NewNamespaceExecutor(nsPath)
	if err != nil {
		return nil, err
	}
	filePath := filepath.Join(path, DiskConfigFile)
	if _, err := nsExec.Execute("ls", []string{filePath}); err == nil {
		return nil, fmt.Errorf("disk cfg on %v exists, cannot override", filePath)
	}

	defer func() {
		if err != nil {
			if derr := DeleteDiskPathReplicaSubdirectoryAndDiskCfgFile(nsExec, path); derr != nil {
				err = errors.Wrapf(err, "cleaning up disk config path %v failed with error: %v", path, derr)
			}

		}
	}()

	if _, err := nsExec.ExecuteWithStdin("dd", []string{"of=" + filePath}, string(encoded)); err != nil {
		return nil, fmt.Errorf("cannot write to disk cfg on %v: %v", filePath, err)
	}
	if err := CreateDiskPathReplicaSubdirectory(path); err != nil {
		return nil, err
	}
	if _, err := nsExec.Execute("sync", []string{filePath}); err != nil {
		return nil, fmt.Errorf("cannot sync disk cfg on %v: %v", filePath, err)
	}

	return cfg, nil
}
