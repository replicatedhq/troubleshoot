package cli

import (
	"syscall"
)

func checkAndSetChroot(newroot string) error {
	if newroot == "" {
		return nil
	}
	if err := syscall.Chroot(newroot); err != nil {
		return err
	}
	return nil
}
