package collect

import (
	"path/filepath"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

func Data(c *Collector, dataCollector *troubleshootv1beta1.Data) (map[string][]byte, error) {
	bundlePath := filepath.Join(dataCollector.Name, dataCollector.CollectorName)
	dataOutput := map[string][]byte{
		bundlePath: []byte(dataCollector.Data),
	}

	return dataOutput, nil
}
