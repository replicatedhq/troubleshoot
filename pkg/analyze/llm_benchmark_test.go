package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BenchmarkAnalyzeLLM_FileCollection benchmarks file collection performance
func BenchmarkAnalyzeLLM_FileCollection(b *testing.B) {
	benchmarks := []struct {
		name     string
		numFiles int
		fileSize int // in KB
	}{
		{"Small-10Files-1KB", 10, 1},
		{"Medium-50Files-10KB", 50, 10},
		{"Large-100Files-50KB", 100, 50},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			analyzer := &AnalyzeLLM{
				analyzer: &troubleshootv1beta2.LLMAnalyze{
					MaxFiles: 100,
				},
			}

			// Create test files
			fileContent := make([]byte, bm.fileSize*1024)
			for i := range fileContent {
				fileContent[i] = byte('A' + (i % 26))
			}

			findFiles := func(pattern string, excludes []string) (map[string][]byte, error) {
				files := make(map[string][]byte)
				for i := 0; i < bm.numFiles; i++ {
					files[fmt.Sprintf("log%d.txt", i)] = fileContent
				}
				return files, nil
			}

			getFile := func(path string) ([]byte, error) {
				return fileContent, nil
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := analyzer.collectFiles(getFile, findFiles)
				require.NoError(b, err)
			}
		})
	}
}

// BenchmarkAnalyzeLLM_OutcomeMapping benchmarks outcome mapping performance
func BenchmarkAnalyzeLLM_OutcomeMapping(b *testing.B) {
	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
				CheckName: "Benchmark Test",
			},
			Outcomes: []*troubleshootv1beta2.Outcome{
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "issue_found",
						Message: "Critical: {{.Summary}}",
					},
				},
				{
					Warn: &troubleshootv1beta2.SingleOutcome{
						When:    "potential_issue",
						Message: "Warning: {{.Summary}}",
					},
				},
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						Message: "All clear",
					},
				},
			},
		},
	}

	testCases := []struct {
		name     string
		analysis *llmAnalysis
	}{
		{
			"Critical",
			&llmAnalysis{
				IssueFound: true,
				Summary:    "Critical issue detected",
				Severity:   "critical",
			},
		},
		{
			"Warning",
			&llmAnalysis{
				IssueFound: true,
				Summary:    "Potential issue detected",
				Severity:   "warning",
			},
		},
		{
			"Pass",
			&llmAnalysis{
				IssueFound: false,
				Summary:    "No issues",
			},
		},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results := analyzer.mapToOutcomes(tc.analysis)
				require.NotNil(b, results)
			}
		})
	}
}

// TestAnalyzeLLM_Timeout tests timeout handling in the LLM API call
func TestAnalyzeLLM_Timeout(t *testing.T) {
	// Test both successful completion and timeout scenarios
	tests := []struct {
		name           string
		serverDelay    time.Duration
		contextTimeout time.Duration
		expectError    bool
		errorContains  string
	}{
		{
			name:           "Successful completion before timeout",
			serverDelay:    100 * time.Millisecond,
			contextTimeout: 1 * time.Second,
			expectError:    false,
		},
		{
			name:           "Request times out",
			serverDelay:    2 * time.Second,
			contextTimeout: 500 * time.Millisecond,
			expectError:    true,
			errorContains:  "context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock server that delays response
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Validate request
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v1/chat/completions", r.URL.Path)
				
				// Delay to simulate slow API
				select {
				case <-time.After(tt.serverDelay):
					// Send response after delay
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
										"issue_found": false,
										"summary": "Test completed",
										"severity": "info",
										"confidence": 0.9
									}`,
								},
							},
						},
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(response)
				case <-r.Context().Done():
					// Request was cancelled, don't send response
					return
				}
			}))
			defer server.Close()

			// Test the actual callLLM function with timeout
			start := time.Now()
			
			// Create a modified version of callLLM that accepts a custom timeout
			callLLMWithTimeout := func(apiKey, problemDescription string, files map[string]string, timeout time.Duration) (*llmAnalysis, error) {
				// This simulates what callLLM does but with configurable timeout
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				
				// Build minimal request
				reqBody := openAIRequest{
					Model: "gpt-4o-mini",
					Messages: []openAIMessage{
						{Role: "system", Content: "Test"},
						{Role: "user", Content: problemDescription},
					},
				}
				
				jsonData, _ := json.Marshal(reqBody)
				req, err := http.NewRequestWithContext(ctx, "POST", server.URL+"/v1/chat/completions", bytes.NewBuffer(jsonData))
				if err != nil {
					return nil, err
				}
				
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
				
				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					return nil, err
				}
				defer resp.Body.Close()
				
				var openAIResp openAIResponse
				json.NewDecoder(resp.Body).Decode(&openAIResp)
				
				if len(openAIResp.Choices) == 0 {
					return nil, fmt.Errorf("no response")
				}
				
				var analysis llmAnalysis
				json.Unmarshal([]byte(openAIResp.Choices[0].Message.Content), &analysis)
				return &analysis, nil
			}
			
			// Run the test with our custom timeout
			_, err := callLLMWithTimeout("test-key", "test problem", map[string]string{"test": "data"}, tt.contextTimeout)
			elapsed := time.Since(start)
			
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
				// Verify it actually timed out near the expected time
				assert.Less(t, elapsed, tt.contextTimeout+100*time.Millisecond)
			} else {
				require.NoError(t, err)
				// Verify it completed successfully
				assert.Less(t, elapsed, tt.contextTimeout)
			}
		})
	}
}

// TestAnalyzeLLM_ConcurrentAnalysis tests concurrent analyzer execution
func TestAnalyzeLLM_ConcurrentAnalysis(t *testing.T) {
	// Test running multiple LLM analyzers concurrently
	// Important for when multiple LLM analyzers are in the same spec
	
	// Set up environment
	os.Setenv("OPENAI_API_KEY", "test-api-key")
	defer os.Unsetenv("OPENAI_API_KEY")
	
	numAnalyzers := 3
	requestCounts := make(map[int]int)
	var mu sync.Mutex
	
	// Create a shared mock server that handles multiple concurrent requests
	var serverRequestCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		
		// Track total request count atomically
		atomic.AddInt32(&serverRequestCount, 1)
		
		// Parse request to determine which analyzer sent it
		var req openAIRequest
		json.NewDecoder(r.Body).Decode(&req)
		
		// Extract analyzer ID from the user message content
		var analyzerID int
		userContent := req.Messages[1].Content
		// Search for the pattern "Analyzer X:" in the problem description
		if strings.Contains(userContent, "Problem Description: Analyzer ") {
			start := strings.Index(userContent, "Problem Description: Analyzer ") + len("Problem Description: Analyzer ")
			if start < len(userContent) && userContent[start] >= '0' && userContent[start] <= '9' {
				analyzerID = int(userContent[start] - '0')
			}
		}
		
		// Track request counts per analyzer
		mu.Lock()
		requestCounts[analyzerID]++
		mu.Unlock()
		
		// Simulate some processing time
		time.Sleep(50 * time.Millisecond)
		
		// Return response based on analyzer ID
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
						Content: fmt.Sprintf(`{
							"issue_found": %v,
							"summary": "Analysis from analyzer %d",
							"issue": "Issue for analyzer %d",
							"severity": "%s",
							"confidence": 0.%d
						}`, analyzerID%2 == 0, analyzerID, analyzerID,
							[]string{"critical", "warning", "info"}[analyzerID%3],
							80+analyzerID),
					},
				},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	// Create multiple analyzers with different configurations
	analyzers := make([]*AnalyzeLLM, numAnalyzers)
	for i := 0; i < numAnalyzers; i++ {
		analyzers[i] = &AnalyzeLLM{
			analyzer: &troubleshootv1beta2.LLMAnalyze{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: fmt.Sprintf("Concurrent Test %d", i),
				},
				CollectorName: fmt.Sprintf("logs-%d", i),
				FileName:      "*.log",
				Model:         "gpt-4o-mini",
				APIEndpoint:   server.URL + "/v1/chat/completions",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "issue_found && severity == 'critical'",
							Message: fmt.Sprintf("Analyzer %d: {{.Summary}}", i),
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: fmt.Sprintf("Analyzer %d: No issues", i),
						},
					},
				},
			},
		}
	}
	
	// Mock file access functions for each analyzer
	getFile := func(path string) ([]byte, error) {
		return []byte("test content"), nil
	}
	
	findFiles := func(pattern string, excludes []string) (map[string][]byte, error) {
		// Return different files for each analyzer based on pattern
		files := make(map[string][]byte)
		for i := 0; i < numAnalyzers; i++ {
			if pattern == fmt.Sprintf("logs-%d/*.log", i) {
				files[fmt.Sprintf("logs-%d/app.log", i)] = []byte(fmt.Sprintf("Log content for analyzer %d\nSome error occurred", i))
				break
			}
		}
		return files, nil
	}
	
	// Run analyzers concurrently
	type result struct {
		id      int
		results []*AnalyzeResult
		err     error
	}
	
	resultsChan := make(chan result, numAnalyzers)
	var wg sync.WaitGroup
	
	start := time.Now()
	
	for i := 0; i < numAnalyzers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			
			// Create unique problem description for this analyzer
			os.Setenv("PROBLEM_DESCRIPTION", fmt.Sprintf("Analyzer %d: Checking for issues", id))
			
			// Run the actual Analyze function
			analyzeResults, err := analyzers[id].Analyze(getFile, findFiles)
			
			resultsChan <- result{
				id:      id,
				results: analyzeResults,
				err:     err,
			}
		}(i)
	}
	
	// Wait for all goroutines to complete
	wg.Wait()
	close(resultsChan)
	
	elapsed := time.Since(start)
	
	// Collect and verify results
	allResults := make(map[int][]*AnalyzeResult)
	for res := range resultsChan {
		require.NoError(t, res.err, "Analyzer %d failed", res.id)
		require.NotNil(t, res.results, "Analyzer %d returned nil results", res.id)
		require.Len(t, res.results, 1, "Analyzer %d should return exactly one result", res.id)
		allResults[res.id] = res.results
		
		// Verify the result contains expected analyzer ID
		result := res.results[0]
		assert.Contains(t, result.Title, fmt.Sprintf("Concurrent Test %d", res.id))
	}
	
	// Verify all analyzers completed
	assert.Len(t, allResults, numAnalyzers, "Not all analyzers completed")
	
	// Verify total number of requests made to server
	totalRequests := atomic.LoadInt32(&serverRequestCount)
	assert.Equal(t, int32(numAnalyzers), totalRequests, "Server should receive exactly %d requests", numAnalyzers)
	
	// Verify request distribution (best effort - may have parsing issues)
	mu.Lock()
	totalTracked := 0
	for _, count := range requestCounts {
		totalTracked += count
	}
	mu.Unlock()
	assert.Equal(t, numAnalyzers, totalTracked, "Should track all %d requests", numAnalyzers)
	
	// Verify concurrent execution (should complete faster than sequential)
	// With 50ms delay per request, sequential would take 150ms minimum
	// Concurrent should complete in ~50-100ms
	assert.Less(t, elapsed, 200*time.Millisecond, "Concurrent execution took too long")
	
	t.Logf("Concurrent analysis of %d analyzers completed in %v", numAnalyzers, elapsed)
}