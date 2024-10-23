package cli

import (
	"errors"
)

func checkAndSetChroot(newroot string) error {
	return errors.New("chroot is only implimented in linux/darwin")
}
