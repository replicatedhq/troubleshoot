//go:build !linux

package collect

import (
	"fmt"
)

func discoverConfiguration(_ string) (cgroupsResult, error) {
	return cgroupsResult{}, fmt.Errorf("Discovery of cgroups not inimplemented for this OS")
}
