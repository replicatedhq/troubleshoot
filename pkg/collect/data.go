package collect

import (
	"path/filepath"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func Data(c *Collector, dataCollector *troubleshootv1beta2.Data) (map[string][]byte, error) {
	bundlePath := filepath.Join(dataCollector.Name, dataCollector.CollectorName)
	dataOutput := map[string][]byte{
		bundlePath: []byte(dataCollector.Data),
	}

	return dataOutput, nil
}
