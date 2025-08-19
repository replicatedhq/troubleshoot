package analyzer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func TestAnalyzeLLM_StructuredOutput(t *testing.T) {
	// Track if server was called and validate request
	serverCalled := false
	structuredOutputRequested := false
	
	// Create a test server that validates structured output request and returns enhanced analysis
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		
		// Validate the request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-api-key")
		
		// Decode and validate the request body
		var req openAIRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)
		
		// Check for structured output configuration
		if req.ResponseFormat != nil {
			structuredOutputRequested = true
			assert.Equal(t, "json_schema", req.ResponseFormat.Type)
			assert.NotNil(t, req.ResponseFormat.JSONSchema)
			assert.Equal(t, "kubernetes_analysis", req.ResponseFormat.JSONSchema.Name)
			assert.True(t, req.ResponseFormat.JSONSchema.Strict)
			
			// Validate schema contains all enhanced fields
			schema := req.ResponseFormat.JSONSchema.Schema
			properties := schema["properties"].(map[string]interface{})
			
			// Check all enhanced fields are in schema
			enhancedFields := []string{
				"commands", "documentation", "root_cause", 
				"affected_pods", "next_steps", "related_issues",
			}
			for _, field := range enhancedFields {
				assert.Contains(t, properties, field, "Schema missing field: %s", field)
			}
		}
		
		// Return comprehensive response with all enhanced fields
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
							"solution": "Increase memory limits or optimize application",
							"severity": "critical",
							"confidence": 0.95,
							"commands": [
								"kubectl patch deployment app -p '{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"app\",\"resources\":{\"limits\":{\"memory\":\"2Gi\"}}}]}}}}'",
								"kubectl rollout restart deployment/app"
							],
							"documentation": [
								"https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/",
								"https://kubernetes.io/docs/tasks/configure-pod-container/assign-memory-resource/"
							],
							"root_cause": "Application memory usage exceeds container limits of 512Mi",
							"affected_pods": ["app-7b9f5d4f6-x2vh8", "app-7b9f5d4f6-k9j2m"],
							"next_steps": [
								"Review application memory usage patterns",
								"Increase memory limits to 2Gi",
								"Consider implementing memory optimization",
								"Monitor pod metrics after changes"
							],
							"related_issues": [
								"High CPU throttling detected",
								"Readiness probe failures during high load"
							]
						}`,
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create analyzer with structured output enabled and test server endpoint
	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
				CheckName: "Test Structured Output",
			},
			Model:               "gpt-4o-mini",
			UseStructuredOutput: true, // Enable structured output
			APIEndpoint:         server.URL + "/v1/chat/completions", // USE TEST SERVER!
		},
	}

	// Test input files
	files := map[string]string{
		"pod.log": "2024-01-10 OOMKilled: Container exceeded memory limit",
	}
	
	// ACTUALLY CALL callLLM to test the full structured output flow
	analysis, err := analyzer.callLLM("test-api-key", "Pod keeps getting killed", files)
	require.NoError(t, err)
	require.NotNil(t, analysis)
	
	// Verify server was called and structured output was requested
	assert.True(t, serverCalled, "Mock server was not called")
	assert.True(t, structuredOutputRequested, "Structured output was not requested")
	
	// Verify all enhanced fields were parsed correctly
	assert.True(t, analysis.IssueFound)
	assert.Equal(t, "Pod is experiencing OOMKilled events", analysis.Summary)
	assert.Equal(t, "The pod is being terminated due to memory limits", analysis.Issue)
	assert.Equal(t, "Increase memory limits or optimize application", analysis.Solution)
	assert.Equal(t, "critical", analysis.Severity)
	assert.Equal(t, 0.95, analysis.Confidence)
	assert.Equal(t, "Application memory usage exceeds container limits of 512Mi", analysis.RootCause)
	
	// Verify arrays were parsed
	assert.Len(t, analysis.Commands, 2)
	assert.Contains(t, analysis.Commands[0], "kubectl patch deployment")
	assert.Len(t, analysis.Documentation, 2)
	assert.Len(t, analysis.AffectedPods, 2)
	assert.Equal(t, "app-7b9f5d4f6-x2vh8", analysis.AffectedPods[0])
	assert.Len(t, analysis.NextSteps, 4)
	assert.Len(t, analysis.RelatedIssues, 2)
	
	// Now test mapToOutcomes with the parsed analysis
	results := analyzer.mapToOutcomes(analysis)
	require.Len(t, results, 1)

	result := results[0]
	assert.True(t, result.IsFail)
	assert.Contains(t, result.Message, "Pod is experiencing OOMKilled events")
	assert.Contains(t, result.Message, "Root Cause: Application memory usage exceeds container limits")
	assert.Contains(t, result.Message, "kubectl patch deployment")
	assert.Contains(t, result.Message, "Affected Pods: app-7b9f5d4f6-x2vh8, app-7b9f5d4f6-k9j2m")
	assert.Contains(t, result.Message, "Next Steps:")
	assert.Contains(t, result.Message, "1. Review application memory usage patterns")
}

func TestAnalyzeLLM_StructuredOutput_DisabledFallback(t *testing.T) {
	// Test that without structured output, the system still works with prompt-based JSON
	serverCalled := false
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		
		var req openAIRequest
		json.NewDecoder(r.Body).Decode(&req)
		
		// When structured output is disabled, there should be no ResponseFormat
		assert.Nil(t, req.ResponseFormat, "ResponseFormat should be nil when structured output is disabled")
		
		// But the prompt should contain JSON instructions
		assert.Contains(t, req.Messages[0].Content, "JSON")
		assert.Contains(t, req.Messages[0].Content, "issue_found")
		assert.Contains(t, req.Messages[0].Content, "confidence")
		
		// Return same response
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
							"summary": "Database connection failure",
							"issue": "Cannot connect to PostgreSQL",
							"solution": "Check database credentials and network connectivity",
							"severity": "critical",
							"confidence": 0.9,
							"commands": ["kubectl get pods -n database"],
							"documentation": [],
							"root_cause": "Invalid credentials in ConfigMap",
							"affected_pods": ["api-server-abc123"],
							"next_steps": ["Verify database is running", "Check credentials"],
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

	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
				CheckName: "Test Without Structured Output",
			},
			Model:               "gpt-3.5-turbo", // This model doesn't support structured output
			UseStructuredOutput: false, // Explicitly disable
			APIEndpoint:         server.URL + "/v1/chat/completions",
		},
	}

	files := map[string]string{
		"app.log": "Error: connect ECONNREFUSED database:5432",
	}
	
	// Call should still work without structured output
	analysis, err := analyzer.callLLM("test-api-key", "Database connection issue", files)
	require.NoError(t, err)
	require.NotNil(t, analysis)
	assert.True(t, serverCalled)
	
	// Verify parsing still works
	assert.True(t, analysis.IssueFound)
	assert.Equal(t, "Database connection failure", analysis.Summary)
	assert.Equal(t, "critical", analysis.Severity)
	assert.Equal(t, 0.9, analysis.Confidence)
	assert.Len(t, analysis.Commands, 1)
	assert.Len(t, analysis.NextSteps, 2)
}

func TestAnalyzeLLM_MarkdownReportGeneration(t *testing.T) {
	// Track if server was called
	serverCalled := false
	
	// Create a mock server that returns a comprehensive analysis
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverCalled = true
		
		// Validate request
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/chat/completions", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "Bearer test-api-key")
		
		var req openAIRequest
		json.NewDecoder(r.Body).Decode(&req)
		
		// Check that the problem description made it through
		userContent := req.Messages[1].Content
		assert.Contains(t, userContent, "Memory leak investigation")
		assert.Contains(t, userContent, "=== logs/app.log ===")
		assert.Contains(t, userContent, "java.lang.OutOfMemoryError")
		
		// Return comprehensive analysis for markdown report
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
							"summary": "Critical memory leak detected in Java application",
							"issue": "The application is experiencing OutOfMemoryError due to heap space exhaustion",
							"solution": "Increase JVM heap size with -Xmx flag or fix the memory leak in the code",
							"severity": "critical",
							"confidence": 0.95,
							"root_cause": "Memory leak caused by unclosed resources in database connection pool",
							"commands": [
								"kubectl get pods -o wide",
								"kubectl describe pod app-xxx",
								"kubectl logs app-xxx --tail=100",
								"kubectl top pods"
							],
							"documentation": [
								"https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/",
								"https://docs.oracle.com/javase/8/docs/technotes/guides/troubleshoot/memleaks.html"
							],
							"affected_pods": ["app-prod-7b9f5d4f6-x2vh8", "app-prod-7b9f5d4f6-k9j2m"],
							"next_steps": [
								"Analyze heap dump to identify memory leak source",
								"Increase JVM heap size to 2GB as temporary fix",
								"Review database connection pool configuration",
								"Implement proper resource cleanup in finally blocks",
								"Add memory monitoring and alerting"
							],
							"related_issues": [
								"High GC pause times observed",
								"Database connection pool exhaustion",
								"Increased response latency during peak hours"
							]
						}`,
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
	
	// Create analyzer with mock server endpoint and config
	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
				CheckName: "Memory Leak Analysis",
			},
			CollectorName:      "logs",
			FileName:           "*.log",
			Model:              "gpt-4o-mini",
			ProblemDescription: "Memory leak investigation",
			APIEndpoint:        server.URL + "/v1/chat/completions", // USE TEST SERVER!
			UseStructuredOutput: true,
		},
	}

	// Mock file access
	getFile := func(path string) ([]byte, error) {
		return []byte("test content"), nil
	}
	
	findFiles := func(pattern string, excludes []string) (map[string][]byte, error) {
		if pattern == "logs/*.log" {
			return map[string][]byte{
				"logs/app.log": []byte(`
					2024-01-10 10:00:00 INFO Application started
					2024-01-10 10:15:00 WARN Memory usage at 80%
					2024-01-10 10:30:00 ERROR java.lang.OutOfMemoryError: Java heap space
					2024-01-10 10:30:01 ERROR at com.app.DatabasePool.getConnection(DatabasePool.java:45)
					2024-01-10 10:30:02 FATAL Application crashed due to memory exhaustion
				`),
			}, nil
		}
		return map[string][]byte{}, nil
	}

	// ACTUALLY CALL ANALYZE TO GET REAL RESULTS
	results, err := analyzer.Analyze(getFile, findFiles)
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.True(t, serverCalled, "Mock server was not called")
	
	// The result should be a fail with our analysis
	result := results[0]
	assert.True(t, result.IsFail)
	assert.Contains(t, result.Message, "memory leak")
	
	// Now get the analysis and generate the markdown report
	// We need to call callLLM directly to get the analysis object
	files, err := analyzer.collectFiles(getFile, findFiles)
	require.NoError(t, err)
	
	analysis, err := analyzer.callLLM("test-api-key", "Memory leak investigation", files)
	require.NoError(t, err)
	require.NotNil(t, analysis)
	
	// NOW TEST THE ACTUAL MARKDOWN GENERATION WITH REAL DATA
	report := analyzer.GenerateMarkdownReport(analysis)
	
	// Verify the markdown report contains all the expected sections with REAL data
	assert.Contains(t, report, "# LLM Analysis Report")
	assert.Contains(t, report, "## Executive Summary")
	assert.Contains(t, report, "🔴 Critical Issue") // Because severity is "critical"
	assert.Contains(t, report, "**Confidence:** 95%") // From our mock response
	assert.Contains(t, report, "Critical memory leak detected in Java application") // Real summary
	
	// Verify all sections that should be present for an issue
	assert.Contains(t, report, "## Root Cause Analysis")
	assert.Contains(t, report, "Memory leak caused by unclosed resources in database connection pool")
	
	assert.Contains(t, report, "## Issue Details")
	assert.Contains(t, report, "OutOfMemoryError due to heap space exhaustion")
	
	assert.Contains(t, report, "## Recommended Solution")
	assert.Contains(t, report, "Increase JVM heap size with -Xmx flag")
	
	assert.Contains(t, report, "## Action Plan")
	assert.Contains(t, report, "1. Analyze heap dump to identify memory leak source")
	assert.Contains(t, report, "2. Increase JVM heap size to 2GB as temporary fix")
	assert.Contains(t, report, "3. Review database connection pool configuration")
	assert.Contains(t, report, "4. Implement proper resource cleanup in finally blocks")
	assert.Contains(t, report, "5. Add memory monitoring and alerting")
	
	assert.Contains(t, report, "## Recommended Commands")
	assert.Contains(t, report, "```bash")
	assert.Contains(t, report, "kubectl get pods -o wide")
	assert.Contains(t, report, "kubectl describe pod app-xxx")
	assert.Contains(t, report, "kubectl logs app-xxx --tail=100")
	assert.Contains(t, report, "kubectl top pods")
	assert.Contains(t, report, "```")
	
	assert.Contains(t, report, "## Affected Resources")
	assert.Contains(t, report, "### Pods")
	assert.Contains(t, report, "- app-prod-7b9f5d4f6-x2vh8")
	assert.Contains(t, report, "- app-prod-7b9f5d4f6-k9j2m")
	
	assert.Contains(t, report, "## Related Issues")
	assert.Contains(t, report, "- High GC pause times observed")
	assert.Contains(t, report, "- Database connection pool exhaustion")
	assert.Contains(t, report, "- Increased response latency during peak hours")
	
	assert.Contains(t, report, "## References")
	assert.Contains(t, report, "https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/")
	assert.Contains(t, report, "https://docs.oracle.com/javase/8/docs/technotes/guides/troubleshoot/memleaks.html")
	
	assert.Contains(t, report, "*Generated by Troubleshoot LLM Analyzer*")
	
	// Test that report generation works for "no issues" case too
	noIssueAnalysis := &llmAnalysis{
		IssueFound: false,
		Summary:    "All systems operating normally",
		Confidence: 0.9,
	}
	
	noIssueReport := analyzer.GenerateMarkdownReport(noIssueAnalysis)
	assert.Contains(t, noIssueReport, "✅ No Issues Found")
	assert.Contains(t, noIssueReport, "All systems operating normally")
	assert.NotContains(t, noIssueReport, "## Root Cause") // Should not have these sections
	assert.NotContains(t, noIssueReport, "## Issue Details")
}

func TestAnalyzeLLM_TemplateVariableReplacement(t *testing.T) {
	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{},
	}

	analysis := &llmAnalysis{
		Summary:      "Test summary",
		Issue:        "Test issue",
		Solution:     "Test solution",
		RootCause:    "Test root cause",
		Severity:     "warning",
		Confidence:   0.75,
		Commands:     []string{"cmd1", "cmd2"},
		AffectedPods: []string{"pod1", "pod2"},
		NextSteps:    []string{"step1", "step2"},
		RelatedIssues: []string{"issue1", "issue2"},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{
			name:     "Basic fields",
			template: "Summary: {{.Summary}}, Issue: {{.Issue}}, Solution: {{.Solution}}",
			expected: "Summary: Test summary, Issue: Test issue, Solution: Test solution",
		},
		{
			name:     "Root cause and severity",
			template: "Root: {{.RootCause}}, Severity: {{.Severity}}, Confidence: {{.ConfidencePercent}}",
			expected: "Root: Test root cause, Severity: warning, Confidence: 75%",
		},
		{
			name:     "Arrays with helper methods",
			template: "Commands: {{.CommandsList}}, Pods: {{.AffectedPodsList}}, Steps: {{.NextStepsList}}",
			expected: "Commands: cmd1; cmd2, Pods: pod1, pod2, Steps: step1; step2",
		},
		{
			name:     "Related issues",
			template: "Related: {{.RelatedIssuesList}}",
			expected: "Related: issue1; issue2",
		},
		{
			name:     "Template with conditions",
			template: "{{if .IssueFound}}Issue found: {{.Summary}}{{else}}No issues{{end}}",
			expected: "No issues", // IssueFound defaults to false
		},
		{
			name:     "Template with range",
			template: "Commands: {{range .Commands}}{{.}} {{end}}",
			expected: "Commands: cmd1 cmd2 ",
		},
		{
			name:     "Invalid template falls back to original",
			template: "{{.InvalidField}}",
			expected: "{{.InvalidField}}", // Should return original on error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analyzer.replaceTemplateVars(tt.template, analysis)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAnalyzeLLM_SmartFileSelection(t *testing.T) {
	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			PriorityPatterns: []string{"error", "fatal", "OOM"},
			SkipPatterns:     []string{"*.png", "*.jpg", "debug-*"},
			PreferRecent:     true,
		},
	}

	tests := []struct {
		name     string
		filePath string
		content  string
		skip     bool
		minScore int
	}{
		{
			name:     "High priority error log",
			filePath: "pod-logs/app-error.log",
			content:  "ERROR: Out of memory error\nFATAL: Application crashed\nOOMKilled",
			skip:     false,
			minScore: 20, // Should have high score
		},
		{
			name:     "Normal log file",
			filePath: "pod-logs/app.log",
			content:  "INFO: Application started\nDEBUG: Processing request",
			skip:     false,
			minScore: 10, // Should have moderate score (log file bonus)
		},
		{
			name:     "Image file should be skipped",
			filePath: "screenshots/error.png",
			content:  "binary content",
			skip:     true,
			minScore: 0,
		},
		{
			name:     "Debug file should be skipped",
			filePath: "debug-trace.log",
			content:  "trace information",
			skip:     true,
			minScore: 0,
		},
		{
			name:     "JSON with errors",
			filePath: "events/pod-events.json",
			content:  `{"message": "error: container crashed", "reason": "OOMKilled"}`,
			skip:     false,
			minScore: 9, // JSON bonus (5) + error keyword (2) + OOM keyword (2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldSkip := analyzer.shouldSkipFile(tt.filePath, analyzer.analyzer.SkipPatterns)
			assert.Equal(t, tt.skip, shouldSkip, "skip mismatch for %s", tt.filePath)

			if !tt.skip {
				score := analyzer.fileScore(tt.filePath, []byte(tt.content), analyzer.analyzer.PriorityPatterns)
				assert.GreaterOrEqual(t, score, tt.minScore, "score too low for %s", tt.filePath)
			}
		})
	}
}

func TestAnalyzeLLM_BinaryFileDetection(t *testing.T) {
	tests := []struct {
		name     string
		content  []byte
		isBinary bool
	}{
		{
			name:     "Text file",
			content:  []byte("This is a normal text file with logs"),
			isBinary: false,
		},
		{
			name:     "JSON file",
			content:  []byte(`{"key": "value", "nested": {"field": 123}}`),
			isBinary: false,
		},
		{
			name:     "XML file",
			content:  []byte(`<?xml version="1.0"?><root><item>value</item></root>`),
			isBinary: false,
		},
		{
			name:     "PNG file header",
			content:  []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			isBinary: true,
		},
		{
			name:     "JPEG file header",
			content:  []byte{0xFF, 0xD8, 0xFF, 0xE0},
			isBinary: true,
		},
		{
			name:     "Binary file with null bytes",
			content:  []byte{0x00, 0x01, 0x02, 0x00, 0x00, 0x03, 0x00, 0x00},
			isBinary: true,
		},
		{
			name:     "Mixed content with single null",
			content:  []byte("Text" + string([]byte{0x00}) + "More text"),
			isBinary: false, // Text with minimal null bytes
		},
		{
			name:     "Empty file",
			content:  []byte{},
			isBinary: false,
		},
		{
			name: "Binary with many nulls",
			content: func() []byte {
				b := make([]byte, 512)
				for i := 0; i < 100; i++ {
					b[i*5] = 0x00
				}
				return b
			}(),
			isBinary: true,
		},
		{
			name:     "HTML file",
			content:  []byte(`<!DOCTYPE html><html><body>Hello</body></html>`),
			isBinary: false,
		},
		{
			name:     "ZIP file header",
			content:  []byte{0x50, 0x4B, 0x03, 0x04},
			isBinary: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBinaryFile(tt.content)
			assert.Equal(t, tt.isBinary, result, "Detection mismatch for %s", tt.name)
		})
	}
}

func TestAnalyzeLLM_FileScoring(t *testing.T) {
	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{},
	}

	priorityPatterns := []string{"error", "fatal", "exception", "OOM"}

	tests := []struct {
		name            string
		filePath        string
		content         string
		expectedMinScore int
		description     string
	}{
		{
			name:            "High priority error log",
			filePath:        "logs/app-error.log",
			content:         "ERROR occurred\nFATAL exception\nERROR again\nOOMKilled",
			expectedMinScore: 25,
			description:     "Multiple error keywords + .log extension",
		},
		{
			name:            "JSON with single error",
			filePath:        "events.json",
			content:         `{"error": "something went wrong"}`,
			expectedMinScore: 7,
			description:     "One error keyword + .json extension",
		},
		{
			name:            "Regular log no errors",
			filePath:        "app.log",
			content:         "INFO: Starting application\nDEBUG: Connected to database",
			expectedMinScore: 10,
			description:     "Just .log extension bonus",
		},
		{
			name:            "Error in filename",
			filePath:        "error-report.txt",
			content:         "Some regular content",
			expectedMinScore: 5,
			description:     "Error in filename only",
		},
		{
			name:            "Recent timestamp",
			filePath:        "recent.log",
			content:         "2024-12-01 Application started",
			expectedMinScore: 13,
			description:     ".log extension + recent timestamp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := analyzer.fileScore(tt.filePath, []byte(tt.content), priorityPatterns)
			assert.GreaterOrEqual(t, score, tt.expectedMinScore, 
				"Score too low for %s. Expected >= %d, got %d. (%s)",
				tt.filePath, tt.expectedMinScore, score, tt.description)
		})
	}
}

func TestAnalyzeLLM_CollectFilesWithSmartSelection(t *testing.T) {
	// Mock file system
	files := map[string][]byte{
		"logs/error.log":       []byte("ERROR: Critical failure\nFATAL: System crashed"),
		"logs/info.log":        []byte("INFO: System running normally"),
		"images/screenshot.png": []byte{0xFF, 0xD8, 0xFF, 0xE0}, // JPEG header
		"debug-trace.log":      []byte("Debug information"),
		"events/crash.json":    []byte(`{"reason": "OOMKilled"}`),
	}

	getFile := func(path string) ([]byte, error) {
		if content, ok := files[path]; ok {
			return content, nil
		}
		return nil, nil
	}

	findFiles := func(pattern string, excluded []string) (map[string][]byte, error) {
		// Simple pattern matching for test
		result := make(map[string][]byte)
		for path, content := range files {
			if strings.Contains(pattern, "*") || strings.Contains(path, pattern) {
				result[path] = content
			}
		}
		return result, nil
	}

	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			FileName:         "*",
			PriorityPatterns: []string{"error", "fatal", "OOM"},
			SkipPatterns:     []string{"*.png", "debug-*"},
			MaxFiles:         3,
		},
	}

	collectedFiles, err := analyzer.collectFiles(getFile, findFiles)
	require.NoError(t, err)

	// Should skip screenshot.png and debug-trace.log
	assert.NotContains(t, collectedFiles, "images/screenshot.png")
	assert.NotContains(t, collectedFiles, "debug-trace.log")

	// Should prioritize error.log and crash.json
	assert.Contains(t, collectedFiles, "logs/error.log")
	assert.Contains(t, collectedFiles, "events/crash.json")

	// Should have at most MaxFiles
	assert.LessOrEqual(t, len(collectedFiles), 3)
}

func TestAnalyzeLLM_StructuredOutputSchema(t *testing.T) {
	// Test the JSON schema generation
	schema := buildAnalysisSchema()
	
	// Verify schema structure
	assert.Equal(t, "object", schema["type"])
	
	properties, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok, "properties should be a map")
	
	// Check required fields are in schema
	requiredFields := []string{
		"issue_found", "summary", "issue", "solution", 
		"severity", "confidence", "commands", "documentation",
		"root_cause", "affected_pods", "next_steps", "related_issues",
	}
	
	for _, field := range requiredFields {
		assert.Contains(t, properties, field, "Schema should contain field: %s", field)
	}
	
	// Check severity enum
	severityProp := properties["severity"].(map[string]interface{})
	severityEnum := severityProp["enum"].([]string)
	assert.Equal(t, []string{"critical", "warning", "info"}, severityEnum)
	
	// Check confidence constraints
	confidenceProp := properties["confidence"].(map[string]interface{})
	assert.Equal(t, 0.0, confidenceProp["minimum"])
	assert.Equal(t, 1.0, confidenceProp["maximum"])
	
	// Check required fields list
	required := schema["required"].([]string)
	assert.Contains(t, required, "issue_found")
	assert.Contains(t, required, "summary")
	assert.Contains(t, required, "severity")
	assert.Contains(t, required, "confidence")
	
	// Check additionalProperties is false
	assert.Equal(t, false, schema["additionalProperties"])
}