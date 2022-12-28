package collect

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectHostOS_Collect(t *testing.T) {
	c := &CollectHostOS{
		hostCollector: &troubleshootv1beta2.HostOS{},
		BundlePath:    "",
	}
	got, err := c.Collect(nil)
	require.NoError(t, err)

	require.Contains(t, got, "host-collectors/system/hostos_info.json")
	values := got["host-collectors/system/hostos_info.json"]

	var m map[string]string
	err = json.Unmarshal(values, &m)
	require.NoError(t, err)

	// Check if values exist. They will be different on different machines.
	assert.Equal(t, 4, len(m))
	assert.Contains(t, m, "name")
	assert.Contains(t, m, "kernelVersion")
	assert.Contains(t, m, "platformVersion")
	assert.Contains(t, m, "platformVersion")
}
