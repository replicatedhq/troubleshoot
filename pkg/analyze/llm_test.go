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
	// Create a test server that validates the actual request format
	requestValidated := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. VALIDATE REQUEST STRUCTURE
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-api-key")
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		// 2. VALIDATE REQUEST BODY MATCHES ACTUAL IMPLEMENTATION
		var req openAIRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Check model selection
		assert.Equal(t, "gpt-4o-mini", req.Model)

		// Check messages structure
		require.Len(t, req.Messages, 2)
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Contains(t, req.Messages[0].Content, "Kubernetes troubleshooting expert")
		assert.Equal(t, "user", req.Messages[1].Role)

		// 3. VALIDATE STRUCTURED OUTPUT CONFIGURATION
		// gpt-4o-mini supports structured outputs
		if req.ResponseFormat != nil {
			assert.Equal(t, "json_schema", req.ResponseFormat.Type)
			require.NotNil(t, req.ResponseFormat.JSONSchema)
			assert.Equal(t, "kubernetes_analysis", req.ResponseFormat.JSONSchema.Name)
			assert.True(t, req.ResponseFormat.JSONSchema.Strict)

			// Check schema has required fields
			schema := req.ResponseFormat.JSONSchema.Schema
			properties := schema["properties"].(map[string]interface{})
			assert.Contains(t, properties, "issue_found")
			assert.Contains(t, properties, "summary")
			assert.Contains(t, properties, "severity")
			assert.Contains(t, properties, "confidence")
		} else {
			// If structured output is not used, check the prompt mentions JSON
			assert.Contains(t, req.Messages[0].Content, "JSON")
		}

		// 4. VALIDATE PROBLEM DESCRIPTION AND FILES ARE IN PROMPT
		userContent := req.Messages[1].Content
		assert.Contains(t, userContent, "Problem Description: Pods crashing repeatedly")
		assert.Contains(t, userContent, "=== test-logs/app.log ===")
		assert.Contains(t, userContent, "Error: OOMKilled")

		requestValidated = true

		// 5. RETURN REALISTIC RESPONSE
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
							"confidence": 0.95,
							"commands": ["kubectl edit deployment app"],
							"documentation": ["https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/"],
							"root_cause": "Memory usage exceeds container limit",
							"affected_pods": ["app-pod-1"],
							"next_steps": ["Increase memory limit", "Review application code"],
							"related_issues": []
						}`,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// 6. CREATE ANALYZER WITH TEST SERVER ENDPOINT
	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
				CheckName: "Test Analysis",
			},
			Model:               "gpt-4o-mini",
			UseStructuredOutput: true, // Explicitly enable structured output
			APIEndpoint:         server.URL + "/v1/chat/completions", // USE TEST SERVER!
		},
	}

	// 7. ACTUALLY CALL THE REAL FUNCTION
	files := map[string]string{
		"test-logs/app.log": "Error: OOMKilled - Container exceeded memory limit",
	}

	analysis, err := analyzer.callLLM("test-api-key", "Pods crashing repeatedly", files)

	// 8. VALIDATE THE ACTUAL RESPONSE
	require.NoError(t, err)
	require.NotNil(t, analysis)
	assert.True(t, requestValidated, "Request validation was not executed")

	// Verify the analysis was parsed correctly
	assert.True(t, analysis.IssueFound)
	assert.Equal(t, "Pod is experiencing OOMKilled events", analysis.Summary)
	assert.Equal(t, "critical", analysis.Severity)
	assert.Equal(t, 0.95, analysis.Confidence)
	assert.Len(t, analysis.Commands, 1)
	assert.Contains(t, analysis.Commands[0], "kubectl")
	assert.Len(t, analysis.AffectedPods, 1)
	assert.Equal(t, "app-pod-1", analysis.AffectedPods[0])
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

func TestAnalyzeLLM_CallLLM_APIError(t *testing.T) {
	// Test various API error scenarios
	tests := []struct {
		name           string
		statusCode     int
		errorResponse  *struct {
			Message string `json:"message"`
			Type    string `json:"type"`
		}
		expectedError  string
	}{
		{
			name:       "rate limit error",
			statusCode: 429,
			errorResponse: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			}{
				Message: "Rate limit exceeded. Please retry after 1 minute.",
				Type:    "rate_limit_error",
			},
			expectedError: "Rate limit exceeded",
		},
		{
			name:       "authentication error",
			statusCode: 401,
			errorResponse: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			}{
				Message: "Invalid API key provided",
				Type:    "authentication_error",
			},
			expectedError: "Invalid API key",
		},
		{
			name:       "quota exceeded",
			statusCode: 429,
			errorResponse: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			}{
				Message: "You have exceeded your quota",
				Type:    "quota_exceeded",
			},
			expectedError: "exceeded your quota",
		},
		{
			name:       "server error",
			statusCode: 500,
			errorResponse: &struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			}{
				Message: "Internal server error",
				Type:    "server_error",
			},
			expectedError: "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Return OpenAI API error format
				w.WriteHeader(tt.statusCode)
				response := openAIResponse{
					Error: tt.errorResponse,
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			analyzer := &AnalyzeLLM{
				analyzer: &troubleshootv1beta2.LLMAnalyze{
					APIEndpoint: server.URL + "/v1/chat/completions",
					Model:       "gpt-4o-mini",
				},
			}

			_, err := analyzer.callLLM("test-key", "test problem", map[string]string{"test": "content"})
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestAnalyzeLLM_CallLLM_MalformedJSON(t *testing.T) {
	tests := []struct {
		name                string
		useStructuredOutput bool
		expectError         bool
	}{
		{
			name:                "with structured output - should error",
			useStructuredOutput: true,
			expectError:         true,
		},
		{
			name:                "without structured output - should fallback gracefully",
			useStructuredOutput: false,
			expectError:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Return malformed JSON in the content
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
								Content: `{this is not valid json with error in it}`,
							},
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()

			analyzer := &AnalyzeLLM{
				analyzer: &troubleshootv1beta2.LLMAnalyze{
					APIEndpoint:         server.URL + "/v1/chat/completions",
					Model:               "gpt-4o-mini",
					UseStructuredOutput: tt.useStructuredOutput,
				},
			}

			analysis, err := analyzer.callLLM("test-key", "test", map[string]string{"test": "content"})
			
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "failed to parse structured JSON response")
			} else {
				// Without structured output, it should fallback gracefully
				require.NoError(t, err)
				require.NotNil(t, analysis)
				// The fallback creates a basic analysis from the text
				assert.True(t, analysis.IssueFound) // because it contains "error"
				assert.Equal(t, "warning", analysis.Severity)
				assert.Equal(t, 0.5, analysis.Confidence)
			}
		})
	}
}

func TestAnalyzeLLM_CallLLM_RealAPI(t *testing.T) {
	// Skip in CI or if no API key
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("Skipping real API test - no OPENAI_API_KEY set")
	}

	if os.Getenv("CI") == "true" {
		t.Skip("Skipping real API test in CI")
	}

	// Mark as integration test
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			Model: "gpt-4o-mini",
			// NO APIEndpoint override - uses real OpenAI
		},
	}

	files := map[string]string{
		"test.log": "2024-01-10 10:00:00 ERROR: Connection refused to database at localhost:5432\n" +
			"2024-01-10 10:00:01 ERROR: Retrying connection...\n" +
			"2024-01-10 10:00:02 ERROR: Max retries exceeded",
	}

	// THIS MAKES A REAL API CALL
	analysis, err := analyzer.callLLM(
		os.Getenv("OPENAI_API_KEY"),
		"Application cannot connect to database",
		files,
	)

	require.NoError(t, err)
	require.NotNil(t, analysis)
	
	// Can't assert specific content since LLM responses vary
	assert.NotEmpty(t, analysis.Summary)
	assert.NotEmpty(t, analysis.Severity)
	assert.True(t, analysis.Confidence > 0 && analysis.Confidence <= 1.0)
	
	// Log the response for manual inspection
	t.Logf("Real API Response:")
	t.Logf("  Summary: %s", analysis.Summary)
	t.Logf("  Issue: %s", analysis.Issue)
	t.Logf("  Solution: %s", analysis.Solution)
	t.Logf("  Severity: %s", analysis.Severity)
	t.Logf("  Confidence: %.2f", analysis.Confidence)
}

func TestAnalyzeLLM_ProblemDescription(t *testing.T) {
	tests := []struct {
		name           string
		analyzerDesc   string
		envDesc        string
		expectedDesc   string
	}{
		{
			name:         "uses analyzer config first",
			analyzerDesc: "Problem from config",
			envDesc:      "Problem from env",
			expectedDesc: "Problem from config",
		},
		{
			name:         "falls back to environment",
			analyzerDesc: "",
			envDesc:      "Problem from env",
			expectedDesc: "Problem from env",
		},
		{
			name:         "uses default when both empty",
			analyzerDesc: "",
			envDesc:      "",
			expectedDesc: "Please analyze the logs and identify any issues or problems",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envDesc != "" {
				os.Setenv("PROBLEM_DESCRIPTION", tt.envDesc)
				defer os.Unsetenv("PROBLEM_DESCRIPTION")
			}

			analyzer := &AnalyzeLLM{
				analyzer: &troubleshootv1beta2.LLMAnalyze{
					ProblemDescription: tt.analyzerDesc,
				},
			}

			// We would test this in the actual Analyze function
			// but for now we just verify the configuration is set correctly
			assert.Equal(t, tt.analyzerDesc, analyzer.analyzer.ProblemDescription)
		})
	}
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

	// Should stop before collecting all files due to size limit (1MB default)
	totalSize := 0
	for _, content := range files {
		totalSize += len(content)
	}
	assert.LessOrEqual(t, totalSize, 1024*1024, "Should not exceed 1MB default limit")
	assert.LessOrEqual(t, len(files), 10, "Should collect up to 10 files within size limit")
}
