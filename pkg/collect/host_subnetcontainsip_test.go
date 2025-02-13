package collect

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectHostSubnetContainsIP(t *testing.T) {
	tests := []struct {
		name        string
		collector   *CollectHostSubnetContainsIP
		expectErr   bool
		expectValue SubnetContainsIPResult
	}{
		{
			name: "ip is in subnet",
			collector: &CollectHostSubnetContainsIP{
				hostCollector: &troubleshootv1beta2.SubnetContainsIP{
					CIDR: "10.0.0.0/8",
					IP:   "10.0.0.5",
				},
			},
			expectValue: SubnetContainsIPResult{
				CIDR:     "10.0.0.0/8",
				IP:       "10.0.0.5",
				Contains: true,
			},
		},
		{
			name: "ip is not in subnet",
			collector: &CollectHostSubnetContainsIP{
				hostCollector: &troubleshootv1beta2.SubnetContainsIP{
					CIDR: "10.0.0.0/8",
					IP:   "192.168.1.1",
				},
			},
			expectValue: SubnetContainsIPResult{
				CIDR:     "10.0.0.0/8",
				IP:       "192.168.1.1",
				Contains: false,
			},
		},
		{
			name: "invalid CIDR",
			collector: &CollectHostSubnetContainsIP{
				hostCollector: &troubleshootv1beta2.SubnetContainsIP{
					CIDR: "invalid",
					IP:   "10.0.0.5",
				},
			},
			expectErr: true,
		},
		{
			name: "invalid IP",
			collector: &CollectHostSubnetContainsIP{
				hostCollector: &troubleshootv1beta2.SubnetContainsIP{
					CIDR: "10.0.0.0/8",
					IP:   "invalid",
				},
			},
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			result, err := test.collector.Collect(nil)

			if test.expectErr {
				req.Error(err)
				return
			}
			req.NoError(err)

			assert.Len(t, result, 1)

			var resultValue SubnetContainsIPResult
			err = json.Unmarshal(result["host-collectors/subnetContainsIP/result.json"], &resultValue)
			req.NoError(err)

			assert.Equal(t, test.expectValue, resultValue)
		})
	}
}
