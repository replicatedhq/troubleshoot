package collect

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectHostMemory_Collect(t *testing.T) {
	c := &CollectHostMemory{
		hostCollector: &troubleshootv1beta2.Memory{},
		BundlePath:    "",
	}
	got, err := c.Collect(nil)
	require.NoError(t, err)

	require.Contains(t, got, "host-collectors/system/memory.json")
	values := got["host-collectors/system/memory.json"]

	var m map[string]int
	err = json.Unmarshal(values, &m)
	require.NoError(t, err)

	// Check if values exist. They will be different on different machines.
	assert.Equal(t, 1, len(m))
	assert.Contains(t, m, "total")
}
