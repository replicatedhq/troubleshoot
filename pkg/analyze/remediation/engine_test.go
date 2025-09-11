package remediation

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRemediationEngine(t *testing.T) {
	engine := NewRemediationEngine()

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.prioritizer)
	assert.NotNil(t, engine.categorizer)
	assert.NotNil(t, engine.executor)
	assert.NotNil(t, engine.correlation)
	assert.NotNil(t, engine.confidence)
	assert.NotEmpty(t, engine.providers)
	assert.NotEmpty(t, engine.templates)
}

func TestGenerateRemediation_BasicFunctionality(t *testing.T) {
	engine := NewRemediationEngine()

	analysisResults := []AnalysisResult{
		{
			ID:          "test-1",
			Title:       "CPU Usage High",
			Description: "CPU usage is above 90%",
			Category:    "resource",
			Severity:    "high",
			Status:      "fail",
			Impact:      ImpactHigh,
			Confidence:  0.9,
			Evidence:    []string{"CPU metrics show sustained high usage"},
			Timestamp:   time.Now(),
		},
		{
			ID:          "test-2",
			Title:       "Storage Space Low",
			Description: "Disk usage is above 85%",
			Category:    "storage",
			Severity:    "medium",
			Status:      "warn",
			Impact:      ImpactMedium,
			Confidence:  0.8,
			Evidence:    []string{"Storage metrics show low available space"},
			Timestamp:   time.Now(),
		},
	}

	options := RemediationOptions{
		IncludeManual:    true,
		IncludeAutomated: true,
		MaxSteps:         10,
		MaxTime:          time.Hour,
		MinPriority:      PriorityLow,
		MaxDifficulty:    DifficultyExpert,
		SkillLevel:       SkillIntermediate,
		DryRun:           true,
	}

	result, err := engine.GenerateRemediation(context.Background(), analysisResults, options)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Steps)
	assert.NotEmpty(t, result.Plans)
	assert.NotNil(t, result.Summary)

	// Verify steps were generated for both analysis results
	resourceSteps := 0
	storageSteps := 0
	for _, step := range result.Steps {
		if step.Category == CategoryResource {
			resourceSteps++
		}
		if step.Category == CategoryStorage {
			storageSteps++
		}
	}
	assert.Greater(t, resourceSteps, 0, "Should have resource remediation steps")
	assert.Greater(t, storageSteps, 0, "Should have storage remediation steps")
}

func TestGenerateRemediation_Filtering(t *testing.T) {
	engine := NewRemediationEngine()

	analysisResults := []AnalysisResult{
		{
			ID:          "test-1",
			Title:       "CPU Usage High",
			Description: "CPU usage is above 90%",
			Category:    "resource",
			Severity:    "high",
			Status:      "fail",
		},
		{
			ID:          "test-2",
			Title:       "Security Policy Missing",
			Description: "RBAC configuration is incomplete",
			Category:    "security",
			Severity:    "critical",
			Status:      "fail",
		},
	}

	// Test with excluded categories
	options := RemediationOptions{
		IncludeManual:      true,
		IncludeAutomated:   true,
		ExcludedCategories: []RemediationCategory{CategorySecurity},
		MaxSteps:           10,
	}

	result, err := engine.GenerateRemediation(context.Background(), analysisResults, options)

	require.NoError(t, err)

	// Verify no security steps were generated
	for _, step := range result.Steps {
		assert.NotEqual(t, CategorySecurity, step.Category, "Security steps should be excluded")
	}
}

func TestGenerateRemediation_MaxStepsLimit(t *testing.T) {
	engine := NewRemediationEngine()

	analysisResults := []AnalysisResult{
		{
			ID:          "test-1",
			Title:       "Resource Issue 1",
			Description: "CPU usage is high",
			Category:    "resource",
			Severity:    "high",
		},
		{
			ID:          "test-2",
			Title:       "Resource Issue 2",
			Description: "Memory usage is high",
			Category:    "resource",
			Severity:    "high",
		},
		{
			ID:          "test-3",
			Title:       "Storage Issue",
			Description: "Disk usage is high",
			Category:    "storage",
			Severity:    "medium",
		},
	}

	options := RemediationOptions{
		IncludeManual:    true,
		IncludeAutomated: true,
		MaxSteps:         2, // Limit to 2 steps
	}

	result, err := engine.GenerateRemediation(context.Background(), analysisResults, options)

	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.Steps), 2, "Should respect max steps limit")
}

func TestGenerateResourceSteps(t *testing.T) {
	engine := NewRemediationEngine()

	result := AnalysisResult{
		ID:          "cpu-test",
		Title:       "High CPU Usage",
		Description: "CPU usage exceeds 90%",
		Category:    "resource",
		Severity:    "high",
	}

	steps := engine.generateResourceSteps(result)

	assert.NotEmpty(t, steps)
	assert.Equal(t, CategoryResource, steps[0].Category)
	assert.Equal(t, "Scale Resources", steps[0].Title)
	assert.NotNil(t, steps[0].Command)
	assert.NotNil(t, steps[0].Verification)
}

func TestGenerateStorageSteps(t *testing.T) {
	engine := NewRemediationEngine()

	result := AnalysisResult{
		ID:          "storage-test",
		Title:       "Low Storage Space",
		Description: "Disk usage is above 85%",
		Category:    "storage",
		Severity:    "medium",
	}

	steps := engine.generateStorageSteps(result)

	assert.NotEmpty(t, steps)
	assert.Equal(t, CategoryStorage, steps[0].Category)
	assert.Equal(t, "Expand Storage", steps[0].Title)
	assert.NotNil(t, steps[0].Manual)
	assert.NotEmpty(t, steps[0].Manual.Instructions)
	assert.NotEmpty(t, steps[0].Manual.Checklist)
	assert.NotEmpty(t, steps[0].Documentation)
}

func TestGenerateNetworkSteps(t *testing.T) {
	engine := NewRemediationEngine()

	result := AnalysisResult{
		ID:          "network-test",
		Title:       "Network Connectivity Issue",
		Description: "Network connectivity is failing",
		Category:    "network",
		Severity:    "high",
	}

	steps := engine.generateNetworkSteps(result)

	assert.NotEmpty(t, steps)
	assert.Equal(t, CategoryNetwork, steps[0].Category)
	assert.Equal(t, "Fix Network Connectivity", steps[0].Title)
	assert.NotNil(t, steps[0].Script)
	assert.Equal(t, LanguageBash, steps[0].Script.Language)
}

func TestGenerateSecuritySteps(t *testing.T) {
	engine := NewRemediationEngine()

	result := AnalysisResult{
		ID:          "security-test",
		Title:       "RBAC Configuration Missing",
		Description: "RBAC policies are not properly configured",
		Category:    "security",
		Severity:    "critical",
	}

	steps := engine.generateSecuritySteps(result)

	assert.NotEmpty(t, steps)
	assert.Equal(t, CategorySecurity, steps[0].Category)
	assert.Equal(t, "Apply Security Policy", steps[0].Title)
	assert.Equal(t, PriorityCritical, steps[0].Priority)
	assert.Equal(t, ImpactHigh, steps[0].Impact)
	assert.Equal(t, DifficultyHard, steps[0].Difficulty)
	assert.NotNil(t, steps[0].Manual)
	assert.NotEmpty(t, steps[0].Documentation)
}

func TestGenerateConfigurationSteps(t *testing.T) {
	engine := NewRemediationEngine()

	result := AnalysisResult{
		ID:          "config-test",
		Title:       "Invalid Configuration",
		Description: "Application configuration has invalid values",
		Category:    "configuration",
		Severity:    "medium",
	}

	steps := engine.generateConfigurationSteps(result)

	assert.NotEmpty(t, steps)
	assert.Equal(t, CategoryConfiguration, steps[0].Category)
	assert.Equal(t, "Update Configuration", steps[0].Title)
	assert.NotNil(t, steps[0].Manual)
	assert.NotEmpty(t, steps[0].Manual.Instructions)
}

func TestFilterSteps_Categories(t *testing.T) {
	engine := NewRemediationEngine()

	steps := []RemediationStep{
		{Category: CategoryResource, Priority: PriorityHigh, Difficulty: DifficultyEasy},
		{Category: CategorySecurity, Priority: PriorityHigh, Difficulty: DifficultyEasy},
		{Category: CategoryStorage, Priority: PriorityHigh, Difficulty: DifficultyEasy},
	}

	// Test allowed categories
	options := RemediationOptions{
		AllowedCategories: []RemediationCategory{CategoryResource, CategoryStorage},
		IncludeManual:     true,
		IncludeAutomated:  true,
	}

	filtered := engine.filterSteps(steps, options)

	assert.Len(t, filtered, 2)
	for _, step := range filtered {
		assert.True(t, step.Category == CategoryResource || step.Category == CategoryStorage)
	}
}

func TestFilterSteps_Priority(t *testing.T) {
	engine := NewRemediationEngine()

	steps := []RemediationStep{
		{Category: CategoryResource, Priority: PriorityLow, Difficulty: DifficultyEasy},
		{Category: CategoryResource, Priority: PriorityMedium, Difficulty: DifficultyEasy},
		{Category: CategoryResource, Priority: PriorityHigh, Difficulty: DifficultyEasy},
	}

	options := RemediationOptions{
		MinPriority:      PriorityMedium,
		IncludeManual:    true,
		IncludeAutomated: true,
	}

	filtered := engine.filterSteps(steps, options)

	assert.Len(t, filtered, 2) // Medium and High priority steps
	for _, step := range filtered {
		assert.True(t, engine.isPriorityHigherOrEqual(step.Priority, PriorityMedium))
	}
}

func TestFilterSteps_Difficulty(t *testing.T) {
	engine := NewRemediationEngine()

	steps := []RemediationStep{
		{Category: CategoryResource, Priority: PriorityHigh, Difficulty: DifficultyEasy},
		{Category: CategoryResource, Priority: PriorityHigh, Difficulty: DifficultyModerate},
		{Category: CategoryResource, Priority: PriorityHigh, Difficulty: DifficultyExpert},
	}

	options := RemediationOptions{
		MaxDifficulty:    DifficultyModerate,
		IncludeManual:    true,
		IncludeAutomated: true,
	}

	filtered := engine.filterSteps(steps, options)

	assert.Len(t, filtered, 2) // Easy and Moderate difficulty steps
	for _, step := range filtered {
		assert.True(t, engine.isDifficultyLowerOrEqual(step.Difficulty, DifficultyModerate))
	}
}

func TestMapSeverityToPriority(t *testing.T) {
	engine := NewRemediationEngine()

	tests := []struct {
		severity string
		expected RemediationPriority
	}{
		{"critical", PriorityCritical},
		{"high", PriorityHigh},
		{"medium", PriorityMedium},
		{"low", PriorityLow},
		{"unknown", PriorityInfo},
		{"", PriorityInfo},
	}

	for _, tt := range tests {
		result := engine.mapSeverityToPriority(tt.severity)
		assert.Equal(t, tt.expected, result, "Severity %s should map to priority %s", tt.severity, tt.expected)
	}
}

func TestInferEnvironmentContext(t *testing.T) {
	engine := NewRemediationEngine()

	tests := []struct {
		name     string
		results  []AnalysisResult
		expected string
	}{
		{
			name: "AWS Environment",
			results: []AnalysisResult{
				{Description: "AWS EKS cluster configuration issue"},
			},
			expected: "aws",
		},
		{
			name: "Azure Environment",
			results: []AnalysisResult{
				{Description: "Azure AKS cluster performance issue"},
			},
			expected: "azure",
		},
		{
			name: "GCP Environment",
			results: []AnalysisResult{
				{Description: "GCP GKE cluster networking problem"},
			},
			expected: "gcp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := engine.inferEnvironmentContext(tt.results)
			assert.Equal(t, tt.expected, env.CloudProvider)
			assert.Equal(t, "kubernetes", env.Platform)
			assert.Contains(t, env.Capabilities, "kubectl")
		})
	}
}

func TestGenerateInsights(t *testing.T) {
	engine := NewRemediationEngine()

	results := []AnalysisResult{
		{
			Category: "resource",
			Title:    "High CPU Usage",
		},
		{
			Category: "resource",
			Title:    "High Memory Usage",
		},
		{
			Category: "resource",
			Title:    "Low Disk Space",
		},
	}

	steps := []RemediationStep{
		{Category: CategoryResource},
		{Category: CategoryResource},
	}

	correlations := []Correlation{
		{
			Strength:    0.8,
			Confidence:  0.9,
			Type:        CorrelationCausal,
			Description: "High resource usage correlation",
		},
	}

	insights := engine.generateInsights(context.Background(), results, steps, correlations)

	assert.NotEmpty(t, insights)

	// Should have pattern insight for multiple resource issues
	hasPatternInsight := false
	hasCorrelationInsight := false

	for _, insight := range insights {
		if insight.Type == InsightPattern {
			hasPatternInsight = true
		}
		if insight.Type == InsightCorrelation {
			hasCorrelationInsight = true
		}
	}

	assert.True(t, hasPatternInsight, "Should generate pattern insight for multiple resource issues")
	assert.True(t, hasCorrelationInsight, "Should generate correlation insight for strong correlation")
}

func TestCreateRemediationPlans(t *testing.T) {
	engine := NewRemediationEngine()

	steps := []RemediationStep{
		{
			Category:      CategoryResource,
			Priority:      PriorityHigh,
			EstimatedTime: 10 * time.Minute,
		},
		{
			Category:      CategoryResource,
			Priority:      PriorityMedium,
			EstimatedTime: 15 * time.Minute,
		},
		{
			Category:      CategorySecurity,
			Priority:      PriorityCritical,
			EstimatedTime: 30 * time.Minute,
		},
	}

	plans := engine.createRemediationPlans(steps, []Correlation{})

	assert.NotEmpty(t, plans)

	// Should have plans for both resource and security categories
	resourcePlan := false
	securityPlan := false

	for _, plan := range plans {
		if len(plan.Categories) > 0 {
			if plan.Categories[0] == CategoryResource {
				resourcePlan = true
				assert.Len(t, plan.Steps, 2)                    // Two resource steps
				assert.Equal(t, 25*time.Minute, plan.TotalTime) // 10 + 15 minutes
				assert.Equal(t, PriorityHigh, plan.Priority)    // Highest priority step
			}
			if plan.Categories[0] == CategorySecurity {
				securityPlan = true
				assert.Len(t, plan.Steps, 1) // One security step
				assert.Equal(t, 30*time.Minute, plan.TotalTime)
				assert.Equal(t, PriorityCritical, plan.Priority)
			}
		}
	}

	assert.True(t, resourcePlan, "Should create resource remediation plan")
	assert.True(t, securityPlan, "Should create security remediation plan")
}

func TestGenerationSummary(t *testing.T) {
	engine := NewRemediationEngine()

	result := &RemediationGenerationResult{
		Steps: []RemediationStep{
			{
				Category:      CategoryResource,
				Priority:      PriorityHigh,
				Difficulty:    DifficultyEasy,
				EstimatedTime: 10 * time.Minute,
				Command:       &CommandStep{},
			},
			{
				Category:      CategorySecurity,
				Priority:      PriorityCritical,
				Difficulty:    DifficultyHard,
				EstimatedTime: 30 * time.Minute,
				Manual:        &ManualStep{},
			},
		},
		Insights: []RemediationInsight{
			{Type: InsightPattern},
		},
		Correlations: []Correlation{
			{Type: CorrelationCausal},
		},
	}

	generationTime := 500 * time.Millisecond
	summary := engine.generateSummary(result, generationTime)

	assert.Equal(t, 2, summary.TotalSteps)
	assert.Equal(t, 1, summary.StepsByCategory[CategoryResource])
	assert.Equal(t, 1, summary.StepsByCategory[CategorySecurity])
	assert.Equal(t, 1, summary.StepsByPriority[PriorityHigh])
	assert.Equal(t, 1, summary.StepsByPriority[PriorityCritical])
	assert.Equal(t, 1, summary.StepsByDifficulty[DifficultyEasy])
	assert.Equal(t, 1, summary.StepsByDifficulty[DifficultyHard])
	assert.Equal(t, 40*time.Minute, summary.EstimatedTime)
	assert.Equal(t, 1, summary.AutomatableSteps)
	assert.Equal(t, 1, summary.ManualSteps)
	assert.Equal(t, 1, summary.CriticalSteps)
	assert.Equal(t, generationTime, summary.GenerationTime)
	assert.Equal(t, 1, summary.InsightCount)
	assert.Equal(t, 1, summary.CorrelationCount)
}

func TestUtilityFunctions(t *testing.T) {
	engine := NewRemediationEngine()

	t.Run("containsCategory", func(t *testing.T) {
		categories := []RemediationCategory{CategoryResource, CategorySecurity}
		assert.True(t, engine.containsCategory(categories, CategoryResource))
		assert.True(t, engine.containsCategory(categories, CategorySecurity))
		assert.False(t, engine.containsCategory(categories, CategoryStorage))
	})

	t.Run("isPriorityHigherOrEqual", func(t *testing.T) {
		assert.True(t, engine.isPriorityHigherOrEqual(PriorityCritical, PriorityHigh))
		assert.True(t, engine.isPriorityHigherOrEqual(PriorityHigh, PriorityHigh))
		assert.False(t, engine.isPriorityHigherOrEqual(PriorityMedium, PriorityHigh))
	})

	t.Run("isDifficultyLowerOrEqual", func(t *testing.T) {
		assert.True(t, engine.isDifficultyLowerOrEqual(DifficultyEasy, DifficultyModerate))
		assert.True(t, engine.isDifficultyLowerOrEqual(DifficultyModerate, DifficultyModerate))
		assert.False(t, engine.isDifficultyLowerOrEqual(DifficultyHard, DifficultyModerate))
	})

	t.Run("calculatePlanImpact", func(t *testing.T) {
		highImpactSteps := []RemediationStep{
			{Impact: ImpactHigh},
			{Impact: ImpactMedium},
		}
		assert.Equal(t, ImpactHigh, engine.calculatePlanImpact(highImpactSteps))

		mediumImpactSteps := []RemediationStep{
			{Impact: ImpactMedium},
			{Impact: ImpactLow},
		}
		assert.Equal(t, ImpactMedium, engine.calculatePlanImpact(mediumImpactSteps))

		lowImpactSteps := []RemediationStep{
			{Impact: ImpactLow},
		}
		assert.Equal(t, ImpactLow, engine.calculatePlanImpact(lowImpactSteps))
	})
}
