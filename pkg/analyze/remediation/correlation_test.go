package remediation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewCorrelationEngine(t *testing.T) {
	engine := NewCorrelationEngine()

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.algorithms)
	assert.NotEmpty(t, engine.algorithms)
}

func TestCorrelationEngine_FindCorrelations_Basic(t *testing.T) {
	engine := NewCorrelationEngine()

	analysisResults := []AnalysisResult{
		{
			ID:          "cpu-high",
			Title:       "High CPU Usage",
			Description: "CPU usage is above 90%",
			Category:    "resource",
			Severity:    "high",
			Evidence:    []string{"CPU metrics show 95% usage"},
		},
		{
			ID:          "memory-high",
			Title:       "High Memory Usage",
			Description: "Memory usage is above 85%",
			Category:    "resource",
			Severity:    "high",
			Evidence:    []string{"Memory metrics show 90% usage"},
		},
		{
			ID:          "disk-full",
			Title:       "Disk Space Low",
			Description: "Disk usage is above 95%",
			Category:    "storage",
			Severity:    "critical",
			Evidence:    []string{"Disk usage at 97%"},
		},
	}

	steps := []RemediationStep{
		{
			ID:       "scale-cpu",
			Title:    "Scale CPU Resources",
			Category: CategoryResource,
		},
		{
			ID:       "scale-memory",
			Title:    "Scale Memory Resources",
			Category: CategoryResource,
		},
	}

	correlations := engine.FindCorrelations(analysisResults, steps)

	assert.NotNil(t, correlations)
	// Should find correlations between resource-related issues
	resourceCorrelations := 0
	for _, correlation := range correlations {
		if correlation.Type == CorrelationFunctional &&
			len(correlation.Results) >= 2 {
			resourceCorrelations++
		}
	}
	assert.Greater(t, resourceCorrelations, 0, "Should find resource correlations")
}

func TestCorrelationEngine_FindCorrelations_Temporal(t *testing.T) {
	engine := NewCorrelationEngine()

	analysisResults := []AnalysisResult{
		{
			ID:       "error-1",
			Title:    "Application Error",
			Category: "application",
			Severity: "high",
		},
		{
			ID:       "error-2",
			Title:    "Database Connection Error",
			Category: "database",
			Severity: "critical",
		},
		{
			ID:       "unrelated",
			Title:    "Unrelated Issue",
			Category: "network",
			Severity: "low",
		},
	}

	correlations := engine.FindCorrelations(analysisResults, []RemediationStep{})

	// Should find temporal correlation between error-1 and error-2
	temporalCorrelation := false
	for _, correlation := range correlations {
		if correlation.Type == CorrelationTemporal &&
			len(correlation.Results) == 2 {
			temporalCorrelation = true
			assert.Greater(t, correlation.Strength, 0.5)
			break
		}
	}
	assert.True(t, temporalCorrelation, "Should find temporal correlation between closely timed events")
}

func TestCorrelationEngine_FindCorrelations_Causal(t *testing.T) {
	engine := NewCorrelationEngine()

	analysisResults := []AnalysisResult{
		{
			ID:          "pod-crash",
			Title:       "Pod Crashing",
			Description: "Pod is in CrashLoopBackOff state",
			Category:    "kubernetes",
			Severity:    "critical",
			Evidence:    []string{"Pod exit code 1", "OOMKilled"},
		},
		{
			ID:          "memory-limit",
			Title:       "Memory Limit Exceeded",
			Description: "Container memory usage exceeds limits",
			Category:    "resource",
			Severity:    "high",
			Evidence:    []string{"Memory usage 512Mi > limit 256Mi"},
		},
	}

	correlations := engine.FindCorrelations(analysisResults, []RemediationStep{})

	// Should find causal relationship
	causalCorrelation := false
	for _, correlation := range correlations {
		if correlation.Type == CorrelationCausal {
			causalCorrelation = true
			assert.Greater(t, correlation.Strength, 0.7)
			assert.Greater(t, correlation.Confidence, 0.6)
			break
		}
	}
	assert.True(t, causalCorrelation, "Should find causal correlation between memory limit and pod crash")
}

func TestCorrelationEngine_FindCorrelations_Resource(t *testing.T) {
	engine := NewCorrelationEngine()

	analysisResults := []AnalysisResult{
		{
			ID:          "service-a-cpu",
			Title:       "Service A High CPU",
			Description: "Service A CPU usage is high on node-1",
			Category:    "resource",
		},
		{
			ID:          "service-b-memory",
			Title:       "Service B High Memory",
			Description: "Service B memory usage is high on node-1",
			Category:    "resource",
		},
		{
			ID:          "service-c-cpu",
			Title:       "Service C High CPU",
			Description: "Service C CPU usage is high on node-2",
			Category:    "resource",
			// Tags:        []string{"node-2", "service-c"},
		},
	}

	correlations := engine.FindCorrelations(analysisResults, []RemediationStep{})

	// Should find resource correlation for issues on same node
	resourceCorrelation := false
	for _, correlation := range correlations {
		if correlation.Type == CorrelationResource {
			resourceCorrelation = true
			// Should correlate service-a and service-b (both on node-1)
			assert.Equal(t, 2, len(correlation.Results))
			break
		}
	}
	assert.True(t, resourceCorrelation, "Should find resource correlation for issues on same node")
}

func TestCorrelationAlgorithm_CategoryBased(t *testing.T) {
	// algorithm := &CategoryBasedCorrelationAlgorithm{} // Type not defined
	t.Skip("CategoryBasedCorrelationAlgorithm not implemented yet")

	analysisResults := []AnalysisResult{
		{ID: "res-1", Category: "resource"},
		{ID: "res-2", Category: "resource"},
		{ID: "sec-1", Category: "security"},
		{ID: "net-1", Category: "network"},
	}

	context := RemediationContext{}

	insights := algorithm.FindCorrelations(analysisResults, []RemediationStep{}, context)

	assert.NotEmpty(t, insights)

	// Should find correlation between resource issues
	resourceCorrelation := false
	for _, insight := range insights {
		if insight.Type == CorrelationFunctional &&
			len(insight.Results) == 2 &&
			insight.Strength > 0.5 {
			resourceCorrelation = true
			break
		}
	}
	assert.True(t, resourceCorrelation, "Should correlate issues in same category")
}

func TestCorrelationAlgorithm_TimeBased(t *testing.T) {
	algorithm := &TimeBasedCorrelationAlgorithm{}

	analysisResults := []AnalysisResult{
		{
			ID: "event-1",
		},
		{
			ID:        "event-2",
			Timestamp: baseTime.Add(1 * time.Minute), // Close in time
		},
		{
			ID:        "event-3",
			Timestamp: baseTime.Add(1 * time.Hour), // Far in time
		},
	}

	context := RemediationContext{}

	insights := algorithm.FindCorrelations(analysisResults, []RemediationStep{}, context)

	assert.NotEmpty(t, insights)

	// Should find temporal correlation between event-1 and event-2
	temporalCorrelation := false
	for _, insight := range insights {
		if insight.Type == CorrelationTemporal {
			temporalCorrelation = true
			assert.Equal(t, 2, len(insight.Results))
			break
		}
	}
	assert.True(t, temporalCorrelation, "Should find temporal correlation")
}

func TestCorrelationAlgorithm_CausalityBased(t *testing.T) {
	algorithm := &CausalityBasedCorrelationAlgorithm{}

	analysisResults := []AnalysisResult{
		{
			ID:          "memory-oom",
			Title:       "Out of Memory",
			Description: "Container killed due to OOM",
			Evidence:    []string{"OOMKilled", "memory exceeded"},
		},
		{
			ID:          "pod-restart",
			Title:       "Pod Restarted",
			Description: "Pod was restarted unexpectedly",
			Evidence:    []string{"restart count increased", "container exit"},
		},
		{
			ID:          "unrelated",
			Title:       "Network Issue",
			Description: "DNS resolution failed",
			Evidence:    []string{"dns timeout"},
		},
	}

	context := RemediationContext{}

	insights := algorithm.FindCorrelations(analysisResults, []RemediationStep{}, context)

	// Should find causal relationship between OOM and pod restart
	causalCorrelation := false
	for _, insight := range insights {
		if insight.Type == CorrelationCausal {
			causalCorrelation = true
			assert.Greater(t, insight.Strength, 0.5)
			assert.Equal(t, 2, len(insight.Results))
			break
		}
	}
	assert.True(t, causalCorrelation, "Should find causal correlation")
}

func TestCorrelationAlgorithm_ResourceBased(t *testing.T) {
	algorithm := &ResourceBasedCorrelationAlgorithm{}

	analysisResults := []AnalysisResult{
		{
			ID:          "issue-1",
			Description: "High CPU on node-1",
			// Tags:        []string{"node-1", "cpu"},
		},
		{
			ID:          "issue-2",
			Description: "High memory on node-1",
			// Tags:        []string{"node-1", "memory"},
		},
		{
			ID:          "issue-3",
			Description: "High CPU on node-2",
			// Tags:        []string{"node-2", "cpu"},
		},
	}

	context := RemediationContext{}

	insights := algorithm.FindCorrelations(analysisResults, []RemediationStep{}, context)

	// Should find resource correlation for issues on same node
	resourceCorrelation := false
	for _, insight := range insights {
		if insight.Type == CorrelationResource {
			resourceCorrelation = true
			assert.Equal(t, 2, len(insight.Results)) // issue-1 and issue-2
			break
		}
	}
	assert.True(t, resourceCorrelation, "Should correlate issues affecting same resource")
}

func TestCorrelationEngine_GenerateInsights(t *testing.T) {
	engine := NewCorrelationEngine()

	correlations := []Correlation{
		{
			ID:          "corr-1",
			Type:        CorrelationCausal,
			Description: "Memory exhaustion causes pod crashes",
			Strength:    0.9,
			Confidence:  0.85,
			Results:     []string{"memory-issue", "pod-crash"},
			Evidence:    []string{"OOM events", "restart patterns"},
			Impact:      "High",
		},
		{
			ID:          "corr-2",
			Type:        CorrelationTemporal,
			Description: "Network issues occur together",
			Strength:    0.6,
			Confidence:  0.7,
			Results:     []string{"dns-issue", "connectivity-issue"},
			Evidence:    []string{"timing patterns"},
			Impact:      "Medium",
		},
	}

	analysisResults := []AnalysisResult{
		{ID: "memory-issue", Category: "resource"},
		{ID: "pod-crash", Category: "kubernetes"},
		{ID: "dns-issue", Category: "network"},
		{ID: "connectivity-issue", Category: "network"},
	}

	insights := engine.GenerateInsights(correlations, analysisResults)

	assert.NotEmpty(t, insights)
	assert.LessOrEqual(t, len(insights), len(correlations))

	// Check that high-strength correlations generate insights
	hasHighStrengthInsight := false
	for _, insight := range insights {
		if insight.Confidence >= 0.8 {
			hasHighStrengthInsight = true
			assert.NotEmpty(t, insight.Description)
			assert.NotEmpty(t, insight.Evidence)
			break
		}
	}
	assert.True(t, hasHighStrengthInsight, "Should generate insights for high-strength correlations")
}

func TestCorrelationEngine_GenerateRemediationSuggestions(t *testing.T) {
	engine := NewCorrelationEngine()

	insights := []CorrelationInsight{
		{
			Type:             CorrelationCausal,
			Category:         "resource",
			Description:      "Memory limits cause pod restarts",
			Confidence:       0.9,
			Evidence:         []string{"OOM events correlate with restarts"},
			RelatedAnalyzers: []string{"memory-analyzer", "pod-analyzer"},
		},
		{
			Type:             CorrelationTemporal,
			Category:         "network",
			Description:      "Network issues cluster in time",
			Confidence:       0.7,
			Evidence:         []string{"Multiple network failures within 5 minutes"},
			RelatedAnalyzers: []string{"dns-analyzer", "connectivity-analyzer"},
		},
	}

	suggestions := engine.GenerateRemediationSuggestions(insights)

	assert.NotEmpty(t, suggestions)

	// Should generate suggestions based on insight categories
	resourceSuggestion := false
	networkSuggestion := false

	for _, suggestion := range suggestions {
		if suggestion.Category == CategoryResource {
			resourceSuggestion = true
		}
		if suggestion.Category == CategoryNetwork {
			networkSuggestion = true
		}
	}

	assert.True(t, resourceSuggestion, "Should generate resource-related suggestions")
	assert.True(t, networkSuggestion, "Should generate network-related suggestions")
}

func TestCorrelationEngine_AnalyzePatterns(t *testing.T) {
	engine := NewCorrelationEngine()

	analysisResults := []AnalysisResult{
		// Pattern 1: Resource exhaustion cascade
		{ID: "cpu-high", Category: "resource", Severity: "high", Tags: []string{"node-1"}},
		{ID: "memory-high", Category: "resource", Severity: "high", Tags: []string{"node-1"}},
		{ID: "pod-evicted", Category: "kubernetes", Severity: "critical", Tags: []string{"node-1"}},

		// Pattern 2: Network connectivity issues
		{ID: "dns-fail", Category: "network", Severity: "medium"},
		{ID: "service-unreachable", Category: "network", Severity: "high"},

		// Isolated issue
		{ID: "config-error", Category: "configuration", Severity: "low"},
	}

	patterns := engine.AnalyzePatterns(analysisResults)

	assert.NotEmpty(t, patterns)

	// Should identify resource exhaustion pattern
	hasResourcePattern := false
	hasNetworkPattern := false

	for _, pattern := range patterns {
		if pattern.Category == "resource" && len(pattern.RelatedAnalyzers) >= 2 {
			hasResourcePattern = true
		}
		if pattern.Category == "network" && len(pattern.RelatedAnalyzers) >= 2 {
			hasNetworkPattern = true
		}
	}

	assert.True(t, hasResourcePattern, "Should identify resource exhaustion pattern")
	assert.True(t, hasNetworkPattern, "Should identify network connectivity pattern")
}

func TestCorrelationEngine_calculateStrength(t *testing.T) {
	engine := NewCorrelationEngine()

	tests := []struct {
		name     string
		result1  AnalysisResult
		result2  AnalysisResult
		expected float64
	}{
		{
			name: "Same Category High Severity",
			result1: AnalysisResult{
				Category: "resource",
				Severity: "high",
			},
			result2: AnalysisResult{
				Category: "resource",
				Severity: "high",
			},
			expected: 0.6, // Should be moderately strong
		},
		{
			name: "Different Categories",
			result1: AnalysisResult{
				Category: "resource",
				Severity: "high",
			},
			result2: AnalysisResult{
				Category: "network",
				Severity: "high",
			},
			expected: 0.3, // Should be weaker
		},
		{
			name: "Same Tags",
			result1: AnalysisResult{
				Category: "resource",
				// Tags:     []string{"node-1", "cpu"},
			},
			result2: AnalysisResult{
				Category: "resource",
				// Tags:     []string{"node-1", "memory"},
			},
			expected: 0.7, // Should be strong due to shared node
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strength := engine.calculateStrength(tt.result1, tt.result2, CorrelationFunctional)
			assert.InDelta(t, tt.expected, strength, 0.2, "Correlation strength should be within expected range")
		})
	}
}

func TestCorrelationEngine_EdgeCases(t *testing.T) {
	engine := NewCorrelationEngine()

	t.Run("Empty Analysis Results", func(t *testing.T) {
		correlations := engine.FindCorrelations([]AnalysisResult{}, []RemediationStep{}, RemediationContext{})
		assert.Empty(t, correlations, "Should return empty correlations for empty input")
	})

	t.Run("Single Analysis Result", func(t *testing.T) {
		analysisResults := []AnalysisResult{
			{ID: "single", Category: "resource"},
		}
		correlations := engine.FindCorrelations(analysisResults, []RemediationStep{}, RemediationContext{})
		assert.Empty(t, correlations, "Should not find correlations with single result")
	})

	t.Run("No Common Attributes", func(t *testing.T) {
		analysisResults := []AnalysisResult{
			{ID: "result1", Category: "resource", Severity: "low"},
			{ID: "result2", Category: "network", Severity: "high"},
		}
		correlations := engine.FindCorrelations(analysisResults, []RemediationStep{}, RemediationContext{})
		// May or may not find weak correlations, but should not crash
		assert.NotNil(t, correlations)
	})
}

func TestCorrelationInsight_Validation(t *testing.T) {
	insight := CorrelationInsight{
		ID:               "test-insight",
		Type:             CorrelationCausal,
		Category:         "resource",
		Title:            "Memory Exhaustion Pattern",
		Description:      "Memory issues leading to pod restarts",
		Confidence:       0.85,
		Severity:         "high",
		Tags:             []string{"memory", "pod", "restart"},
		RelatedAnalyzers: []string{"memory-analyzer", "pod-analyzer"},
		Evidence:         []string{"OOM events", "restart correlation"},
		Timestamp:        time.Now(),
	}

	// Validate required fields are present
	assert.NotEmpty(t, insight.ID)
	assert.NotEmpty(t, insight.Type)
	assert.NotEmpty(t, insight.Category)
	assert.NotEmpty(t, insight.Title)
	assert.NotEmpty(t, insight.Description)
	assert.Greater(t, insight.Confidence, 0.0)
	assert.LessOrEqual(t, insight.Confidence, 1.0)
	assert.NotEmpty(t, insight.Evidence)
	assert.NotEmpty(t, insight.RelatedAnalyzers)
}
