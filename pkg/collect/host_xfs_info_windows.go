package collect

import (
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func collectXFSInfo(hostCollector *troubleshootv1beta2.XFSInfo) (map[string][]byte, error) {
	return nil, errors.New("XFS collector is only implemented for Linux")
}
