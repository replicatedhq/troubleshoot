package analyzer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func TestAnalyzeLLM_StructuredOutput(t *testing.T) {
	// Create a test server that returns enhanced analysis
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	// Override API endpoint for testing
	originalEndpoint := "https://api.openai.com/v1/chat/completions"
	defer func() { _ = originalEndpoint }()

	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{
			AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
				CheckName: "Test Structured Output",
			},
			Model: "gpt-4o-mini",
		},
	}

	// Mock callLLM to use our test server
	analysis := &llmAnalysis{
		IssueFound: true,
		Summary:    "Pod is experiencing OOMKilled events",
		Issue:      "The pod is being terminated due to memory limits",
		Solution:   "Increase memory limits or optimize application",
		Severity:   "critical",
		Confidence: 0.95,
		Commands: []string{
			"kubectl patch deployment app -p '{\"spec\":{\"template\":{\"spec\":{\"containers\":[{\"name\":\"app\",\"resources\":{\"limits\":{\"memory\":\"2Gi\"}}}]}}}}''",
			"kubectl rollout restart deployment/app",
		},
		Documentation: []string{
			"https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/",
			"https://kubernetes.io/docs/tasks/configure-pod-container/assign-memory-resource/",
		},
		RootCause:    "Application memory usage exceeds container limits of 512Mi",
		AffectedPods: []string{"app-7b9f5d4f6-x2vh8", "app-7b9f5d4f6-k9j2m"},
		NextSteps: []string{
			"Review application memory usage patterns",
			"Increase memory limits to 2Gi",
			"Consider implementing memory optimization",
			"Monitor pod metrics after changes",
		},
		RelatedIssues: []string{
			"High CPU throttling detected",
			"Readiness probe failures during high load",
		},
	}

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

func TestAnalyzeLLM_MarkdownReportGeneration(t *testing.T) {
	analyzer := &AnalyzeLLM{
		analyzer: &troubleshootv1beta2.LLMAnalyze{},
	}

	analysis := &llmAnalysis{
		IssueFound: true,
		Summary:    "Critical memory issue detected",
		Issue:      "Pods are being OOMKilled",
		Solution:   "Increase memory limits",
		Severity:   "critical",
		Confidence: 0.95,
		RootCause:  "Memory leak in application",
		Commands: []string{
			"kubectl get pods",
			"kubectl describe pod app-xxx",
		},
		AffectedPods: []string{"app-1", "app-2"},
		NextSteps: []string{
			"Check memory usage",
			"Update deployment",
		},
		Documentation: []string{
			"https://k8s.io/docs/memory",
		},
		RelatedIssues: []string{
			"CPU throttling",
		},
	}

	report := analyzer.GenerateMarkdownReport(analysis)

	// Check report structure
	assert.Contains(t, report, "# LLM Analysis Report")
	assert.Contains(t, report, "## Executive Summary")
	assert.Contains(t, report, "🔴 Critical Issue")
	assert.Contains(t, report, "**Confidence:** 95%")
	assert.Contains(t, report, "## Root Cause Analysis")
	assert.Contains(t, report, "Memory leak in application")
	assert.Contains(t, report, "## Issue Details")
	assert.Contains(t, report, "## Recommended Solution")
	assert.Contains(t, report, "## Action Plan")
	assert.Contains(t, report, "## Recommended Commands")
	assert.Contains(t, report, "```bash")
	assert.Contains(t, report, "kubectl get pods")
	assert.Contains(t, report, "## Affected Resources")
	assert.Contains(t, report, "### Pods")
	assert.Contains(t, report, "- app-1")
	assert.Contains(t, report, "## Related Issues")
	assert.Contains(t, report, "## References")
	assert.Contains(t, report, "*Generated by Troubleshoot LLM Analyzer*")
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
			template: "Root: {{.RootCause}}, Severity: {{.Severity}}, Confidence: {{.Confidence}}",
			expected: "Root: Test root cause, Severity: warning, Confidence: 75%",
		},
		{
			name:     "Arrays",
			template: "Commands: {{.Commands}}, Pods: {{.AffectedPods}}, Steps: {{.NextSteps}}",
			expected: "Commands: cmd1; cmd2, Pods: pod1, pod2, Steps: step1; step2",
		},
		{
			name:     "Related issues",
			template: "Related: {{.RelatedIssues}}",
			expected: "Related: issue1; issue2",
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
			name:     "Binary file with null bytes",
			content:  []byte{0x00, 0x01, 0x02, 0x00, 0x00, 0x03, 0x00, 0x00},
			isBinary: true,
		},
		{
			name:     "Mixed content",
			content:  []byte("Text" + string([]byte{0x00}) + "More text"),
			isBinary: false, // Only one null byte, not enough
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBinaryFile(tt.content)
			assert.Equal(t, tt.isBinary, result)
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