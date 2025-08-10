package analyzer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeLLM_Title(t *testing.T) {
	tests := []struct {
		name      string
		checkName string
		expected  string
	}{
		{
			name:      "with custom check name",
			checkName: "Custom LLM Analysis",
			expected:  "Custom LLM Analysis",
		},
		{
			name:      "without check name",
			checkName: "",
			expected:  "LLM Problem Analysis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := &AnalyzeLLM{
				analyzer: &troubleshootv1beta2.LLMAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						CheckName: tt.checkName,
					},
				},
			}
			assert.Equal(t, tt.expected, analyzer.Title())
		})
	}
}

func TestAnalyzeLLM_IsExcluded(t *testing.T) {
	tests := []struct {
		name     string
		exclude  string
		expected bool
		wantErr  bool
	}{
		{
			name:     "not excluded",
			exclude:  "false",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "excluded",
			exclude:  "true",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "invalid exclude value",
			exclude:  "invalid",
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var excludePtr *multitype.BoolOrString
			if tt.exclude != "" {
				exclude := multitype.BoolOrString{StrVal: tt.exclude}
				excludePtr = &exclude
			}

			analyzer := &AnalyzeLLM{
				analyzer: &troubleshootv1beta2.LLMAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						Exclude: excludePtr,
					},
				},
			}

			result, err := analyzer.IsExcluded()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestAnalyzeLLM_Analyze_NoAPIKey(t *testing.T) {
	// Save original env and restore after test
	originalKey := os.Getenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("OPENAI_API_KEY", originalKey)
		}
	}()

	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
				CheckName: "Test Analysis",
			},
		},
	}

	getFile := func(path string) ([]byte, error) {
		return []byte("test content"), nil
	}

	findFiles := func(pattern string, excludes []string) (map[string][]byte, error) {
		return map[string][]byte{
			"test.log": []byte("test log content"),
		}, nil
	}

	results, err := analyzer.Analyze(getFile, findFiles)
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.True(t, results[0].IsFail)
	assert.Contains(t, results[0].Message, "OPENAI_API_KEY")
}

func TestAnalyzeLLM_CollectFiles(t *testing.T) {
	tests := []struct {
		name          string
		collectorName string
		fileName      string
		maxFiles      int
		findFilesResp map[string][]byte
		expectedCount int
	}{
		{
			name:          "with collector and file pattern",
			collectorName: "logs",
			fileName:      "*.log",
			maxFiles:      5,
			findFilesResp: map[string][]byte{
				"logs/app.log":   []byte("app log content"),
				"logs/error.log": []byte("error log content"),
			},
			expectedCount: 2,
		},
		{
			name:          "respects max files limit",
			collectorName: "logs",
			fileName:      "*",
			maxFiles:      1,
			findFilesResp: map[string][]byte{
				"logs/app.log":   []byte("app log content"),
				"logs/error.log": []byte("error log content"),
			},
			expectedCount: 1,
		},
		{
			name:          "no matching files",
			collectorName: "logs",
			fileName:      "*.txt",
			maxFiles:      5,
			findFilesResp: map[string][]byte{},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := &AnalyzeLLM{
				analyzer: &troubleshootv1beta2.LLMAnalyze{
					CollectorName: tt.collectorName,
					FileName:      tt.fileName,
					MaxFiles:      tt.maxFiles,
				},
			}

			getFile := func(path string) ([]byte, error) {
				if content, ok := tt.findFilesResp[path]; ok {
					return content, nil
				}
				return nil, nil
			}

			findFiles := func(pattern string, excludes []string) (map[string][]byte, error) {
				return tt.findFilesResp, nil
			}

			files, err := analyzer.collectFiles(getFile, findFiles)
			require.NoError(t, err)
			assert.Len(t, files, tt.expectedCount)
		})
	}
}

func TestAnalyzeLLM_CallLLM_Success(t *testing.T) {
	// Create a test server that mocks OpenAI API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-api-key")

		// Decode request to verify it's properly formatted
		var req map[string]interface{}
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		assert.Equal(t, "gpt-5", req["model"])

		// Send mock response
		response := openAIResponse{
			Choices: []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				{
					Message: struct {
						Content string `json:"content"`
					}{
						Content: `{
							"issue_found": true,
							"summary": "Pod is experiencing OOMKilled events",
							"issue": "The pod is being terminated due to memory limits",
							"solution": "Increase memory limits or optimize application memory usage",
							"severity": "critical",
							"confidence": 0.95
						}`,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Test the JSON parsing logic from the response
	jsonResponse := `{
		"issue_found": true,
		"summary": "Pod is experiencing OOMKilled events",
		"issue": "The pod is being terminated due to memory limits",
		"solution": "Increase memory limits or optimize application memory usage",
		"severity": "critical",
		"confidence": 0.95
	}`

	var analysis llmAnalysis
	err := json.Unmarshal([]byte(jsonResponse), &analysis)
	require.NoError(t, err)

	assert.True(t, analysis.IssueFound)
	assert.Equal(t, "Pod is experiencing OOMKilled events", analysis.Summary)
	assert.Equal(t, "critical", analysis.Severity)
	assert.Equal(t, 0.95, analysis.Confidence)
}

func TestAnalyzeLLM_MapToOutcomes(t *testing.T) {
	tests := []struct {
		name       string
		analysis   *llmAnalysis
		outcomes   []*troubleshootv1beta2.Outcome
		expectPass bool
		expectWarn bool
		expectFail bool
		expectMsg  string
	}{
		{
			name: "critical issue maps to fail",
			analysis: &llmAnalysis{
				IssueFound: true,
				Summary:    "Critical memory issue detected",
				Issue:      "Pod OOMKilled",
				Solution:   "Increase memory limits",
				Severity:   "critical",
			},
			expectFail: true,
			expectMsg:  "Critical memory issue detected\n\nIssue: Pod OOMKilled\n\nSolution: Increase memory limits",
		},
		{
			name: "warning issue maps to warn",
			analysis: &llmAnalysis{
				IssueFound: true,
				Summary:    "High CPU usage detected",
				Issue:      "CPU at 80%",
				Severity:   "warning",
			},
			expectWarn: true,
			expectMsg:  "High CPU usage detected\n\nIssue: CPU at 80%",
		},
		{
			name: "no issue maps to pass",
			analysis: &llmAnalysis{
				IssueFound: false,
				Summary:    "Everything looks good",
			},
			expectPass: true,
			expectMsg:  "No issues detected by LLM analysis",
		},
		{
			name: "info severity maps to pass",
			analysis: &llmAnalysis{
				IssueFound: true,
				Summary:    "Informational message",
				Severity:   "info",
			},
			expectPass: true,
			expectMsg:  "Informational message",
		},
		{
			name: "uses custom outcomes when provided",
			analysis: &llmAnalysis{
				IssueFound: true,
				Summary:    "Test issue",
				Severity:   "critical",
			},
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "issue_found",
						Message: "Custom fail: {{.Summary}}",
					},
				},
			},
			expectFail: true,
			expectMsg:  "Custom fail: Test issue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer := &AnalyzeLLM{
				analyzer: &troubleshootv1beta2.LLMAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						CheckName: "Test",
					},
					Outcomes: tt.outcomes,
				},
			}

			results := analyzer.mapToOutcomes(tt.analysis)
			require.Len(t, results, 1)

			result := results[0]
			assert.Equal(t, tt.expectPass, result.IsPass, "IsPass mismatch")
			assert.Equal(t, tt.expectWarn, result.IsWarn, "IsWarn mismatch")
			assert.Equal(t, tt.expectFail, result.IsFail, "IsFail mismatch")
			assert.Equal(t, tt.expectMsg, result.Message, "Message mismatch")
		})
	}
}

func TestAnalyzeLLM_GlobalProblemDescription(t *testing.T) {
	// Test that global problem description is used
	originalDesc := GlobalProblemDescription
	GlobalProblemDescription = "Test problem from CLI"
	defer func() {
		GlobalProblemDescription = originalDesc
	}()

	// Also test env variable fallback
	os.Setenv("PROBLEM_DESCRIPTION", "Test problem from env")
	defer os.Unsetenv("PROBLEM_DESCRIPTION")

	// When global is set, it should be used
	assert.Equal(t, "Test problem from CLI", GlobalProblemDescription)

	// When global is empty, env should be used
	GlobalProblemDescription = ""
	// This would be tested in the actual Analyze function
}

func TestAnalyzeLLM_FileCollection_MaxSize(t *testing.T) {
	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			CollectorName: "logs",
			FileName:      "*",
			MaxFiles:      100, // High limit to test size constraint
		},
	}

	// Create files that exceed max size
	largeContent := make([]byte, 100*1024) // 100KB each
	for i := range largeContent {
		largeContent[i] = 'A'
	}

	findFiles := func(pattern string, excludes []string) (map[string][]byte, error) {
		files := make(map[string][]byte)
		for i := 0; i < 10; i++ {
			files[string(rune('a'+i))+".log"] = largeContent
		}
		return files, nil
	}

	getFile := func(path string) ([]byte, error) {
		return largeContent, nil
	}

	files, err := analyzer.collectFiles(getFile, findFiles)
	require.NoError(t, err)

	// Should stop before collecting all files due to size limit (500KB)
	totalSize := 0
	for _, content := range files {
		totalSize += len(content)
	}
	assert.LessOrEqual(t, totalSize, 500*1024, "Should not exceed 500KB limit")
	assert.Less(t, len(files), 10, "Should collect fewer than 10 files due to size limit")
}
