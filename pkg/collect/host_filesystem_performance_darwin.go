package collect

import "github.com/pkg/errors"

func HostFilesystemPerformance(c *HostCollector) (map[string][]byte, error) {
	return nil, errors.New("Filesystem performance collector is only implemented for Linux")
}
