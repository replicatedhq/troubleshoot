package remediation

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewConfidenceScorer(t *testing.T) {
	scorer := NewConfidenceScorer()

	assert.NotNil(t, scorer)
	assert.NotEmpty(t, scorer.factors)
	assert.Equal(t, 7, len(scorer.factors)) // Should have 7 default factors
	assert.NotNil(t, scorer.weights)
}

func TestConfidenceScorer_ScoreStep(t *testing.T) {
	scorer := NewConfidenceScorer()

	step := RemediationStep{
		ID:          "test-step",
		Title:       "Test Remediation Step",
		Description: "A test step for confidence scoring",
		Category:    CategoryConfiguration,
		Priority:    PriorityHigh,
		Impact:      ImpactMedium,
		Difficulty:  DifficultyEasy,
		Verification: &VerificationStep{
			Command: &CommandStep{Command: "kubectl get pods"},
		},
		Rollback: &RollbackStep{
			Command: &CommandStep{Command: "kubectl rollout undo"},
		},
		Documentation: []DocumentationLink{
			{
				Title: "Official Kubernetes Documentation",
				URL:   "https://kubernetes.io/docs/",
				Type:  DocTypeOfficial,
			},
		},
		Tags: []string{"official", "validated"},
	}

	context := RemediationContext{
		Environment: EnvironmentContext{
			Platform:      "kubernetes",
			CloudProvider: "aws",
			Version:       "1.21",
		},
		UserPreferences: UserPreferences{
			SkillLevel: SkillIntermediate,
		},
	}

	scoredStep := scorer.ScoreStep(step, context)

	// Check that confidence tag was added
	hasConfidenceTag := false
	for _, tag := range scoredStep.Tags {
		if contains([]string{tag}, "confidence:") {
			hasConfidenceTag = true
			break
		}
	}
	assert.True(t, hasConfidenceTag, "Should add confidence tag to step")
}

func TestConfidenceScorer_CalculateConfidence(t *testing.T) {
	scorer := NewConfidenceScorer()

	step := RemediationStep{
		Category:   CategoryConfiguration,
		Priority:   PriorityHigh,
		Impact:     ImpactMedium,
		Difficulty: DifficultyEasy,
		Verification: &VerificationStep{
			Command:        &CommandStep{Command: "kubectl get pods"},
			ExpectedOutput: "Running",
		},
		Rollback: &RollbackStep{
			Command: &CommandStep{Command: "kubectl rollout undo"},
		},
		Documentation: []DocumentationLink{
			{Type: DocTypeOfficial},
		},
		Tags:          []string{"official", "validated"},
		EstimatedTime: 10 * time.Minute,
		Prerequisites: []string{"kubectl access"},
	}

	context := RemediationContext{
		Environment: EnvironmentContext{
			Platform:     "kubernetes",
			Capabilities: []string{"kubectl"},
		},
		UserPreferences: UserPreferences{
			SkillLevel: SkillIntermediate,
		},
		AnalysisResults: []AnalysisResult{
			{Evidence: []string{"Log shows error"}},
		},
	}

	score := scorer.CalculateConfidence(step, context)

	assert.Greater(t, score.Overall, 0.0)
	assert.LessOrEqual(t, score.Overall, 1.0)
	assert.NotEmpty(t, score.Factors)
	assert.NotEmpty(t, score.Explanation)
	assert.Equal(t, "1.0.0", score.Metadata.Version)

	// Check that all expected factors are present
	expectedFactors := []string{
		"success_rate", "evidence_quality", "source_reliability",
		"context_match", "complexity", "test_coverage", "expert_validation",
	}
	for _, factor := range expectedFactors {
		_, exists := score.Factors[factor]
		assert.True(t, exists, "Should have factor: %s", factor)
	}
}

func TestSuccessRateFactor(t *testing.T) {
	factor := &SuccessRateFactor{}

	tests := []struct {
		name     string
		step     RemediationStep
		context  RemediationContext
		minScore float64
		maxScore float64
	}{
		{
			name: "Easy Configuration Step",
			step: RemediationStep{
				Difficulty: DifficultyEasy,
				Category:   CategoryConfiguration,
				Command:    &CommandStep{},
			},
			minScore: 0.8,
			maxScore: 1.0,
		},
		{
			name: "Expert Network Step",
			step: RemediationStep{
				Difficulty: DifficultyExpert,
				Category:   CategoryNetwork,
				Manual:     &ManualStep{},
			},
			minScore: 0.0,
			maxScore: 0.5,
		},
		{
			name: "Moderate Security Step with Command",
			step: RemediationStep{
				Difficulty: DifficultyModerate,
				Category:   CategorySecurity,
				Command:    &CommandStep{},
			},
			minScore: 0.5,
			maxScore: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := factor.Calculate(tt.step, tt.context)
			assert.GreaterOrEqual(t, score, tt.minScore, "Score should be at least %f", tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore, "Score should be at most %f", tt.maxScore)
		})
	}
}

func TestEvidenceQualityFactor(t *testing.T) {
	factor := &EvidenceQualityFactor{}

	tests := []struct {
		name     string
		step     RemediationStep
		context  RemediationContext
		minScore float64
	}{
		{
			name: "Step with Verification and Rollback",
			step: RemediationStep{
				Verification: &VerificationStep{},
				Rollback:     &RollbackStep{},
				Documentation: []DocumentationLink{
					{Type: DocTypeOfficial},
				},
			},
			context: RemediationContext{
				AnalysisResults: []AnalysisResult{
					{Evidence: []string{"Error log"}},
				},
			},
			minScore: 0.8,
		},
		{
			name: "Basic Step",
			step: RemediationStep{},
			context: RemediationContext{
				AnalysisResults: []AnalysisResult{},
			},
			minScore: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := factor.Calculate(tt.step, tt.context)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, 1.0)
		})
	}
}

func TestSourceReliabilityFactor(t *testing.T) {
	factor := &SourceReliabilityFactor{}

	step := RemediationStep{
		Documentation: []DocumentationLink{
			{Type: DocTypeOfficial},
		},
		Verification: &VerificationStep{},
		Tags:         []string{"official", "builtin"},
	}

	context := RemediationContext{
		Environment: EnvironmentContext{
			Platform: "kubernetes",
		},
	}

	score := factor.Calculate(step, context)

	assert.Greater(t, score, 0.8, "Should have high reliability score with official docs and verification")
	assert.LessOrEqual(t, score, 1.0)
}

func TestContextMatchFactor(t *testing.T) {
	factor := &ContextMatchFactor{}

	step := RemediationStep{
		Description: "Update Kubernetes deployment configuration",
		Tags:        []string{"kubernetes", "kubectl"},
	}

	context := RemediationContext{
		Environment: EnvironmentContext{
			Platform:      "kubernetes",
			CloudProvider: "aws",
		},
	}

	score := factor.Calculate(step, context)

	assert.Greater(t, score, 0.5, "Should have decent context match with matching platform and capabilities")
	assert.LessOrEqual(t, score, 1.0)
}

func TestComplexityFactor(t *testing.T) {
	factor := &ComplexityFactor{}

	tests := []struct {
		name     string
		step     RemediationStep
		minScore float64
		maxScore float64
	}{
		{
			name: "Simple Easy Step",
			step: RemediationStep{
				Difficulty:    DifficultyEasy,
				EstimatedTime: 5 * time.Minute,
				Dependencies:  []string{},
				Command:       &CommandStep{},
			},
			minScore: 0.8,
			maxScore: 1.0,
		},
		{
			name: "Complex Expert Step",
			step: RemediationStep{
				Difficulty:    DifficultyExpert,
				EstimatedTime: 2 * time.Hour,
				Dependencies:  []string{"dep1", "dep2", "dep3", "dep4"},
				Command:       &CommandStep{},
				Manual:        &ManualStep{},
				Script:        &ScriptStep{},
			},
			minScore: 0.0,
			maxScore: 0.4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := factor.Calculate(tt.step, RemediationContext{})
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore)
		})
	}
}

func TestTestCoverageFactor(t *testing.T) {
	factor := &TestCoverageFactor{}

	step := RemediationStep{
		Verification: &VerificationStep{
			Command: &CommandStep{
				Command: "kubectl get pods",
			},
			ExpectedOutput: "Running",
		},
		Rollback:      &RollbackStep{},
		Prerequisites: []string{"kubectl access"},
	}

	score := factor.Calculate(step, RemediationContext{})

	assert.Greater(t, score, 0.8, "Should have high test coverage score with verification, rollback, and prerequisites")
	assert.LessOrEqual(t, score, 1.0)
}

func TestExpertValidationFactor(t *testing.T) {
	factor := &ExpertValidationFactor{}

	tests := []struct {
		name     string
		step     RemediationStep
		context  RemediationContext
		minScore float64
	}{
		{
			name: "Official Documentation with Beginner User",
			step: RemediationStep{
				Documentation: []DocumentationLink{
					{Type: DocTypeOfficial},
				},
				Tags:       []string{"validated", "official"},
				Difficulty: DifficultyHard,
			},
			context: RemediationContext{
				UserPreferences: UserPreferences{
					SkillLevel: SkillBeginner,
				},
			},
			minScore: 0.4, // Reduced due to skill mismatch
		},
		{
			name: "Expert User with Easy Step",
			step: RemediationStep{
				Documentation: []DocumentationLink{
					{Type: DocTypeOfficial},
				},
				Tags:       []string{"recommended"},
				Difficulty: DifficultyEasy,
			},
			context: RemediationContext{
				UserPreferences: UserPreferences{
					SkillLevel: SkillExpert,
				},
			},
			minScore: 0.9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := factor.Calculate(tt.step, tt.context)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, 1.0)
		})
	}
}

func TestConfidenceScorer_generateExplanation(t *testing.T) {
	scorer := NewConfidenceScorer()

	factorScores := map[string]float64{
		"success_rate":       0.9,
		"evidence_quality":   0.8,
		"source_reliability": 0.7,
		"context_match":      0.3,
		"complexity":         0.6,
		"test_coverage":      0.8,
		"expert_validation":  0.5,
	}

	details := ConfidenceDetails{
		EvidenceStrength:  0.8,
		HistoricalSuccess: 0.9,
	}

	explanation := scorer.generateExplanation(0.75, factorScores, details)

	assert.NotEmpty(t, explanation)
	assert.Contains(t, explanation, "confidence")
	assert.Contains(t, explanation, "Strongest factor")
	assert.Contains(t, explanation, "Weakest factor")
}

func TestConfidenceScorer_generateWarnings(t *testing.T) {
	scorer := NewConfidenceScorer()

	step := RemediationStep{
		EstimatedTime: 3 * time.Hour, // Long execution time
		Rollback:      nil,           // No rollback
		Verification:  nil,           // No verification
	}

	factorScores := map[string]float64{
		"success_rate":      0.3, // Low success rate
		"evidence_quality":  0.3, // Low evidence quality
		"context_match":     0.4, // Low context match
		"complexity":        0.2, // High complexity (low score)
		"test_coverage":     0.3, // Low test coverage
		"expert_validation": 0.6,
	}

	warnings := scorer.generateWarnings(step, 0.4, factorScores)

	assert.NotEmpty(t, warnings)

	// Should have specific warnings
	hasOverallWarning := false
	hasSuccessRateWarning := false
	hasNoRollbackWarning := false
	hasLongTimeWarning := false

	for _, warning := range warnings {
		if contains([]string{warning}, "below 50%") {
			hasOverallWarning = true
		}
		if contains([]string{warning}, "success rate") {
			hasSuccessRateWarning = true
		}
		if contains([]string{warning}, "rollback") {
			hasNoRollbackWarning = true
		}
		if contains([]string{warning}, "Long execution time") {
			hasLongTimeWarning = true
		}
	}

	assert.True(t, hasOverallWarning, "Should warn about low overall confidence")
	assert.True(t, hasSuccessRateWarning, "Should warn about low success rate")
	assert.True(t, hasNoRollbackWarning, "Should warn about no rollback")
	assert.True(t, hasLongTimeWarning, "Should warn about long execution time")
}

func TestConfidenceWeights(t *testing.T) {
	scorer := NewConfidenceScorer()

	// Verify weights sum to reasonable total (around 1.0)
	totalWeight := scorer.weights.SuccessRate +
		scorer.weights.EvidenceQuality +
		scorer.weights.SourceReliability +
		scorer.weights.ContextMatch +
		scorer.weights.Complexity +
		scorer.weights.TestCoverage +
		scorer.weights.ExpertValidation

	assert.InDelta(t, 1.0, totalWeight, 0.01, "Weights should sum to approximately 1.0")
}

func TestFactorWeights(t *testing.T) {
	scorer := NewConfidenceScorer()

	// Test that each factor returns its expected weight
	expectedWeights := map[string]float64{
		"success_rate":       0.25,
		"evidence_quality":   0.20,
		"source_reliability": 0.15,
		"context_match":      0.15,
		"complexity":         0.10,
		"test_coverage":      0.10,
		"expert_validation":  0.05,
	}

	for _, factor := range scorer.factors {
		expectedWeight := expectedWeights[factor.Name()]
		assert.Equal(t, expectedWeight, factor.Weight(), "Factor %s should have weight %f", factor.Name(), expectedWeight)
	}
}

