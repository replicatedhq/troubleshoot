package collect

import (
	"encoding/json"
	"path/filepath"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
)

type DataOutput map[string][]byte

func Data(ctx *Context, dataCollector *troubleshootv1beta1.Data) ([]byte, error) {
	bundlePath := filepath.Join(dataCollector.Name, dataCollector.CollectorName)
	dataOutput := DataOutput{
		bundlePath: []byte(dataCollector.Data),
	}

	b, err := json.MarshalIndent(dataOutput, "", "  ")
	if err != nil {
		return nil, err
	}

	return b, nil
}
