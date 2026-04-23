package analyzer

import (
	"encoding/json"
	"testing"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeHostRegistryImagesCheckCondition(t *testing.T) {
	tests := []struct {
		name        string
		conditional string
		data        collect.RegistryInfo
		expected    bool
		expectErr   string
	}{
		{
			name:        "all images verified",
			conditional: "missing == 0",
			data: collect.RegistryInfo{
				Images: map[string]collect.RegistryImage{
					"registry.example.com/app:v1": {Exists: true},
					"registry.example.com/app:v2": {Exists: true},
				},
			},
			expected: true,
		},
		{
			name:        "some images missing",
			conditional: "missing > 0",
			data: collect.RegistryInfo{
				Images: map[string]collect.RegistryImage{
					"registry.example.com/app:v1": {Exists: true},
					"registry.example.com/app:v2": {Exists: false},
				},
			},
			expected: true,
		},
		{
			name:        "verified count matches",
			conditional: "verified == 2",
			data: collect.RegistryInfo{
				Images: map[string]collect.RegistryImage{
					"registry.example.com/app:v1": {Exists: true},
					"registry.example.com/app:v2": {Exists: true},
				},
			},
			expected: true,
		},
		{
			name:        "errors count matches",
			conditional: "errors > 0",
			data: collect.RegistryInfo{
				Images: map[string]collect.RegistryImage{
					"registry.example.com/app:v1": {Error: "connection refused"},
				},
			},
			expected: true,
		},
		{
			name:        "errors count zero when no errors",
			conditional: "errors == 0",
			data: collect.RegistryInfo{
				Images: map[string]collect.RegistryImage{
					"registry.example.com/app:v1": {Exists: true},
				},
			},
			expected: true,
		},
		{
			name:        "mixed results - missing false",
			conditional: "missing == 0",
			data: collect.RegistryInfo{
				Images: map[string]collect.RegistryImage{
					"registry.example.com/app:v1": {Exists: true},
					"registry.example.com/app:v2": {Exists: false},
					"registry.example.com/app:v3": {Error: "timeout"},
				},
			},
			expected: false,
		},
		{
			name:        "invalid conditional format",
			conditional: "missing",
			data: collect.RegistryInfo{
				Images: map[string]collect.RegistryImage{},
			},
			expected:  false,
			expectErr: "unable to parse conditional",
		},
		{
			name:        "unmarshal error",
			conditional: "missing == 0",
			expected:    false,
			expectErr:   "failed to unmarshal registry info",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			a := &AnalyzeHostRegistryImages{}

			var data []byte
			if test.expectErr == "failed to unmarshal registry info" {
				data = []byte(`{not valid json}`)
			} else {
				var err error
				data, err = json.Marshal(test.data)
				req.NoError(err)
			}

			result, err := a.CheckCondition(test.conditional, data)
			if test.expectErr != "" {
				req.ErrorContains(err, test.expectErr)
			} else {
				req.NoError(err)
			}
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestAnalyzeHostRegistryImages(t *testing.T) {
	tests := []struct {
		name                     string
		hostAnalyzer             *troubleshootv1beta2.HostRegistryImagesAnalyze
		getCollectedFileContents func(string) ([]byte, error)
		expectedResults          []*AnalyzeResult
		expectedError            string
	}{
		{
			name: "pass when all images exist",
			hostAnalyzer: &troubleshootv1beta2.HostRegistryImagesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "missing == 0",
							Message: "All images are available",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == "host-collectors/registry-images/images.json" {
					return json.Marshal(collect.RegistryInfo{
						Images: map[string]collect.RegistryImage{
							"registry.example.com/app:v1": {Exists: true},
						},
					})
				}
				return nil, errors.New("file not found")
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Registry Images",
					IsPass:  true,
					Message: "All images are available",
				},
			},
		},
		{
			name: "fail when images are missing",
			hostAnalyzer: &troubleshootv1beta2.HostRegistryImagesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "missing > 0",
							Message: "Some images are not available",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == "host-collectors/registry-images/images.json" {
					return json.Marshal(collect.RegistryInfo{
						Images: map[string]collect.RegistryImage{
							"registry.example.com/app:v1": {Exists: false},
						},
					})
				}
				return nil, errors.New("file not found")
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Registry Images",
					IsFail:  true,
					Message: "Some images are not available",
				},
			},
		},
		{
			name: "custom collector name used in path",
			hostAnalyzer: &troubleshootv1beta2.HostRegistryImagesAnalyze{
				CollectorName: "my-registry",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "missing == 0",
							Message: "All images are available",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == "host-collectors/registry-images/my-registry.json" {
					return json.Marshal(collect.RegistryInfo{
						Images: map[string]collect.RegistryImage{
							"registry.example.com/app:v1": {Exists: true},
						},
					})
				}
				return nil, errors.New("file not found")
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Registry Images",
					IsPass:  true,
					Message: "All images are available",
				},
			},
		},
		{
			name: "return error when collection data missing",
			hostAnalyzer: &troubleshootv1beta2.HostRegistryImagesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "missing == 0",
							Message: "All images are available",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				return nil, errors.New("file not found")
			},
			expectedResults: []*AnalyzeResult{
				{
					Title: "Registry Images",
				},
			},
			expectedError: "file not found",
		},
		{
			name: "warn on errors",
			hostAnalyzer: &troubleshootv1beta2.HostRegistryImagesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "errors > 0",
							Message: "Some images could not be checked",
						},
					},
				},
			},
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == "host-collectors/registry-images/images.json" {
					return json.Marshal(collect.RegistryInfo{
						Images: map[string]collect.RegistryImage{
							"registry.example.com/app:v1": {Error: "connection refused"},
						},
					})
				}
				return nil, errors.New("file not found")
			},
			expectedResults: []*AnalyzeResult{
				{
					Title:   "Registry Images",
					IsWarn:  true,
					Message: "Some images could not be checked",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			a := &AnalyzeHostRegistryImages{
				hostAnalyzer: test.hostAnalyzer,
			}

			results, err := a.Analyze(test.getCollectedFileContents, nil)

			if test.expectedError != "" {
				req.ErrorContains(err, test.expectedError)
			} else {
				req.NoError(err)
			}
			req.Equal(test.expectedResults, results)
		})
	}
}

func TestAnalyzeHostRegistryImagesTitle(t *testing.T) {
	t.Run("default title", func(t *testing.T) {
		a := &AnalyzeHostRegistryImages{
			hostAnalyzer: &troubleshootv1beta2.HostRegistryImagesAnalyze{},
		}
		assert.Equal(t, "Registry Images", a.Title())
	})

	t.Run("custom title", func(t *testing.T) {
		a := &AnalyzeHostRegistryImages{
			hostAnalyzer: &troubleshootv1beta2.HostRegistryImagesAnalyze{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "My Registry Check",
				},
			},
		}
		assert.Equal(t, "My Registry Check", a.Title())
	})
}
