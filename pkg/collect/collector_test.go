package collect

import (
	"testing"

	troubleshootv1beta1 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta1"
	"github.com/stretchr/testify/assert"
)

func Test_ParseSpec(t *testing.T) {
	tests := []struct {
		name         string
		spec         string
		expectError  bool
		expectObject interface{}
	}{
		{
			name:        "cluster-info",
			spec:        "cluster-info: {}",
			expectError: false,
			expectObject: &troubleshootv1beta1.Collect{
				ClusterInfo: &troubleshootv1beta1.ClusterInfo{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c, err := parseSpec(test.spec)

			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expectObject, c)
		})
	}
}
