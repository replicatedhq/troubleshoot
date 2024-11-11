package analyzer

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeHostSysctlCheckCondition(t *testing.T) {
	tests := []struct {
		name        string
		conditional string
		collected   string
		expected    bool
		expectErr   string
	}{
		{
			name:        "errors out if we can't unmarshal data",
			conditional: "net.ipv4.conf.all.arp_filter = 0",
			collected:   `{not JSON}`,
			expected:    false,
			expectErr:   "failed to unmarshal data",
		},
		{
			name:        "errors out if the matched conditional is missing elements",
			conditional: "net.ipv4.conf.all.arp_filter =",
			collected:   `{}`,
			expected:    false,
			expectErr:   `expected 3 parts in when "net.ipv4.conf.all.arp_filter ="`,
		},
		{
			name:        "errors out if the parameter in the condition was not collected",
			conditional: "net.ipv4.conf.all.arp_filter = 0",
			collected:   `{"net.ipv4.conf.all.arp_ignore": "0"}`,
			expected:    false,
			expectErr:   `"net.ipv4.conf.all.arp_filter" does not exist on collected sysctl`,
		},
		{
			name:        "errors out if the collected parameter does not support inequalities",
			conditional: "net.ipv4.tcp_available_congestion_control > 0",
			collected:   `{"net.ipv4.tcp_available_congestion_control": "reno cubic"}`,
			expected:    false,
			expectErr:   `has value "reno cubic", cannot be used with provided operator ">"`,
		},
		{
			name:        "errors out if the provided value for the conditional does not support inequalities",
			conditional: "net.ipv4.conf.all.arp_filter > broken",
			collected:   `{"net.ipv4.conf.all.arp_filter": "0"}`,
			expected:    false,
			expectErr:   `has value "broken", cannot be used with provided operator ">"`,
		},
		{
			name:        "errors out if the provided operator is unsupported",
			conditional: "net.ipv4.conf.all.arp_filter <== 0",
			collected:   `{"net.ipv4.conf.all.arp_filter": "0"}`,
			expected:    false,
			expectErr:   `failed to parse comparison operator "<=="`,
		},
		{
			name:        "equals with ints",
			conditional: "net.ipv4.conf.all.arp_filter = 0",
			collected:   `{"net.ipv4.conf.all.arp_filter": "0"}`,
			expected:    true,
		},
		{
			name:        "equals with different data types",
			conditional: "net.ipv4.conf.all.arp_filter = will be false",
			collected:   `{"net.ipv4.conf.all.arp_filter": "0"}`,
			expected:    false,
		},
		{
			name:        "equals with strings",
			conditional: "net.ipv4.tcp_available_congestion_control = reno cubic",
			collected:   `{"net.ipv4.tcp_available_congestion_control": "reno cubic"}`,
			expected:    true,
		},
		{
			name:        "triple equals works",
			conditional: "net.ipv4.tcp_available_congestion_control === reno cubic",
			collected:   `{"net.ipv4.tcp_available_congestion_control": "reno cubic"}`,
			expected:    true,
		},
		{
			name:        "double equals works",
			conditional: "net.ipv4.tcp_available_congestion_control == reno cubic",
			collected:   `{"net.ipv4.tcp_available_congestion_control": "reno cubic"}`,
			expected:    true,
		},
		{
			name:        "lower than succeeds",
			conditional: "net.ipv4.conf.default.arp_ignore < 1",
			collected:   `{"net.ipv4.conf.default.arp_ignore": "0"}`,
			expected:    true,
		},
		{
			name:        "lower than fails",
			conditional: "net.ipv4.conf.default.arp_ignore < 1",
			collected:   `{"net.ipv4.conf.default.arp_ignore": "1"}`,
			expected:    false,
		},
		{
			name:        "lower than or equals succeeds",
			conditional: "net.ipv4.conf.default.arp_ignore <= 1",
			collected:   `{"net.ipv4.conf.default.arp_ignore": "1"}`,
			expected:    true,
		},
		{
			name:        "lower than or equals fails",
			conditional: "net.ipv4.conf.default.arp_ignore <= 1",
			collected:   `{"net.ipv4.conf.default.arp_ignore": "2"}`,
			expected:    false,
		},
		{
			name:        "higher than succeeds",
			conditional: "net.ipv4.conf.default.arp_ignore > 1",
			collected:   `{"net.ipv4.conf.default.arp_ignore": "2"}`,
			expected:    true,
		},
		{
			name:        "higher than fails",
			conditional: "net.ipv4.conf.default.arp_ignore > 1",
			collected:   `{"net.ipv4.conf.default.arp_ignore": "1"}`,
			expected:    false,
		},
		{
			name:        "higher than or equals succeeds",
			conditional: "net.ipv4.conf.default.arp_ignore >= 1",
			collected:   `{"net.ipv4.conf.default.arp_ignore": "1"}`,
			expected:    true,
		},
		{
			name:        "higher than or equals fails",
			conditional: "net.ipv4.conf.default.arp_ignore >= 2",
			collected:   `{"net.ipv4.conf.default.arp_ignore": "1"}`,
			expected:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			analyzeHostSysctl := AnalyzeHostSysctl{}

			// JSON encoded sysctl collected output
			data := []byte(test.collected)

			// Call the CheckCondition method
			result, err := analyzeHostSysctl.CheckCondition(test.conditional, data)
			if test.expectErr != "" {
				req.ErrorContains(err, test.expectErr)
			} else {
				req.NoError(err)
			}
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestAnalyzeHostSysctl(t *testing.T) {
	tests := []struct {
		name                     string
		hostAnalyzer             *troubleshootv1beta2.HostSysctlAnalyze
		getCollectedFileContents func(string) ([]byte, error)
		expectedResults          []*AnalyzeResult
		expectedError            string
	}{
		{
			name: "Pass on successful condition (local)",
			hostAnalyzer: &troubleshootv1beta2.HostSysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.conf.default.arp_ignore >= 1",
							Message: "ARP ignore is enabled",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				// Simulate local sysctl content retrieval
				if path == collect.HostSysctlPath {

					data := map[string]string{
						"net.ipv4.conf.default.arp_ignore": "2",
					}

					return json.Marshal(data)
				}
				return nil, errors.New("file not found")
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Sysctl",
					IsPass:  true,
					Message: "ARP ignore is enabled",
				},
			},
			expectedError: "",
		},
		{
			name: "Fail on condition (remote node)",
			hostAnalyzer: &troubleshootv1beta2.HostSysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.conf.default.arp_filter = 0",
							Message: "ARP filter is disabled, please enable it via `sysctl net.ipv4.conf.default.arp_filter=1`",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				// Simulate remote node list and sysctl content retrieval
				if path == constants.NODE_LIST_FILE {
					nodeNames := nodeNames{Nodes: []string{"node1"}}
					return json.Marshal(nodeNames)
				}
				if path == fmt.Sprintf("%s/node1/%s", collect.NodeInfoBaseDir, collect.HostSysctlFileName) {
					data := map[string]string{
						"net.ipv4.conf.default.arp_filter": "0",
					}

					return json.Marshal(data)
				}
				return nil, errors.New("file not found")
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Sysctl - Node node1",
					IsFail:  true,
					Message: "ARP filter is disabled, please enable it via `sysctl net.ipv4.conf.default.arp_filter=1`",
				},
			},
			expectedError: "",
		},
		{
			name: "Warn on condition(remote node)",
			hostAnalyzer: &troubleshootv1beta2.HostSysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.tcp_available_congestion_control = reno cubic",
							Message: "Unexpected TCP congestion control algorithm available",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				// Simulate remote node list and sysctl content retrieval
				if path == constants.NODE_LIST_FILE {
					nodeNames := nodeNames{Nodes: []string{"node1"}}
					return json.Marshal(nodeNames)
				}
				if path == fmt.Sprintf("%s/node1/%s", collect.NodeInfoBaseDir, collect.HostSysctlFileName) {
					data := map[string]string{
						"net.ipv4.tcp_available_congestion_control": "reno cubic",
					}

					return json.Marshal(data)
				}
				return nil, errors.New("file not found")
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Sysctl - Node node1",
					IsWarn:  true,
					Message: "Unexpected TCP congestion control algorithm available",
				},
			},
			expectedError: "",
		},
		{
			name: "Return error if collection fails",
			hostAnalyzer: &troubleshootv1beta2.HostSysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.tcp_available_congestion_control = reno cubic",
							Message: "Unexpected TCP congestion control algorithm available",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				return nil, errors.New("file not found")
			},
			expectedResults: []*AnalyzeResult{
				{
					Title: "Sysctl",
				},
			},
			expectedError: "file not found",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			analyzeHostSysctl := AnalyzeHostSysctl{
				hostAnalyzer: test.hostAnalyzer,
			}

			results, err := analyzeHostSysctl.Analyze(test.getCollectedFileContents, nil)

			if test.expectedError != "" {
				req.ErrorContains(err, test.expectedError)
			} else {
				req.NoError(err)
			}
			req.Equal(test.expectedResults, results)
		})
	}
}
