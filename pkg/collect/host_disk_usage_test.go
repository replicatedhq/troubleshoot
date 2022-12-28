package collect

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectHostDiskUsage_Collect(t *testing.T) {
	tests := []struct {
		name          string
		collectorName string
		path          string
		wantErr       bool
	}{
		{
			name: "valid path with no collector name succeeds",
			path: "/",
		},
		{
			name:          "valid path with a collector name succeeds",
			path:          "/",
			collectorName: "custome-name",
		},
		{
			name:    "extra long path recursion fails",
			path:    strings.Repeat("/a", 51),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CollectHostDiskUsage{
				hostCollector: &troubleshootv1beta2.DiskUsage{
					Path: tt.path,
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: tt.collectorName,
					},
				},
				BundlePath: "",
			}
			got, err := c.Collect(nil)
			assert.Equal(t, tt.wantErr, err != nil)

			if err == nil {
				key := "host-collectors/diskUsage/diskUsage.json"
				if tt.collectorName != "" {
					key = fmt.Sprintf("host-collectors/diskUsage/%s.json", tt.collectorName)
				}
				require.Contains(t, got, key)
				values := got[key]

				var m map[string]int
				err = json.Unmarshal(values, &m)
				require.NoError(t, err)

				// Check if values exist. They will be different on different machines.
				assert.Equal(t, 2, len(m))
				assert.Contains(t, m, "total_bytes")
				assert.Contains(t, m, "used_bytes")
			}
		})
	}
}
