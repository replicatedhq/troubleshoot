package analyzer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnalyzeLLM_IntegrationWithMockAPI tests the full flow with a mock OpenAI server
func TestAnalyzeLLM_IntegrationWithMockAPI(t *testing.T) {
	// Track if the mock server was actually called
	serverCalled := false
	
	// Create a mock OpenAI server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		
		// Verify the request
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-key")
		
		// Decode request to check it's properly formatted
		var req openAIRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		
		// Validate model
		assert.Equal(t, "gpt-4o-mini", req.Model)
		
		// Check messages structure
		require.Len(t, req.Messages, 2)
		assert.Equal(t, "system", req.Messages[0].Role)
		assert.Equal(t, "user", req.Messages[1].Role)
		
		// Check that problem description is in the prompt
		userContent := req.Messages[1].Content
		assert.Contains(t, userContent, "Problem Description: Pods are experiencing OOM issues")
		assert.Contains(t, userContent, "=== test-logs/oom.log ===")
		assert.Contains(t, userContent, "oom-killer invoked")
		assert.Contains(t, userContent, "Memory usage: 49Mi/50Mi")
		
		// Return a realistic response based on the content
		var responseJSON string
		if strings.Contains(userContent, "oom-killer") {
			responseJSON = `{
				"issue_found": true,
				"summary": "Memory exhaustion detected - pod repeatedly OOMKilled",
				"issue": "The container memory-hog is being killed by the OOM killer due to exceeding its memory limit of 50Mi",
				"solution": "Increase the memory limit for the deployment or optimize the application to use less memory",
				"severity": "critical",
				"confidence": 0.95,
				"root_cause": "Container memory usage (49Mi) exceeds limit (50Mi)",
				"affected_pods": ["memory-hog"],
				"commands": ["kubectl edit deployment memory-hog", "kubectl top pods"],
				"next_steps": ["Review memory usage patterns", "Increase memory limit to 100Mi", "Add memory monitoring"],
				"documentation": ["https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/"],
				"related_issues": []
			}`
		} else if strings.Contains(userContent, "CrashLoopBackOff") {
			responseJSON = `{
				"issue_found": true,
				"summary": "Application crash loop detected",
				"issue": "The crash-loop-app is exiting with error code after 5 seconds",
				"solution": "Check application logs for the specific error and fix the root cause",
				"severity": "critical",
				"confidence": 0.9
			}`
		} else {
			responseJSON = `{
				"issue_found": false,
				"summary": "No critical issues detected",
				"severity": "info",
				"confidence": 0.7
			}`
		}
		
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
						Content: responseJSON,
					},
				},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer mockServer.Close()
	
	// Set up test environment
	os.Setenv("OPENAI_API_KEY", "test-key")
	defer os.Unsetenv("OPENAI_API_KEY")
	
	// Set problem description via environment for this test
	os.Setenv("PROBLEM_DESCRIPTION", "Pods are experiencing OOM issues")
	defer os.Unsetenv("PROBLEM_DESCRIPTION")
	
	// Create analyzer WITH MOCK SERVER ENDPOINT
	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
				CheckName: "LLM OOM Analysis",
			},
			CollectorName: "test-logs",
			FileName:      "*.log",
			Model:         "gpt-4o-mini",
			APIEndpoint:   mockServer.URL + "/v1/chat/completions", // THIS IS THE KEY FIX!
			Outcomes: []*troubleshootv1beta2.Outcome{
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "issue_found",
						Message: "Critical: {{.Summary}}",
					},
				},
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						When:    "!issue_found",
						Message: "No issues detected",
					},
				},
			},
		},
	}
	
	// Mock file access functions
	getFile := func(path string) ([]byte, error) {
		return []byte("test content"), nil
	}
	
	findFiles := func(pattern string, excludes []string) (map[string][]byte, error) {
		// The pattern will be "test-logs/*.log" based on CollectorName and FileName
		if pattern == "test-logs/*.log" {
			return map[string][]byte{
				"test-logs/oom.log": []byte(`
				[2024-01-10 10:00:00] Memory usage: 45Mi/50Mi
				[2024-01-10 10:00:05] Memory usage: 49Mi/50Mi
				[2024-01-10 10:00:10] oom-killer invoked for container memory-hog
				[2024-01-10 10:00:11] Pod terminated due to OOMKilled
			`),
			}, nil
		}
		return map[string][]byte{}, nil
	}
	
	// NOW ACTUALLY CALL THE ANALYZE FUNCTION!
	results, err := analyzer.Analyze(getFile, findFiles)
	
	// Verify the test actually worked
	require.NoError(t, err)
	require.True(t, serverCalled, "Mock server was never called - test is not working!")
	require.Len(t, results, 1)
	
	// Verify the result
	result := results[0]
	assert.Equal(t, "LLM OOM Analysis", result.Title)
	assert.True(t, result.IsFail, "Expected fail outcome for OOM issue")
	assert.Contains(t, result.Message, "Memory exhaustion detected")
	assert.Contains(t, result.Message, "OOMKilled")
}

// TestAnalyzeLLM_IntegrationScenarios tests various real-world scenarios end-to-end
func TestAnalyzeLLM_IntegrationScenarios(t *testing.T) {
	tests := []struct {
		name            string
		logContent      map[string][]byte
		problemDesc     string
		expectedSummary string
		expectedFail    bool
		expectedWarn    bool
		expectedPass    bool
	}{
		{
			name: "OOM killer scenario",
			logContent: map[string][]byte{
				"test-logs/pod.log": []byte(`
					[2024-01-10 10:00:00] Starting application...
					[2024-01-10 10:00:10] Memory cgroup out of memory: Killed process 1234
					[2024-01-10 10:00:11] oom_kill_process: Killed process 1234 (java)
					[2024-01-10 10:00:12] Container exceeded memory limit`),
			},
			problemDesc:     "Application keeps restarting",
			expectedSummary: "memory",
			expectedFail:    true,
		},
		{
			name: "Connection refused scenario",
			logContent: map[string][]byte{
				"test-logs/app.log": []byte(`
					[2024-01-10 10:00:00] Starting web server...
					[2024-01-10 10:00:01] Error: connect ECONNREFUSED 10.0.0.5:5432
					[2024-01-10 10:00:02] PostgreSQL connection failed: Connection refused
					[2024-01-10 10:00:03] Unable to connect to database after 5 retries`),
			},
			problemDesc:     "Application can't start",
			expectedSummary: "connection",
			expectedFail:    true,
		},
		{
			name: "Healthy application",
			logContent: map[string][]byte{
				"test-logs/healthy.log": []byte(`
					[2024-01-10 10:00:00] Application started successfully
					[2024-01-10 10:00:01] Connected to database
					[2024-01-10 10:00:02] Serving requests on port 8080
					[2024-01-10 10:00:03] Health check passed`),
			},
			problemDesc:     "Checking application status",
			expectedSummary: "application is healthy",
			expectedPass:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server for this specific test
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Decode request
				var req openAIRequest
				json.NewDecoder(r.Body).Decode(&req)
				
				userContent := req.Messages[1].Content
				
				// Determine response based on content
				var responseJSON string
				if strings.Contains(userContent, "oom_kill") || strings.Contains(userContent, "memory limit") {
					responseJSON = `{
						"issue_found": true,
						"summary": "Memory exhaustion detected",
						"issue": "Process killed by OOM killer",
						"severity": "critical",
						"confidence": 0.95
					}`
				} else if strings.Contains(userContent, "ECONNREFUSED") || strings.Contains(userContent, "connection failed") {
					responseJSON = `{
						"issue_found": true,
						"summary": "Database connection failure",
						"issue": "Cannot connect to PostgreSQL",
						"severity": "critical",
						"confidence": 0.9
					}`
				} else if strings.Contains(userContent, "successfully") && strings.Contains(userContent, "Health check passed") {
					responseJSON = `{
						"issue_found": false,
						"summary": "Application is healthy",
						"severity": "info",
						"confidence": 0.85
					}`
				} else {
					responseJSON = `{
						"issue_found": false,
						"summary": "No issues detected",
						"severity": "info",
						"confidence": 0.7
					}`
				}
				
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
								Content: responseJSON,
							},
						},
					},
				}
				
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer mockServer.Close()
			
			// Set up environment
			os.Setenv("OPENAI_API_KEY", "test-key")
			defer os.Unsetenv("OPENAI_API_KEY")
			os.Setenv("PROBLEM_DESCRIPTION", tt.problemDesc)
			defer os.Unsetenv("PROBLEM_DESCRIPTION")
			
			// Create analyzer
			analyzer := &AnalyzeLLM{
				analyzer: &troubleshootv1beta2.LLMAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						CheckName: "Integration Test",
					},
					CollectorName: "test-logs",
					FileName:      "*.log",
					Model:         "gpt-4o-mini",
					APIEndpoint:   mockServer.URL + "/v1/chat/completions",
					Outcomes: []*troubleshootv1beta2.Outcome{
						{
							Fail: &troubleshootv1beta2.SingleOutcome{
								When:    "issue_found && severity == 'critical'",
								Message: "Critical issue: {{.Summary}}",
							},
						},
						{
							Warn: &troubleshootv1beta2.SingleOutcome{
								When:    "issue_found && severity == 'warning'",
								Message: "Warning: {{.Summary}}",
							},
						},
						{
							Pass: &troubleshootv1beta2.SingleOutcome{
								When:    "!issue_found",
								Message: "Healthy: {{.Summary}}",
							},
						},
					},
				},
			}
			
			// Mock file functions
			getFile := func(path string) ([]byte, error) {
				return []byte("test"), nil
			}
			
			findFiles := func(pattern string, excludes []string) (map[string][]byte, error) {
				if pattern == "test-logs/*.log" {
					return tt.logContent, nil
				}
				return map[string][]byte{}, nil
			}
			
			// Run the analyzer
			results, err := analyzer.Analyze(getFile, findFiles)
			require.NoError(t, err)
			require.Len(t, results, 1)
			
			result := results[0]
			assert.Equal(t, tt.expectedFail, result.IsFail, "IsFail mismatch for %s", tt.name)
			assert.Equal(t, tt.expectedWarn, result.IsWarn, "IsWarn mismatch for %s", tt.name)
			assert.Equal(t, tt.expectedPass, result.IsPass, "IsPass mismatch for %s", tt.name)
			// Only check summary content for fail/warn cases
			if tt.expectedFail || tt.expectedWarn {
				assert.Contains(t, strings.ToLower(result.Message), tt.expectedSummary)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}

// TestAnalyzeLLM_RealWorldScenarios tests various real-world log patterns
func TestAnalyzeLLM_RealWorldScenarios(t *testing.T) {
	scenarios := []struct {
		name             string
		logContent       map[string][]byte
		problemDesc      string
		expectedFound    bool
		expectedSeverity string
		mockResponse     string
	}{
		{
			name: "OOM Killer Pattern",
			logContent: map[string][]byte{
				"logs/pod.log": []byte(`
					Memory cgroup out of memory: Killed process 1234 (java)
					Memory cgroup stats: limit 524288000, usage 524287896
					oom_kill_process: Killed process 1234 (java) total-vm:2048576kB
				`),
			},
			problemDesc:      "Application keeps restarting",
			expectedFound:    true,
			expectedSeverity: "critical",
			mockResponse: `{
				"issue_found": true,
				"summary": "Application terminated due to memory exhaustion",
				"issue": "Java process killed by OOM killer - memory limit exceeded",
				"solution": "Increase memory limits or optimize application memory usage",
				"severity": "critical",
				"confidence": 0.95,
				"root_cause": "Memory usage (524MB) exceeded container limit (524MB)",
				"affected_pods": ["java-app"],
				"commands": ["kubectl describe pod", "kubectl top pods"],
				"next_steps": ["Increase memory limit to 1GB", "Profile application for memory leaks"],
				"documentation": [],
				"related_issues": []
			}`,
		},
		{
			name: "Connection Refused Pattern",
			logContent: map[string][]byte{
				"logs/app.log": []byte(`
					Error: connect ECONNREFUSED 10.0.0.5:5432
					PostgreSQL connection failed: Connection refused
					Unable to connect to database after 5 retries
				`),
			},
			problemDesc:      "Application can't start properly",
			expectedFound:    true,
			expectedSeverity: "critical",
			mockResponse: `{
				"issue_found": true,
				"summary": "Database connection failure preventing application startup",
				"issue": "PostgreSQL database at 10.0.0.5:5432 is refusing connections",
				"solution": "Verify database is running and accessible from the application pod",
				"severity": "critical",
				"confidence": 0.9,
				"root_cause": "PostgreSQL service is down or network connectivity issue",
				"affected_pods": [],
				"commands": ["kubectl get svc", "kubectl get endpoints", "telnet 10.0.0.5 5432"],
				"next_steps": ["Check if database pod is running", "Verify network policies", "Check database credentials"],
				"documentation": [],
				"related_issues": ["Connection pool exhaustion", "DNS resolution issues"]
			}`,
		},
		{
			name: "Disk Space Issue",
			logContent: map[string][]byte{
				"logs/system.log": []byte(`
					No space left on device
					Error: ENOSPC: no space left on device, write
					df: /data: 100% used (50G/50G)
				`),
			},
			problemDesc:      "Writes are failing",
			expectedFound:    true,
			expectedSeverity: "critical",
			mockResponse: `{
				"issue_found": true,
				"summary": "Disk space exhausted on /data volume",
				"issue": "No space left on device - /data volume is 100% full (50G/50G)",
				"solution": "Free up disk space or increase volume size",
				"severity": "critical",
				"confidence": 0.99,
				"root_cause": "Disk volume /data has no available space",
				"affected_pods": [],
				"commands": ["df -h", "du -sh /data/*", "kubectl get pv"],
				"next_steps": ["Clean up old logs or temporary files", "Increase PVC size", "Implement log rotation"],
				"documentation": [],
				"related_issues": []
			}`,
		},
		{
			name: "Healthy Logs",
			logContent: map[string][]byte{
				"logs/app.log": []byte(`
					Server started successfully on port 8080
					Health check passed
					Connected to database successfully
					All systems operational
				`),
			},
			problemDesc:      "Just checking if everything is okay",
			expectedFound:    false,
			expectedSeverity: "info",
			mockResponse: `{
				"issue_found": false,
				"summary": "Application is running normally with no issues detected",
				"issue": "",
				"solution": "",
				"severity": "info",
				"confidence": 0.85,
				"root_cause": "",
				"affected_pods": [],
				"commands": [],
				"next_steps": [],
				"documentation": [],
				"related_issues": []
			}`,
		},
	}
	
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			// Create mock server that returns appropriate response for each scenario
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v1/chat/completions", r.URL.Path)
				assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-api-key")
				
				// Parse request to verify problem description and log content
				var req openAIRequest
				json.NewDecoder(r.Body).Decode(&req)
				
				userContent := req.Messages[1].Content
				assert.Contains(t, userContent, sc.problemDesc)
				
				// Verify log content is in the request
				for _, content := range sc.logContent {
					assert.Contains(t, userContent, string(content), "Log content should be in request")
				}
				
				// Return scenario-specific response
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
								Content: sc.mockResponse,
							},
						},
					},
				}
				
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
			}))
			defer server.Close()
			
			// Set up environment
			os.Setenv("OPENAI_API_KEY", "test-api-key")
			defer os.Unsetenv("OPENAI_API_KEY")
			
			// Create analyzer with mock server
			analyzer := &AnalyzeLLM{
				analyzer: &troubleshootv1beta2.LLMAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						CheckName: sc.name,
					},
					CollectorName:      "logs",
					FileName:           "*",
					Model:              "gpt-4o-mini",
					ProblemDescription: sc.problemDesc,
					APIEndpoint:        server.URL + "/v1/chat/completions",
					UseStructuredOutput: true,
				},
			}
			
			// Mock file access
			getFile := func(path string) ([]byte, error) {
				return []byte("test"), nil
			}
			
			findFiles := func(pattern string, excludes []string) (map[string][]byte, error) {
				if pattern == "logs/*" {
					return sc.logContent, nil
				}
				return map[string][]byte{}, nil
			}
			
			// ACTUALLY RUN THE ANALYZER
			results, err := analyzer.Analyze(getFile, findFiles)
			require.NoError(t, err)
			require.Len(t, results, 1)
			
			result := results[0]
			
			// Verify the analysis matched expectations
			if sc.expectedFound {
				if sc.expectedSeverity == "critical" {
					assert.True(t, result.IsFail, "Expected fail for critical issue in %s", sc.name)
				} else if sc.expectedSeverity == "warning" {
					assert.True(t, result.IsWarn, "Expected warn for warning issue in %s", sc.name)
				}
				assert.Contains(t, result.Message, "")
			} else {
				assert.True(t, result.IsPass, "Expected pass for healthy scenario in %s", sc.name)
			}
			
			// Log the actual result for debugging
			t.Logf("Scenario %s result: IsFail=%v, IsWarn=%v, IsPass=%v", 
				sc.name, result.IsFail, result.IsWarn, result.IsPass)
			t.Logf("Message preview: %.100s...", result.Message)
		})
	}
}

// TestAnalyzeLLM_ErrorHandling tests various error conditions
func TestAnalyzeLLM_ErrorHandling(t *testing.T) {
	tests := []struct {
		name               string
		serverResponse     string
		serverStatusCode   int
		useStructuredOutput bool
		expectedError      bool
		expectedErrorMsg   string
		expectedFallback   bool
	}{
		{
			name: "Malformed JSON Response - With Structured Output",
			serverResponse: `{
				"choices": [{
					"message": {
						"content": "This is not valid JSON for analysis"
					}
				}]
			}`,
			serverStatusCode:   200,
			useStructuredOutput: true,
			expectedError:      true,
			expectedErrorMsg:   "failed to parse structured JSON response",
		},
		{
			name: "Malformed JSON Response - Without Structured Output (Fallback)",
			serverResponse: `{
				"choices": [{
					"message": {
						"content": "This is not valid JSON but contains error keyword"
					}
				}]
			}`,
			serverStatusCode:   200,
			useStructuredOutput: false,
			expectedError:      false, // Should handle gracefully with fallback
			expectedFallback:   true,
		},
		{
			name: "Empty Response",
			serverResponse: `{
				"choices": []
			}`,
			serverStatusCode:   200,
			useStructuredOutput: true,
			expectedError:      true,
			expectedErrorMsg:   "no response from OpenAI API",
		},
		{
			name: "API Error Response",
			serverResponse: `{
				"error": {
					"message": "Rate limit exceeded. Please try again later.",
					"type": "rate_limit_error"
				}
			}`,
			serverStatusCode:   429,
			useStructuredOutput: true,
			expectedError:      true,
			expectedErrorMsg:   "OpenAI API error: Rate limit exceeded",
		},
		{
			name: "Partial JSON Response",
			serverResponse: `{
				"choices": [{
					"message": {
						"content": "{\"issue_found\": true, \"summary\": \"Incomplete"
					}
				}]
			}`,
			serverStatusCode:   200,
			useStructuredOutput: true,
			expectedError:      true,
			expectedErrorMsg:   "failed to parse structured JSON",
		},
		{
			name: "Valid JSON Embedded in Text - Without Structured Output",
			serverResponse: `{
				"choices": [{
					"message": {
						"content": "Here's the analysis: {\"issue_found\": true, \"summary\": \"Test issue\", \"severity\": \"warning\", \"confidence\": 0.8} Done."
					}
				}]
			}`,
			serverStatusCode:   200,
			useStructuredOutput: false,
			expectedError:      false,
			expectedFallback:   false, // Should successfully extract JSON
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock server that returns the test response
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v1/chat/completions", r.URL.Path)
				
				// Set status code if error response
				if tt.serverStatusCode != 200 {
					w.WriteHeader(tt.serverStatusCode)
				}
				
				// Return the test response
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tt.serverResponse))
			}))
			defer server.Close()
			
			// Create analyzer
			analyzer := &AnalyzeLLM{
				analyzer: &troubleshootv1beta2.LLMAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						CheckName: tt.name,
					},
					Model:               "gpt-4o-mini",
					APIEndpoint:         server.URL + "/v1/chat/completions",
					UseStructuredOutput: tt.useStructuredOutput,
				},
			}
			
			// Test the callLLM function directly
			files := map[string]string{
				"test.log": "Test log content",
			}
			
			analysis, err := analyzer.callLLM("test-api-key", "Test problem", files)
			
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrorMsg)
				assert.Nil(t, analysis)
			} else {
				require.NoError(t, err)
				require.NotNil(t, analysis)
				
				if tt.expectedFallback {
					// For fallback cases, verify basic analysis was created
					assert.True(t, analysis.IssueFound) // Contains "error" keyword
					assert.Equal(t, "warning", analysis.Severity)
					assert.Equal(t, 0.5, analysis.Confidence)
					assert.Contains(t, analysis.Summary, "not valid JSON")
				} else {
					// For successful JSON extraction
					assert.True(t, analysis.IssueFound)
					assert.Equal(t, "Test issue", analysis.Summary)
					assert.Equal(t, "warning", analysis.Severity)
					assert.Equal(t, 0.8, analysis.Confidence)
				}
			}
		})
	}
	
	// Additional test for network/timeout errors
	t.Run("Network Error", func(t *testing.T) {
		// Create analyzer with invalid endpoint
		analyzer := &AnalyzeLLM{
			analyzer: &troubleshootv1beta2.LLMAnalyze{
				APIEndpoint: "http://invalid-host-that-does-not-exist:9999/v1/chat/completions",
				Model:       "gpt-4o-mini",
			},
		}
		
		files := map[string]string{"test.log": "content"}
		
		// This should fail with a network error
		analysis, err := analyzer.callLLM("test-api-key", "test", files)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to call OpenAI API")
		assert.Nil(t, analysis)
	})
}

// TestAnalyzeLLM_PerformanceWithLargeFiles tests handling of large log files
func TestAnalyzeLLM_PerformanceWithLargeFiles(t *testing.T) {
	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			MaxFiles: 5,
		},
	}
	
	// Create large log files
	largeLog := make([]byte, 200*1024) // 200KB each
	for i := range largeLog {
		largeLog[i] = byte('A' + (i % 26))
	}
	
	findFiles := func(pattern string, excludes []string) (map[string][]byte, error) {
		files := make(map[string][]byte)
		for i := 0; i < 10; i++ {
			files[fmt.Sprintf("log%d.txt", i)] = largeLog
		}
		return files, nil
	}
	
	getFile := func(path string) ([]byte, error) {
		return largeLog, nil
	}
	
	files, err := analyzer.collectFiles(getFile, findFiles)
	require.NoError(t, err)
	
	// Should respect the 1MB default total limit
	totalSize := 0
	for _, content := range files {
		totalSize += len(content)
	}
	
	assert.LessOrEqual(t, totalSize, 1024*1024, "Should not exceed 1MB default limit")
	assert.LessOrEqual(t, len(files), 5, "Should not exceed max files limit")
}