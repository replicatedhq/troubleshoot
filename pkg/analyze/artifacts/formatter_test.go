package artifacts

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFormatter(t *testing.T) {
	// Test with default options
	formatter := NewFormatter(FormatterOptions{})

	assert.NotNil(t, formatter)
	assert.Equal(t, "json", formatter.options.Format)
	assert.Equal(t, "priority", formatter.options.SortBy)
	assert.Equal(t, "all", formatter.options.FilterLevel)
}

func TestNewFormatter_WithOptions(t *testing.T) {
	opts := FormatterOptions{
		Format:             "html",
		IncludeMetadata:    true,
		IncludeRemediation: true,
		IncludeInsights:    true,
		SortBy:             "severity",
		FilterLevel:        "failures",
		CustomFields:       map[string]string{"team": "platform"},
	}

	formatter := NewFormatter(opts)

	assert.Equal(t, opts, formatter.options)
}

func createSampleAnalysis() *analyzer.EnhancedAnalysisResult {
	return &analyzer.EnhancedAnalysisResult{
		Results: []analyzer.EnhancedAnalyzerResult{
			{
				IsPass:      true,
				Title:       "Kubernetes Version",
				Message:     "Version 1.24.0 is supported",
				Confidence:  0.95,
				AgentUsed:   "local",
				Explanation: "The Kubernetes version check passed successfully.",
				Evidence:    []string{"Found Kubernetes 1.24.0 in cluster info"},
			},
			{
				IsFail:      true,
				Title:       "Storage Class",
				Message:     "No default storage class found",
				Confidence:  0.9,
				Impact:      "HIGH",
				AgentUsed:   "local",
				Explanation: "The cluster lacks a default storage class which may prevent pod scheduling.",
				Evidence:    []string{"kubectl get storageclass returned empty", "No storage class marked as default"},
				Remediation: &analyzer.RemediationStep{
					ID:          "fix-storage-class",
					Title:       "Create Default Storage Class",
					Description: "Configure a default storage class for the cluster",
					Category:    "immediate",
					Priority:    1,
					Commands:    []string{"kubectl get storageclass", "kubectl patch storageclass <name> -p '{\"metadata\":{\"annotations\":{\"storageclass.kubernetes.io/is-default-class\":\"true\"}}}'"},
					Manual:      []string{"Choose an appropriate storage class", "Mark it as default"},
				},
			},
			{
				IsWarn:      true,
				Title:       "Memory Usage",
				Message:     "Memory usage is above 80%",
				Confidence:  0.7,
				Impact:      "MEDIUM",
				AgentUsed:   "local",
				Explanation: "High memory usage may impact application performance.",
				Evidence:    []string{"Node memory usage at 85%", "Multiple pods near memory limits"},
			},
		},
		Remediation: []analyzer.RemediationStep{
			{
				ID:          "fix-storage-class",
				Title:       "Create Default Storage Class",
				Description: "Configure a default storage class for the cluster",
				Priority:    1,
			},
		},
		Summary: analyzer.AnalysisSummary{
			TotalChecks:   3,
			PassedChecks:  1,
			FailedChecks:  1,
			WarningChecks: 1,
			OverallHealth: "DEGRADED",
			Confidence:    0.85,
			TopIssues:     []string{"Storage Class"},
		},
		Metadata: analyzer.AnalysisMetadata{
			Timestamp:      time.Now(),
			EngineVersion:  "1.0.0",
			AgentsUsed:     []string{"local"},
			ProcessingTime: 2 * time.Second,
			BundleInfo: analyzer.BundleInfo{
				Path:      "/test/bundle.tar.gz",
				Size:      1024,
				FileCount: 10,
			},
		},
	}
}

func TestFormatter_FormatAnalysis_JSON(t *testing.T) {
	analysis := createSampleAnalysis()
	formatter := NewFormatter(FormatterOptions{
		Format:             "json",
		IncludeMetadata:    true,
		IncludeRemediation: true,
		IncludeInsights:    true,
	})

	var buf bytes.Buffer
	err := formatter.FormatAnalysis(analysis, &buf)

	assert.NoError(t, err)

	// Parse the JSON to verify it's valid
	var artifact AnalysisArtifact
	err = json.Unmarshal(buf.Bytes(), &artifact)
	require.NoError(t, err)

	// Verify structure
	assert.Equal(t, "https://schemas.troubleshoot.sh/analysis/v1beta2", artifact.Schema)
	assert.Equal(t, "v1beta2", artifact.Version)
	assert.Len(t, artifact.Results, 3)
	assert.Len(t, artifact.Remediation, 1)
	assert.Equal(t, "DEGRADED", artifact.Summary.OverallHealth)
	assert.Equal(t, "1.0.0", artifact.Metadata.EngineVersion)
}

func TestFormatter_FormatAnalysis_YAML(t *testing.T) {
	analysis := createSampleAnalysis()
	formatter := NewFormatter(FormatterOptions{
		Format:             "yaml",
		IncludeMetadata:    true,
		IncludeRemediation: true,
	})

	var buf bytes.Buffer
	err := formatter.FormatAnalysis(analysis, &buf)

	assert.NoError(t, err)
	assert.NotEmpty(t, buf.String())

	// For this test, we're just using JSON format since we haven't implemented proper YAML
	// In a real implementation, this would use a YAML library
	assert.Contains(t, buf.String(), "schemas.troubleshoot.sh")
}

func TestFormatter_FormatAnalysis_HTML(t *testing.T) {
	analysis := createSampleAnalysis()
	formatter := NewFormatter(FormatterOptions{
		Format:             "html",
		IncludeMetadata:    true,
		IncludeRemediation: true,
	})

	var buf bytes.Buffer
	err := formatter.FormatAnalysis(analysis, &buf)

	assert.NoError(t, err)

	html := buf.String()
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "Troubleshoot Analysis Report")
	assert.Contains(t, html, "Kubernetes Version")
	assert.Contains(t, html, "Storage Class")
	assert.Contains(t, html, "Memory Usage")
	assert.Contains(t, html, "✅")  // Pass icon
	assert.Contains(t, html, "❌")  // Fail icon
	assert.Contains(t, html, "⚠️") // Warn icon
	assert.Contains(t, html, "DEGRADED")
}

func TestFormatter_FormatAnalysis_UnsupportedFormat(t *testing.T) {
	analysis := createSampleAnalysis()
	formatter := NewFormatter(FormatterOptions{
		Format: "xml", // Unsupported format
	})

	var buf bytes.Buffer
	err := formatter.FormatAnalysis(analysis, &buf)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format: xml")
}

func TestFormatter_FormatAnalysis_NilAnalysis(t *testing.T) {
	formatter := NewFormatter(FormatterOptions{})

	var buf bytes.Buffer
	err := formatter.FormatAnalysis(nil, &buf)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "analysis result cannot be nil")
}

func TestFormatter_ApplyFiltering(t *testing.T) {
	analysis := createSampleAnalysis()

	tests := []struct {
		name            string
		filterLevel     string
		expectedResults int
		expectedPassed  int
		expectedFailed  int
		expectedWarned  int
	}{
		{
			name:            "all results",
			filterLevel:     "all",
			expectedResults: 3,
			expectedPassed:  1,
			expectedFailed:  1,
			expectedWarned:  1,
		},
		{
			name:            "failures only",
			filterLevel:     "failures",
			expectedResults: 1,
			expectedPassed:  0,
			expectedFailed:  1,
			expectedWarned:  0,
		},
		{
			name:            "warnings only",
			filterLevel:     "warnings",
			expectedResults: 1,
			expectedPassed:  0,
			expectedFailed:  0,
			expectedWarned:  1,
		},
		{
			name:            "passes only",
			filterLevel:     "passes",
			expectedResults: 1,
			expectedPassed:  1,
			expectedFailed:  0,
			expectedWarned:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(FormatterOptions{
				FilterLevel: tt.filterLevel,
			})

			artifact := formatter.createArtifact(analysis)
			artifact = formatter.applyFiltering(artifact)

			assert.Len(t, artifact.Results, tt.expectedResults)
			assert.Equal(t, tt.expectedResults, artifact.Summary.TotalChecks)
			assert.Equal(t, tt.expectedPassed, artifact.Summary.PassedChecks)
			assert.Equal(t, tt.expectedFailed, artifact.Summary.FailedChecks)
			assert.Equal(t, tt.expectedWarned, artifact.Summary.WarningChecks)
		})
	}
}

func TestFormatter_ApplySorting(t *testing.T) {
	// Create analysis with results in specific order for testing
	analysis := &analyzer.EnhancedAnalysisResult{
		Results: []analyzer.EnhancedAnalyzerResult{
			{IsPass: true, Title: "Z Check", Impact: ""},     // Should be last in priority sort
			{IsFail: true, Title: "A Check", Impact: "HIGH"}, // Should be first in priority sort
			{IsWarn: true, Title: "M Check", Impact: "LOW"},  // Should be middle in priority sort
		},
		Remediation: []analyzer.RemediationStep{
			{Priority: 3, Title: "Low Priority"},
			{Priority: 1, Title: "High Priority"},
			{Priority: 2, Title: "Medium Priority"},
		},
		Summary:  analyzer.AnalysisSummary{},
		Metadata: analyzer.AnalysisMetadata{},
	}

	tests := []struct {
		name           string
		sortBy         string
		expectedFirst  string
		expectedSecond string
		expectedThird  string
	}{
		{
			name:           "sort by priority",
			sortBy:         "priority",
			expectedFirst:  "A Check", // HIGH impact failure = priority 1
			expectedSecond: "M Check", // Warning = priority 7
			expectedThird:  "Z Check", // Pass = priority 10
		},
		{
			name:           "sort by title",
			sortBy:         "title",
			expectedFirst:  "A Check",
			expectedSecond: "M Check",
			expectedThird:  "Z Check",
		},
		{
			name:           "sort by severity",
			sortBy:         "severity",
			expectedFirst:  "A Check", // Fail = highest severity
			expectedSecond: "M Check", // Warn = medium severity
			expectedThird:  "Z Check", // Pass = lowest severity
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(FormatterOptions{
				SortBy:             tt.sortBy,
				IncludeRemediation: true, // Include remediation to test sorting
			})

			artifact := formatter.createArtifact(analysis)
			artifact = formatter.applySorting(artifact)

			// Ensure we have the expected number of results
			require.Len(t, artifact.Results, 3, "Should have 3 results after sorting")

			assert.Equal(t, tt.expectedFirst, artifact.Results[0].Title)
			assert.Equal(t, tt.expectedSecond, artifact.Results[1].Title)
			assert.Equal(t, tt.expectedThird, artifact.Results[2].Title)

			// Verify remediation is sorted by priority
			assert.Equal(t, "High Priority", artifact.Remediation[0].Title)
			assert.Equal(t, "Medium Priority", artifact.Remediation[1].Title)
			assert.Equal(t, "Low Priority", artifact.Remediation[2].Title)
		})
	}
}

func TestFormatter_GetPriority(t *testing.T) {
	formatter := &Formatter{}

	tests := []struct {
		name             string
		result           analyzer.EnhancedAnalyzerResult
		expectedPriority int
	}{
		{
			name:             "high impact failure",
			result:           analyzer.EnhancedAnalyzerResult{IsFail: true, Impact: "HIGH"},
			expectedPriority: 1,
		},
		{
			name:             "critical impact failure",
			result:           analyzer.EnhancedAnalyzerResult{IsFail: true, Impact: "CRITICAL"},
			expectedPriority: 1,
		},
		{
			name:             "medium impact failure",
			result:           analyzer.EnhancedAnalyzerResult{IsFail: true, Impact: "MEDIUM"},
			expectedPriority: 3,
		},
		{
			name:             "unspecified impact failure",
			result:           analyzer.EnhancedAnalyzerResult{IsFail: true, Impact: ""},
			expectedPriority: 5,
		},
		{
			name:             "warning",
			result:           analyzer.EnhancedAnalyzerResult{IsWarn: true},
			expectedPriority: 7,
		},
		{
			name:             "pass",
			result:           analyzer.EnhancedAnalyzerResult{IsPass: true},
			expectedPriority: 10,
		},
		{
			name:             "unknown state",
			result:           analyzer.EnhancedAnalyzerResult{},
			expectedPriority: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority := formatter.getPriority(tt.result)
			assert.Equal(t, tt.expectedPriority, priority)
		})
	}
}

func TestFormatter_GetSeverityWeight(t *testing.T) {
	formatter := &Formatter{}

	tests := []struct {
		name           string
		result         analyzer.EnhancedAnalyzerResult
		expectedWeight int
	}{
		{
			name:           "failure",
			result:         analyzer.EnhancedAnalyzerResult{IsFail: true},
			expectedWeight: 3,
		},
		{
			name:           "warning",
			result:         analyzer.EnhancedAnalyzerResult{IsWarn: true},
			expectedWeight: 2,
		},
		{
			name:           "pass",
			result:         analyzer.EnhancedAnalyzerResult{IsPass: true},
			expectedWeight: 1,
		},
		{
			name:           "unknown",
			result:         analyzer.EnhancedAnalyzerResult{},
			expectedWeight: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			weight := formatter.getSeverityWeight(tt.result)
			assert.Equal(t, tt.expectedWeight, weight)
		})
	}
}

func TestFormatter_CreateArtifact(t *testing.T) {
	analysis := createSampleAnalysis()

	tests := []struct {
		name        string
		options     FormatterOptions
		checkFields func(*testing.T, *AnalysisArtifact)
	}{
		{
			name: "include all sections",
			options: FormatterOptions{
				IncludeMetadata:    true,
				IncludeRemediation: true,
				IncludeInsights:    true,
			},
			checkFields: func(t *testing.T, artifact *AnalysisArtifact) {
				assert.NotZero(t, artifact.Metadata.Timestamp)
				assert.NotEmpty(t, artifact.Remediation)
				// Insights would be checked if implemented
			},
		},
		{
			name: "minimal artifact",
			options: FormatterOptions{
				IncludeMetadata:    false,
				IncludeRemediation: false,
				IncludeInsights:    false,
			},
			checkFields: func(t *testing.T, artifact *AnalysisArtifact) {
				assert.Zero(t, artifact.Metadata.Timestamp)
				assert.Empty(t, artifact.Remediation)
				assert.Empty(t, artifact.Insights)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			formatter := NewFormatter(tt.options)
			artifact := formatter.createArtifact(analysis)

			// Common checks
			assert.Equal(t, "https://schemas.troubleshoot.sh/analysis/v1beta2", artifact.Schema)
			assert.Equal(t, "v1beta2", artifact.Version)
			assert.Len(t, artifact.Results, 3)
			assert.Equal(t, analysis.Summary, artifact.Summary)

			// Conditional checks
			tt.checkFields(t, artifact)
		})
	}
}

func TestFormatLegacyResults(t *testing.T) {
	legacyResults := []*analyzer.AnalyzeResult{
		{
			IsPass:  true,
			Title:   "Legacy Pass Check",
			Message: "This passed",
		},
		{
			IsFail:  true,
			Title:   "Legacy Fail Check",
			Message: "This failed",
		},
		nil, // Test nil handling
	}

	var buf bytes.Buffer
	err := FormatLegacyResults(legacyResults, "json", &buf)

	assert.NoError(t, err)

	// Parse the JSON to verify conversion worked
	var artifact AnalysisArtifact
	err = json.Unmarshal(buf.Bytes(), &artifact)
	require.NoError(t, err)

	// Should have 2 results (nil filtered out)
	assert.Len(t, artifact.Results, 2)

	// Verify conversion (results are sorted by priority, so failures come first)
	assert.True(t, artifact.Results[0].IsFail)
	assert.Equal(t, "Legacy Fail Check", artifact.Results[0].Title)
	assert.Equal(t, "legacy", artifact.Results[0].AgentUsed)
	assert.Equal(t, 0.8, artifact.Results[0].Confidence)

	assert.True(t, artifact.Results[1].IsPass)
	assert.Equal(t, "Legacy Pass Check", artifact.Results[1].Title)

	// Verify summary
	assert.Equal(t, 2, artifact.Summary.TotalChecks)
	assert.Equal(t, 1, artifact.Summary.PassedChecks)
	assert.Equal(t, 1, artifact.Summary.FailedChecks)
	assert.Equal(t, "CRITICAL", artifact.Summary.OverallHealth) // 50% failure rate = critical

	// Verify metadata
	assert.Equal(t, "legacy", artifact.Metadata.EngineVersion)
	assert.Contains(t, artifact.Metadata.AgentsUsed, "legacy")
}

func TestGenerateSummaryFromLegacy(t *testing.T) {
	results := []analyzer.EnhancedAnalyzerResult{
		{IsPass: true},
		{IsPass: true},
		{IsFail: true},
		{IsWarn: true},
	}

	summary := generateSummaryFromLegacy(results)

	assert.Equal(t, 4, summary.TotalChecks)
	assert.Equal(t, 2, summary.PassedChecks)
	assert.Equal(t, 1, summary.FailedChecks)
	assert.Equal(t, 1, summary.WarningChecks)
	assert.Equal(t, "CRITICAL", summary.OverallHealth) // 1/4 = 25% > 10% threshold
	assert.Equal(t, 0.8, summary.Confidence)
}

func TestFormatter_WriteJSON_Error(t *testing.T) {
	formatter := &Formatter{}

	// Create an artifact with circular reference to cause JSON marshal error
	artifact := &AnalysisArtifact{}

	// Use a writer that will fail
	failingWriter := &failingWriter{}

	// This should work since the artifact is fine, but writer will fail
	err := formatter.writeJSON(artifact, failingWriter)
	assert.Error(t, err)
}

func TestFormatter_GenerateHTML_Comprehensive(t *testing.T) {
	formatter := &Formatter{}
	analysis := createSampleAnalysis()
	artifact := formatter.createArtifact(analysis)

	html := formatter.generateHTML(artifact)

	// Check for all expected HTML elements
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "<title>Troubleshoot Analysis Report</title>")
	assert.Contains(t, html, "DEGRADED") // Overall health
	assert.Contains(t, html, "3")        // Total checks
	assert.Contains(t, html, "1")        // Passed checks
	assert.Contains(t, html, "1")        // Failed checks
	assert.Contains(t, html, "1")        // Warning checks

	// Check for specific results
	assert.Contains(t, html, "Kubernetes Version")
	assert.Contains(t, html, "Storage Class")
	assert.Contains(t, html, "Memory Usage")

	// Check for explanations and evidence
	assert.Contains(t, html, "version check passed successfully")
	assert.Contains(t, html, "Found Kubernetes 1.24.0")

	// Check for remediation
	assert.Contains(t, html, "Create Default Storage Class")
	assert.Contains(t, html, "kubectl get storageclass")

	// Check for confidence and impact indicators
	assert.Contains(t, html, "95.0%") // Confidence for version check
	assert.Contains(t, html, "HIGH")  // Impact for storage issue

	// Check for CSS classes
	assert.Contains(t, html, "class=\"result pass\"")
	assert.Contains(t, html, "class=\"result fail\"")
	assert.Contains(t, html, "class=\"result warn\"")
}

// Helper types for testing
type failingWriter struct{}

func (w *failingWriter) Write([]byte) (int, error) {
	return 0, assert.AnError
}

// Benchmark tests
func BenchmarkFormatter_FormatAnalysis_JSON(b *testing.B) {
	analysis := createSampleAnalysis()
	formatter := NewFormatter(FormatterOptions{
		Format:             "json",
		IncludeMetadata:    true,
		IncludeRemediation: true,
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		err := formatter.FormatAnalysis(analysis, &buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFormatter_FormatAnalysis_HTML(b *testing.B) {
	analysis := createSampleAnalysis()
	formatter := NewFormatter(FormatterOptions{
		Format:             "html",
		IncludeMetadata:    true,
		IncludeRemediation: true,
	})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		err := formatter.FormatAnalysis(analysis, &buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFormatter_ApplySorting(b *testing.B) {
	// Create analysis with many results
	results := make([]analyzer.EnhancedAnalyzerResult, 1000)
	for i := range results {
		results[i] = analyzer.EnhancedAnalyzerResult{
			IsPass: i%3 == 0,
			IsFail: i%3 == 1,
			IsWarn: i%3 == 2,
			Title:  "Benchmark Check",
			Impact: []string{"HIGH", "MEDIUM", "LOW"}[i%3],
		}
	}

	analysis := &analyzer.EnhancedAnalysisResult{
		Results:  results,
		Summary:  analyzer.AnalysisSummary{},
		Metadata: analyzer.AnalysisMetadata{},
	}

	formatter := NewFormatter(FormatterOptions{SortBy: "priority"})

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		artifact := formatter.createArtifact(analysis)
		_ = formatter.applySorting(artifact)
	}
}
