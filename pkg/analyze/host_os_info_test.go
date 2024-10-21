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

func TestAnalyzeHostOSCheckCondition(t *testing.T) {
	tests := []struct {
		name        string
		conditional string
		osInfo      collect.HostOSInfo
		expected    bool
		expectErr   bool
	}{
		{
			name:        "kernelVersion == 4.15 when actual is 4.15",
			conditional: "kernelVersion == 4.15",
			osInfo: collect.HostOSInfo{
				KernelVersion: "4.15.0",
			},
			expected:  true,
			expectErr: false,
		},
		{
			name:        "kernelVersion < 4.15 when actual is 4.16",
			conditional: "kernelVersion < 4.15",
			osInfo: collect.HostOSInfo{
				KernelVersion: "4.16.0",
			},
			expected:  false,
			expectErr: false,
		},
		{
			name:        "centos == 8.2 when actual is 8.2",
			conditional: "centos == 8.2",
			osInfo: collect.HostOSInfo{
				Platform:        "centos",
				PlatformVersion: "8.2",
			},
			expected:  true,
			expectErr: false,
		},
		{
			name:        "ubuntu == 20.04 when actual is 18.04",
			conditional: "ubuntu == 20.04",
			osInfo: collect.HostOSInfo{
				Platform:        "ubuntu",
				PlatformVersion: "18.04",
			},
			expected:  false,
			expectErr: false,
		},
		{
			name:        "invalid conditional format",
			conditional: "invalid conditional",
			osInfo: collect.HostOSInfo{
				Platform:        "ubuntu",
				PlatformVersion: "18.04",
			},
			expected:  false,
			expectErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Create the AnalyzeHostOS object
			analyzeHostOS := AnalyzeHostOS{}

			// Simulate the OS info as JSON-encoded data
			rawData, err := json.Marshal(test.osInfo)
			require.NoError(t, err)

			// Call the CheckCondition method
			result, err := analyzeHostOS.CheckCondition(test.conditional, rawData)
			if test.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected, result)
			}
		})
	}
}

func TestAnalyzeHostOS(t *testing.T) {
	tests := []struct {
		name                     string
		hostAnalyzer             *troubleshootv1beta2.HostOSAnalyze
		getCollectedFileContents func(string) ([]byte, error)
		result                   []*AnalyzeResult
		expectErr                bool
	}{
		{
			name: "successfully retrieve local content and analyze",
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu >= 00.1.2",
							Message: "supported distribution matches ubuntu >= 00.1.2",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "unsupported distribution",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == collect.HostOSInfoPath {
					return json.Marshal(collect.HostOSInfo{
						Name:            "myhost",
						KernelVersion:   "5.4.0-1034-gcp",
						PlatformVersion: "00.1.2",
						Platform:        "ubuntu",
					})
				}
				return nil, errors.New("file not found")
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info",
					IsPass:  true,
					Message: "supported distribution matches ubuntu >= 00.1.2",
				},
			},
			expectErr: false,
		},
		{
			name: "local content not found, retrieve and analyze remote content",
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu == 18.04",
							Message: "supported remote ubuntu 18.04",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "unsupported distribution",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == constants.NODE_LIST_FILE {
					return json.Marshal(nodeNames{Nodes: []string{"node1"}})
				}
				if path == fmt.Sprintf("%s/node1/%s", collect.NodeInfoBaseDir, collect.HostInfoFileName) {
					return json.Marshal(collect.HostOSInfo{
						Name:            "nodehost",
						KernelVersion:   "4.15.0-1034-aws",
						PlatformVersion: "18.04",
						Platform:        "ubuntu",
					})
				}
				return nil, errors.New("file not found")
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info - Node node1",
					IsPass:  true,
					Message: "supported remote ubuntu 18.04",
				},
			},
			expectErr: false,
		},
		{
			name: "fail to retrieve both local and remote content",
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "failed analysis",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				return nil, errors.New("file not found")
			},
			result: []*AnalyzeResult{
				{
					Title: "Host OS Info",
				},
			},
			expectErr: true,
		},
		{
			name: "error during remote content analysis",
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "analysis failed",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == constants.NODE_LIST_FILE {
					return json.Marshal(nodeNames{Nodes: []string{"node1"}})
				}
				if path == fmt.Sprintf("%s/node1/%s", collect.NodeInfoBaseDir, collect.HostInfoFileName) {
					return nil, errors.New("file not found")
				}
				return nil, errors.New("file not found")
			},
			result:    nil,
			expectErr: true,
		},
		{
			name: "pass if centos-1.2.0-kernel >= 1.2.0",
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "centos-1.2.0-kernel >= 1.2.0",
							Message: "supported kernel matches centos-1.2.0-kernel >= 1.2.0",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "unsupported distribution",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == constants.NODE_LIST_FILE {
					return json.Marshal(nodeNames{Nodes: []string{"node1"}})
				}
				if path == fmt.Sprintf("%s/node1/%s", collect.NodeInfoBaseDir, collect.HostInfoFileName) {
					return json.Marshal(collect.HostOSInfo{
						Name:            "nodehost",
						KernelVersion:   "1.2.0-1034-aws",
						PlatformVersion: "1.2.0",
						Platform:        "centos",
					})
				}
				return nil, errors.New("file not found")
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info - Node node1",
					IsPass:  true,
					Message: "supported kernel matches centos-1.2.0-kernel >= 1.2.0",
				},
			},
			expectErr: false,
		},
		{
			name: "warn if ubuntu <= 16.04",
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu <= 16.04",
							Message: "System performs best with Ubuntu version higher than 16.04",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu > 16.04",
							Message: "Ubuntu version is sufficient",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == collect.HostOSInfoPath {
					return json.Marshal(collect.HostOSInfo{
						Name:            "myhost",
						KernelVersion:   "4.15.0-1234-gcp",
						PlatformVersion: "16.04",
						Platform:        "ubuntu",
					})
				}
				return nil, errors.New("file not found")
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info",
					IsWarn:  true,
					Message: "System performs best with Ubuntu version higher than 16.04",
				},
			},
			expectErr: false,
		},
		{
			name: "analyze multiple nodes with different OS info",
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "ubuntu == 18.04",
							Message: "supported ubuntu version",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							Message: "unsupported ubuntu version",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == constants.NODE_LIST_FILE {
					return json.Marshal(nodeNames{Nodes: []string{"node1", "node2"}})
				}
				if path == fmt.Sprintf("%s/node1/%s", collect.NodeInfoBaseDir, collect.HostInfoFileName) {
					return json.Marshal(collect.HostOSInfo{
						Name:            "nodehost",
						KernelVersion:   "4.15.0-1034-aws",
						PlatformVersion: "18.04",
						Platform:        "ubuntu",
					})
				}
				if path == fmt.Sprintf("%s/node2/%s", collect.NodeInfoBaseDir, collect.HostInfoFileName) {
					return json.Marshal(collect.HostOSInfo{
						Name:            "nodehost",
						KernelVersion:   "4.15.0-1034-aws",
						PlatformVersion: "16.04",
						Platform:        "ubuntu",
					})
				}
				return nil, errors.New("file not found")
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host OS Info - Node node1",
					IsPass:  true,
					Message: "supported ubuntu version",
				},
				{
					Title:   "Host OS Info - Node node2",
					IsFail:  true,
					Message: "unsupported ubuntu version",
				},
			},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Set up the AnalyzeHostOS object with the custom hostAnalyzer per test
			analyzeHostOS := AnalyzeHostOS{
				hostAnalyzer: test.hostAnalyzer,
			}

			// Call the Analyze function
			results, err := analyzeHostOS.Analyze(test.getCollectedFileContents, nil)

			// Check for errors and compare results
			if test.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.result, results)
			}
		})
	}
}
