package collect

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectHostCPU_Collect(t *testing.T) {
	c := &CollectHostCPU{
		hostCollector: &troubleshootv1beta2.CPU{},
		BundlePath:    "",
	}
	got, err := c.Collect(nil)
	require.NoError(t, err)

	require.Contains(t, got, "host-collectors/system/cpu.json")
	values := got["host-collectors/system/cpu.json"]

	var m map[string]interface{}
	err = json.Unmarshal(values, &m)
	require.NoError(t, err)

	// Check if values exist. They will be different on different machines.
	assert.Equal(t, 3, len(m))
	assert.Contains(t, m, "logicalCount")
	assert.Contains(t, m, "physicalCount")
	assert.Contains(t, m, "flags")
	//assert.Contains(t, m, "machineArch")
}
