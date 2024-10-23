package cli

import (
	"errors"
	"syscall"

	"github.com/replicatedhq/troubleshoot/internal/util"
)

func checkAndSetChroot(newroot string) error {
	if newroot == "" {
		return nil
	}
	if !util.IsRunningAsRoot() {
		return errors.New("Can only chroot when run as root")
	}
	if err := syscall.Chroot(newroot); err != nil {
		return err
	}
	return nil
}
