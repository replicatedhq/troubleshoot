//go:build windows
// +build windows

package util

import (
	"github.com/pkg/errors"
)

func GetDiskInfo(directory string) (*DiskInfo, error) {
	return nil, errors.Errorf("cannot get disk info of directory %v", directory)
}

func RemoveHostDirectoryContent(directory string) error {
	return errors.Errorf("failed to remove host directory %v", directory)
}

func CopyHostDirectoryContent(src, dest string) error {
	return errors.Errorf("failed to copy the content from %v to %v for the host", src, dest)
}

func CreateDiskPathReplicaSubdirectory(path string) error {
	return errors.Errorf("error creating data path %v on host", path)
}

func ExpandFileSystem(volumeName string) error {
	return errors.Errorf("error expanding filsystem on %v", volumeName)
}

func DetectFileSystem(volumeName string) (string, error) {
	return "", errors.Errorf("error detecting filsystem on %v", volumeName)
}

func GetDiskConfig(path string) (*DiskConfig, error) {
	return nil, errors.Errorf("error getting disk config from %v", path)
}

func GenerateDiskConfig(path string) (*DiskConfig, error) {
	return nil, errors.Errorf("error generating disk config from %v", path)
}
