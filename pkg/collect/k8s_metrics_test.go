package collect

import (
	"testing"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConstructEndpoint(t *testing.T) {
	// Define test cases
	testCases := []struct {
		name             string
		metricRequest    troubleshootv1beta2.MetricRequest
		expectedEndpoint string
		expectedMetric   string
		expectedError    error
	}{
		{
			name: "Namespaced object with namespace and object name",
			metricRequest: troubleshootv1beta2.MetricRequest{
				Namespace:          "namespace",
				ObjectName:         "object",
				ResourceMetricName: "pods/metric",
			},
			expectedEndpoint: "/apis/custom.metrics.k8s.io/v1beta1/namespaces/namespace/pods/object/metric",
			expectedMetric:   "metric",
			expectedError:    nil,
		},
		{
			name: "Namespaced object with namespace and empty object name",
			metricRequest: troubleshootv1beta2.MetricRequest{
				Namespace:          "namespace",
				ObjectName:         "",
				ResourceMetricName: "pods/metric",
			},
			expectedEndpoint: "/apis/custom.metrics.k8s.io/v1beta1/namespaces/namespace/pods/*/metric",
			expectedMetric:   "metric",
			expectedError:    nil,
		},
		{
			name: "Non-namespaced object",
			metricRequest: troubleshootv1beta2.MetricRequest{
				ResourceMetricName: "nodes/metric",
				ObjectName:         "object",
			},
			expectedEndpoint: "/apis/custom.metrics.k8s.io/v1beta1/nodes/object/metric",
			expectedMetric:   "metric",
			expectedError:    nil,
		},
		{
			name: "Non-namespaced object with empty object name",
			metricRequest: troubleshootv1beta2.MetricRequest{
				ResourceMetricName: "namespaces/metric",
				ObjectName:         "",
			},
			expectedEndpoint: "/apis/custom.metrics.k8s.io/v1beta1/namespace/*/metric",
			expectedMetric:   "metric",
			expectedError:    nil,
		},
		{
			name: "Invalid metric name format",
			metricRequest: troubleshootv1beta2.MetricRequest{
				ResourceMetricName: "invalid-metric-name",
				ObjectName:         "object",
			},
			expectedEndpoint: "",
			expectedMetric:   "",
			expectedError:    errors.New("wrong metric name format"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function under test
			endpoint, metric, err := constructEndpoint(tc.metricRequest)

			// Verify the results
			if tc.expectedError != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedError.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedEndpoint, endpoint)
				assert.Equal(t, tc.expectedMetric, metric)
			}
		})
	}
}
