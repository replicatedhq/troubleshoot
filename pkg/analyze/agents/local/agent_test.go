package local

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLocalAgent(t *testing.T) {
	agent := NewLocalAgent()

	assert.NotNil(t, agent)
	assert.Equal(t, "local", agent.Name())
	assert.Equal(t, "1.0.0", agent.Version())
	assert.Contains(t, agent.Capabilities(), "offline-analysis")
	assert.Contains(t, agent.Capabilities(), "built-in-analyzers")
}

func TestLocalAgent_HealthCheck(t *testing.T) {
	agent := NewLocalAgent()
	ctx := context.Background()

	err := agent.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestLocalAgent_HealthCheck_WithContext(t *testing.T) {
	agent := NewLocalAgent()

	// Test with cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Should still pass since health check doesn't use context extensively
	err := agent.HealthCheck(ctx)
	assert.NoError(t, err)
}

func TestLocalAgent_GetDefaultAnalyzers(t *testing.T) {
	agent := NewLocalAgent()

	analyzers, hostAnalyzers, err := agent.getDefaultAnalyzers()

	assert.NoError(t, err)
	assert.NotEmpty(t, analyzers)
	assert.NotEmpty(t, hostAnalyzers)

	// Check that we have essential analyzers
	hasClusterVersion := false
	hasNodeResources := false
	hasStorageClass := false

	for _, analyzer := range analyzers {
		if analyzer.ClusterVersion != nil {
			hasClusterVersion = true
		}
		if analyzer.NodeResources != nil {
			hasNodeResources = true
		}
		if analyzer.StorageClass != nil {
			hasStorageClass = true
		}
	}

	assert.True(t, hasClusterVersion, "Should have cluster version analyzer")
	assert.True(t, hasNodeResources, "Should have node resources analyzer")
	assert.True(t, hasStorageClass, "Should have storage class analyzer")

	// Check host analyzers
	hasCPU := false
	hasMemory := false

	for _, hostAnalyzer := range hostAnalyzers {
		if hostAnalyzer.CPU != nil {
			hasCPU = true
		}
		if hostAnalyzer.Memory != nil {
			hasMemory = true
		}
	}

	assert.True(t, hasCPU, "Should have CPU analyzer")
	assert.True(t, hasMemory, "Should have memory analyzer")
}

func TestLocalAgent_EnhanceResults(t *testing.T) {
	agent := NewLocalAgent()

	// Create basic analyzer results
	basicResults := []*analyzer.AnalyzeResult{
		{
			IsPass:  true,
			Title:   "Kubernetes Version",
			Message: "Kubernetes 1.24.0 is supported",
		},
		{
			IsFail:  true,
			Title:   "Storage Class",
			Message: "No default storage class found",
		},
		{
			IsWarn:  true,
			Title:   "Memory Usage",
			Message: "Memory usage is above 80%",
		},
		nil, // Test nil handling
	}

	enhanced := agent.enhanceResults(basicResults)

	// Should have 3 results (nil filtered out)
	assert.Len(t, enhanced, 3)

	// Check first result (pass)
	result1 := enhanced[0]
	assert.True(t, result1.IsPass)
	assert.Equal(t, "Kubernetes Version", result1.Title)
	assert.Equal(t, "local", result1.AgentUsed)
	assert.Greater(t, result1.Confidence, 0.0)
	assert.NotEmpty(t, result1.Explanation)
	assert.Empty(t, result1.Impact)    // Pass results don't have impact
	assert.Nil(t, result1.Remediation) // Pass results don't need remediation

	// Check second result (fail)
	result2 := enhanced[1]
	assert.True(t, result2.IsFail)
	assert.Equal(t, "Storage Class", result2.Title)
	assert.NotEmpty(t, result2.Impact)
	assert.NotNil(t, result2.Remediation)
	assert.NotEmpty(t, result2.Explanation)
	assert.NotEmpty(t, result2.Evidence)

	// Check third result (warn)
	result3 := enhanced[2]
	assert.True(t, result3.IsWarn)
	assert.Equal(t, "LOW", result3.Impact)
	assert.NotEmpty(t, result3.Explanation)
}

func TestLocalAgent_CalculateConfidence(t *testing.T) {
	agent := NewLocalAgent()

	tests := []struct {
		name               string
		result             *analyzer.AnalyzeResult
		expectedConfidence float64
		tolerance          float64
	}{
		{
			name: "version check - high confidence",
			result: &analyzer.AnalyzeResult{
				Title:   "Kubernetes Version",
				Message: "Version 1.24.0",
			},
			expectedConfidence: 0.95,
			tolerance:          0.01,
		},
		{
			name: "uncertain check - low confidence",
			result: &analyzer.AnalyzeResult{
				Title:   "Unknown Status",
				Message: "Unable to determine the status",
			},
			expectedConfidence: 0.3,
			tolerance:          0.01,
		},
		{
			name: "regular check - base confidence",
			result: &analyzer.AnalyzeResult{
				Title:   "Storage Class",
				Message: "Default storage class exists",
			},
			expectedConfidence: 0.8,
			tolerance:          0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			confidence := agent.calculateConfidence(tt.result)
			assert.InDelta(t, tt.expectedConfidence, confidence, tt.tolerance)
		})
	}
}

func TestLocalAgent_DetermineImpact(t *testing.T) {
	agent := NewLocalAgent()

	tests := []struct {
		name           string
		result         *analyzer.AnalyzeResult
		expectedImpact string
	}{
		{
			name: "failed version check - high impact",
			result: &analyzer.AnalyzeResult{
				IsFail: true,
				Title:  "Kubernetes Version",
			},
			expectedImpact: "HIGH",
		},
		{
			name: "failed node check - high impact",
			result: &analyzer.AnalyzeResult{
				IsFail: true,
				Title:  "Node Resources",
			},
			expectedImpact: "HIGH",
		},
		{
			name: "failed storage check - high impact",
			result: &analyzer.AnalyzeResult{
				IsFail: true,
				Title:  "Storage Class",
			},
			expectedImpact: "HIGH",
		},
		{
			name: "failed other check - medium impact",
			result: &analyzer.AnalyzeResult{
				IsFail: true,
				Title:  "Custom Check",
			},
			expectedImpact: "MEDIUM",
		},
		{
			name: "warning check - low impact",
			result: &analyzer.AnalyzeResult{
				IsWarn: true,
				Title:  "Memory Usage",
			},
			expectedImpact: "LOW",
		},
		{
			name: "passing check - no impact",
			result: &analyzer.AnalyzeResult{
				IsPass: true,
				Title:  "All Good",
			},
			expectedImpact: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			impact := agent.determineImpact(tt.result)
			assert.Equal(t, tt.expectedImpact, impact)
		})
	}
}

func TestLocalAgent_GenerateExplanation(t *testing.T) {
	agent := NewLocalAgent()

	tests := []struct {
		name          string
		result        *analyzer.AnalyzeResult
		shouldContain []string
	}{
		{
			name: "passing check",
			result: &analyzer.AnalyzeResult{
				IsPass:  true,
				Title:   "Storage Check",
				Message: "Storage is available",
			},
			shouldContain: []string{"storage check", "passed successfully", "Storage is available"},
		},
		{
			name: "failing check",
			result: &analyzer.AnalyzeResult{
				IsFail:  true,
				Title:   "Memory Check",
				Message: "Insufficient memory",
			},
			shouldContain: []string{"memory check", "failed", "Insufficient memory"},
		},
		{
			name: "warning check",
			result: &analyzer.AnalyzeResult{
				IsWarn:  true,
				Title:   "CPU Usage",
				Message: "High CPU usage detected",
			},
			shouldContain: []string{"cpu usage", "warning", "High CPU usage detected", "performance"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			explanation := agent.generateExplanation(tt.result)

			assert.NotEmpty(t, explanation)
			for _, expected := range tt.shouldContain {
				assert.Contains(t, strings.ToLower(explanation), strings.ToLower(expected))
			}
		})
	}
}

func TestLocalAgent_InferProblem(t *testing.T) {
	agent := NewLocalAgent()

	tests := []struct {
		name            string
		result          *analyzer.AnalyzeResult
		expectedProblem string
	}{
		{
			name: "version issue",
			result: &analyzer.AnalyzeResult{
				Title: "Kubernetes Version",
			},
			expectedProblem: "version does not meet requirements",
		},
		{
			name: "node issue",
			result: &analyzer.AnalyzeResult{
				Title: "Node Resources",
			},
			expectedProblem: "insufficient cluster resources",
		},
		{
			name: "storage issue",
			result: &analyzer.AnalyzeResult{
				Title: "Storage Class",
			},
			expectedProblem: "storage configuration is inadequate",
		},
		{
			name: "memory issue",
			result: &analyzer.AnalyzeResult{
				Title:   "Resource Check",
				Message: "Memory usage too high",
			},
			expectedProblem: "memory-related issues",
		},
		{
			name: "generic issue",
			result: &analyzer.AnalyzeResult{
				Title: "Unknown Check",
			},
			expectedProblem: "system requirement is not met",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problem := agent.inferProblem(tt.result)
			assert.Contains(t, problem, tt.expectedProblem)
		})
	}
}

func TestLocalAgent_GatherEvidence(t *testing.T) {
	agent := NewLocalAgent()

	targetResult := &analyzer.AnalyzeResult{
		Title:   "Storage Class",
		Message: "No default storage class found",
	}

	allResults := []*analyzer.AnalyzeResult{
		targetResult,
		{
			Title:   "Storage Volume",
			Message: "Cannot create persistent volume",
			IsFail:  true,
		},
		{
			Title:   "Memory Usage",
			Message: "Memory is fine",
			IsPass:  true,
		},
		nil, // Test nil handling
	}

	evidence := agent.gatherEvidence(targetResult, allResults)

	assert.NotEmpty(t, evidence)
	assert.Contains(t, evidence[0], "Check result: No default storage class found")

	// Should include related failures but not passes or nils
	foundRelated := false
	for _, ev := range evidence {
		if strings.Contains(ev, "Storage Volume") {
			foundRelated = true
			break
		}
	}
	assert.True(t, foundRelated, "Should find related storage issue")
}

func TestLocalAgent_AreResultsRelated(t *testing.T) {
	agent := NewLocalAgent()

	result1 := &analyzer.AnalyzeResult{Title: "Storage Class Check"}
	result2 := &analyzer.AnalyzeResult{Title: "Storage Volume Status"}
	result3 := &analyzer.AnalyzeResult{Title: "Memory Usage"}

	// Should be related (both contain "Storage")
	assert.True(t, agent.areResultsRelated(result1, result2))

	// Should not be related
	assert.False(t, agent.areResultsRelated(result1, result3))

	// Should not be related to itself (different check in implementation)
	assert.False(t, agent.areResultsRelated(result1, result1))
}

func TestLocalAgent_GenerateRemediation(t *testing.T) {
	agent := NewLocalAgent()

	tests := []struct {
		name           string
		result         *analyzer.AnalyzeResult
		expectedChecks []string // Things that should be in the remediation
	}{
		{
			name: "version issue",
			result: &analyzer.AnalyzeResult{
				Title:  "Kubernetes Version",
				IsFail: true,
			},
			expectedChecks: []string{"kubectl version", "upgrade"},
		},
		{
			name: "storage issue",
			result: &analyzer.AnalyzeResult{
				Title:  "Storage Class",
				IsFail: true,
			},
			expectedChecks: []string{"kubectl get storageclass", "storage class"},
		},
		{
			name: "node issue",
			result: &analyzer.AnalyzeResult{
				Title:  "Node Resources",
				IsFail: true,
			},
			expectedChecks: []string{"kubectl get nodes", "kubectl describe nodes"},
		},
		{
			name: "generic issue",
			result: &analyzer.AnalyzeResult{
				Title:  "Custom Check",
				IsFail: true,
			},
			expectedChecks: []string{"configuration", "logs"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			remediation := agent.generateRemediation(tt.result)

			assert.NotNil(t, remediation)
			assert.NotEmpty(t, remediation.ID)
			assert.NotEmpty(t, remediation.Title)
			assert.NotEmpty(t, remediation.Description)
			assert.Equal(t, "immediate", remediation.Category)
			assert.Greater(t, remediation.Priority, 0)
			assert.NotNil(t, remediation.Validation)

			// Check that expected elements are present
			allText := strings.ToLower(remediation.Description)
			for _, cmd := range remediation.Commands {
				allText += " " + strings.ToLower(cmd)
			}
			for _, manual := range remediation.Manual {
				allText += " " + strings.ToLower(manual)
			}

			for _, expected := range tt.expectedChecks {
				assert.Contains(t, allText, strings.ToLower(expected),
					"Remediation should contain guidance about %s", expected)
			}
		})
	}
}

func TestLocalAgent_GetPriority(t *testing.T) {
	agent := NewLocalAgent()

	tests := []struct {
		name             string
		result           *analyzer.AnalyzeResult
		expectedPriority int
	}{
		{
			name:             "version failure - highest priority",
			result:           &analyzer.AnalyzeResult{IsFail: true, Title: "Kubernetes Version"},
			expectedPriority: 1,
		},
		{
			name:             "node failure - highest priority",
			result:           &analyzer.AnalyzeResult{IsFail: true, Title: "Node Resources"},
			expectedPriority: 1,
		},
		{
			name:             "storage failure - high priority",
			result:           &analyzer.AnalyzeResult{IsFail: true, Title: "Storage Class"},
			expectedPriority: 2,
		},
		{
			name:             "other failure - medium priority",
			result:           &analyzer.AnalyzeResult{IsFail: true, Title: "Custom Check"},
			expectedPriority: 5,
		},
		{
			name:             "warning - low priority",
			result:           &analyzer.AnalyzeResult{IsWarn: true, Title: "Memory Usage"},
			expectedPriority: 7,
		},
		{
			name:             "pass - no priority needed",
			result:           &analyzer.AnalyzeResult{IsPass: true, Title: "All Good"},
			expectedPriority: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			priority := agent.getPriority(tt.result)
			assert.Equal(t, tt.expectedPriority, priority)
		})
	}
}

func TestLocalAgent_FindRelatedIssues(t *testing.T) {
	agent := NewLocalAgent()

	targetResult := &analyzer.AnalyzeResult{
		Title: "Storage Class Check",
	}

	allResults := []*analyzer.AnalyzeResult{
		{Title: "Memory Check"},   // index 0 - not related
		targetResult,              // index 1 - self
		{Title: "Storage Volume"}, // index 2 - related
		{Title: "CPU Usage"},      // index 3 - not related
		{Title: "Storage Policy"}, // index 4 - related
	}

	related := agent.findRelatedIssues(targetResult, allResults, 1)

	// Should find related issues at indices 2 and 4
	assert.Contains(t, related, "2")
	assert.Contains(t, related, "4")
	assert.NotContains(t, related, "0")
	assert.NotContains(t, related, "1") // Should not include self
	assert.NotContains(t, related, "3")
}

func TestLocalAgent_GenerateInsights(t *testing.T) {
	agent := NewLocalAgent()

	// Create results with multiple resource failures
	results := []analyzer.EnhancedAnalyzerResult{
		{IsPass: true, Title: "Network Check"},
		{IsFail: true, Title: "Node Resources"},
		{IsFail: true, Title: "Memory Usage"},
		{IsFail: true, Title: "CPU Allocation"},
		{IsWarn: true, Title: "Storage Warning"},
	}

	insights := agent.generateInsights(results)

	assert.NotEmpty(t, insights)

	// Should detect resource correlation
	found := false
	for _, insight := range insights {
		if strings.Contains(insight.Title, "Resource") &&
			strings.Contains(insight.Title, "Issues") &&
			insight.Type == "correlation" {
			found = true
			assert.Greater(t, insight.Confidence, 0.0)
			assert.Equal(t, "HIGH", insight.Impact)
			assert.NotEmpty(t, insight.Evidence)
			break
		}
	}
	assert.True(t, found, "Should generate resource correlation insight")
}

func TestLocalAgent_FindCorrelations(t *testing.T) {
	agent := NewLocalAgent()

	tests := []struct {
		name              string
		results           []analyzer.EnhancedAnalyzerResult
		expectCorrelation bool
	}{
		{
			name: "multiple resource failures - should correlate",
			results: []analyzer.EnhancedAnalyzerResult{
				{IsFail: true, Title: "Node Resources"},
				{IsFail: true, Title: "Memory Usage"},
				{IsPass: true, Title: "Network"},
			},
			expectCorrelation: true,
		},
		{
			name: "single resource failure - should not correlate",
			results: []analyzer.EnhancedAnalyzerResult{
				{IsFail: true, Title: "Node Resources"},
				{IsPass: true, Title: "Network"},
			},
			expectCorrelation: false,
		},
		{
			name: "no resource failures - should not correlate",
			results: []analyzer.EnhancedAnalyzerResult{
				{IsPass: true, Title: "Node Resources"},
				{IsPass: true, Title: "Memory Usage"},
			},
			expectCorrelation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insights := agent.findCorrelations(tt.results)

			if tt.expectCorrelation {
				assert.NotEmpty(t, insights)
				assert.Contains(t, insights[0].Title, "Resource")
				assert.Equal(t, "correlation", insights[0].Type)
			} else {
				assert.Empty(t, insights)
			}
		})
	}
}

func TestLocalAgent_GenerateRecommendations(t *testing.T) {
	agent := NewLocalAgent()

	tests := []struct {
		name                 string
		results              []analyzer.EnhancedAnalyzerResult
		expectRecommendation bool
	}{
		{
			name: "many failures - should recommend",
			results: []analyzer.EnhancedAnalyzerResult{
				{IsFail: true, Title: "Check 1"},
				{IsFail: true, Title: "Check 2"},
				{IsFail: true, Title: "Check 3"},
			},
			expectRecommendation: true,
		},
		{
			name: "many warnings - should recommend",
			results: []analyzer.EnhancedAnalyzerResult{
				{IsWarn: true, Title: "Check 1"},
				{IsWarn: true, Title: "Check 2"},
				{IsWarn: true, Title: "Check 3"},
				{IsWarn: true, Title: "Check 4"},
			},
			expectRecommendation: true,
		},
		{
			name: "mostly healthy - should not recommend",
			results: []analyzer.EnhancedAnalyzerResult{
				{IsPass: true, Title: "Check 1"},
				{IsPass: true, Title: "Check 2"},
				{IsWarn: true, Title: "Check 3"},
			},
			expectRecommendation: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insights := agent.generateRecommendations(tt.results)

			if tt.expectRecommendation {
				assert.NotEmpty(t, insights)
				assert.Contains(t, insights[0].Title, "Health")
				assert.Equal(t, "recommendation", insights[0].Type)
			} else {
				assert.Empty(t, insights)
			}
		})
	}
}

func TestLocalAgent_Analyze_Integration(t *testing.T) {
	agent := NewLocalAgent()

	// Create a temporary bundle directory with required structure
	tmpDir := t.TempDir()
	bundlePath := filepath.Join(tmpDir, "test-bundle")
	err := os.MkdirAll(bundlePath, 0755)
	require.NoError(t, err)

	// Create version.json file (required by FindBundleRootDir)
	versionFile := filepath.Join(bundlePath, "version.json")
	err = os.WriteFile(versionFile, []byte(`{"version": "test"}`), 0644)
	require.NoError(t, err)

	// Create bundle structure
	bundle := &analyzer.SupportBundle{
		Path:    bundlePath,
		RootDir: bundlePath,
		GetFile: func(filename string) ([]byte, error) {
			return []byte("test data"), nil
		},
		FindFiles: func(path string, filenames []string) (map[string][]byte, error) {
			return map[string][]byte{"test.txt": []byte("test")}, nil
		},
	}

	ctx := context.Background()
	specs := []analyzer.AnalyzerSpec{} // Empty for this test

	result, err := agent.Analyze(ctx, bundle, specs)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "local", result.AgentName)
	assert.Greater(t, result.ProcessingTime, time.Duration(0))
	assert.NotNil(t, result.Metadata)
	assert.Equal(t, true, result.Metadata["enhancementsApplied"])

	// Should have some default analyzer results
	assert.NotEmpty(t, result.Results)

	// All results should have the agent name set
	for _, res := range result.Results {
		assert.Equal(t, "local", res.AgentUsed)
		assert.Greater(t, res.Confidence, 0.0)
	}
}

func TestLocalAgent_Analyze_WithNilBundle(t *testing.T) {
	agent := NewLocalAgent()

	ctx := context.Background()
	specs := []analyzer.AnalyzerSpec{}

	result, err := agent.Analyze(ctx, nil, specs)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "bundle cannot be nil")
}

// Benchmark tests
func BenchmarkLocalAgent_EnhanceResults(b *testing.B) {
	agent := NewLocalAgent()

	// Create many basic results
	basicResults := make([]*analyzer.AnalyzeResult, 100)
	for i := range basicResults {
		basicResults[i] = &analyzer.AnalyzeResult{
			IsPass:  i%3 == 0,
			IsFail:  i%3 == 1,
			IsWarn:  i%3 == 2,
			Title:   "Benchmark Check",
			Message: "Benchmark message",
		}
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = agent.enhanceResults(basicResults)
	}
}

func BenchmarkLocalAgent_GenerateRemediation(b *testing.B) {
	agent := NewLocalAgent()

	result := &analyzer.AnalyzeResult{
		IsFail:  true,
		Title:   "Storage Class",
		Message: "No default storage class found",
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = agent.generateRemediation(result)
	}
}
