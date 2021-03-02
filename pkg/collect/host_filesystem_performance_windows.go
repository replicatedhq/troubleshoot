package collect

import (
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func collectHostFilesystemPerformance(hostCollector *troubleshootv1beta2.FilesystemPerformance) (map[string][]byte, error) {
	return nil, errors.New("Filesystem performance collector is only implemented for Linux")
}
