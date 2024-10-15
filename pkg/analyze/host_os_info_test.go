package analyzer

import (
	"encoding/json"
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
			name:        "kernelVersion == 5.10 when actual is 5.10.42",
			conditional: "kernelVersion == 5.10",
			osInfo: collect.HostOSInfo{
				KernelVersion: "5.10.42",
			},
			expected:  true,
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
		hostAnalyzer             *troubleshootv1beta2.HostOSAnalyze // Different types of analyzers per test case
		getCollectedFileContents func(string) ([]byte, error)       // Mock function
		findFiles                getChildCollectedFileContents
		expectedResults          []*AnalyzeResult
		expectedError            string
	}{
		{
			name: "successfully retrieve local content and analyze",
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "os == localOS",
							Message: "local content analyzed",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == collect.HostOSInfoPath {
					return []byte(`{"Name": "localOS"}`), nil
				}
				return nil, errors.New("file not found")
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Host OS Info",
					IsPass:  true,
					Message: "local content analyzed",
				},
			},
			expectedError: "",
		},
		{
			name: "local content not found, retrieve and analyze remote content",
			hostAnalyzer: &troubleshootv1beta2.HostOSAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "os == remoteOS",
							Message: "remote content analyzed",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == constants.NODE_LIST_FILE {
					nodeNames := NodeNames{Nodes: []string{"node1"}}
					return json.Marshal(nodeNames)
				}
				if path == "remoteBaseDir/node1/remoteFileName" {
					return []byte(`{"Name": "remoteOS"}`), nil
				}
				return nil, errors.New("file not found")
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Host OS Info - Node node1",
					IsPass:  true,
					Message: "remote content analyzed",
				},
			},
			expectedError: "",
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
			expectedResults: []*AnalyzeResult{
				{
					Title: "Host OS Info",
				},
			},
			expectedError: "failed to get node list",
		},
		{
			name: "error during content analysis",
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
					nodeNames := NodeNames{Nodes: []string{"node1"}}
					return json.Marshal(nodeNames)
				}
				if path == "remoteBaseDir/node1/remoteFileName" {
					return []byte(`{"Name": "remoteOS"}`), nil
				}
				return nil, errors.New("file not found")
			},
			expectedResults: nil,
			expectedError:   "failed to analyze OS version",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Set up the AnalyzeHostOS object with the custom hostAnalyzer per test
			analyzeHostOS := AnalyzeHostOS{
				hostAnalyzer: test.hostAnalyzer,
			}

			// Call the Analyze function
			results, err := analyzeHostOS.Analyze(test.getCollectedFileContents, test.findFiles)

			// Check for errors and compare results
			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedResults, results)
			}
		})
	}
}
