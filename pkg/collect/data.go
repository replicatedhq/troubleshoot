package collect

import (
	"bytes"
	"path/filepath"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func Data(c *Collector, dataCollector *troubleshootv1beta2.Data) (CollectorResult, error) {
	bundlePath := filepath.Join(dataCollector.Name, dataCollector.CollectorName)

	output := NewResult()
	output.SaveResult(c.BundlePath, bundlePath, bytes.NewBuffer([]byte(dataCollector.Data)))

	return output, nil
}
