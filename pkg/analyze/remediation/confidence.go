package remediation

import (
	"fmt"
	"strings"
	"time"
)

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ConfidenceScorer calculates confidence scores for remediation steps and analysis results
type ConfidenceScorer struct {
	factors []ConfidenceFactor
	weights ConfidenceWeights
}

// ConfidenceFactor represents a factor that influences confidence scoring
type ConfidenceFactor interface {
	Name() string
	Calculate(step RemediationStep, context RemediationContext) float64
	Weight() float64
}

// ConfidenceWeights defines weights for different confidence factors
type ConfidenceWeights struct {
	SuccessRate       float64 `json:"success_rate"`       // Historical success rate
	EvidenceQuality   float64 `json:"evidence_quality"`   // Quality of supporting evidence
	SourceReliability float64 `json:"source_reliability"` // Reliability of information source
	ContextMatch      float64 `json:"context_match"`      // How well context matches
	Complexity        float64 `json:"complexity"`         // Complexity of the remediation
	TestCoverage      float64 `json:"test_coverage"`      // Test coverage for verification
	ExpertValidation  float64 `json:"expert_validation"`  // Expert review/validation
}

// ConfidenceScore represents a detailed confidence assessment
type ConfidenceScore struct {
	Overall     float64            `json:"overall"`     // Overall confidence (0-1)
	Factors     map[string]float64 `json:"factors"`     // Individual factor scores
	Details     ConfidenceDetails  `json:"details"`     // Detailed breakdown
	Explanation string             `json:"explanation"` // Human-readable explanation
	Warnings    []string           `json:"warnings"`    // Confidence warnings
	Metadata    ConfidenceMetadata `json:"metadata"`    // Additional metadata
}

// ConfidenceDetails provides detailed confidence information
type ConfidenceDetails struct {
	EvidenceStrength  float64 `json:"evidence_strength"`  // Strength of supporting evidence
	HistoricalSuccess float64 `json:"historical_success"` // Historical success rate
	EnvironmentMatch  float64 `json:"environment_match"`  // Environment compatibility
	ComplexityPenalty float64 `json:"complexity_penalty"` // Penalty for complexity
	VerificationScore float64 `json:"verification_score"` // Verification capability score
	RiskAssessment    float64 `json:"risk_assessment"`    // Risk assessment score
}

// ConfidenceMetadata contains metadata about confidence calculation
type ConfidenceMetadata struct {
	CalculatedAt time.Time `json:"calculated_at"`
	Version      string    `json:"version"`
	FactorsUsed  []string  `json:"factors_used"`
	DataSources  []string  `json:"data_sources"`
	Assumptions  []string  `json:"assumptions"`
	Limitations  []string  `json:"limitations"`
}

// NewConfidenceScorer creates a new confidence scorer
func NewConfidenceScorer() *ConfidenceScorer {
	return &ConfidenceScorer{
		factors: []ConfidenceFactor{
			&SuccessRateFactor{},
			&EvidenceQualityFactor{},
			&SourceReliabilityFactor{},
			&ContextMatchFactor{},
			&ComplexityFactor{},
			&TestCoverageFactor{},
			&ExpertValidationFactor{},
		},
		weights: ConfidenceWeights{
			SuccessRate:       0.25,
			EvidenceQuality:   0.20,
			SourceReliability: 0.15,
			ContextMatch:      0.15,
			Complexity:        0.10,
			TestCoverage:      0.10,
			ExpertValidation:  0.05,
		},
	}
}

// ScoreStep calculates confidence score for a remediation step
func (cs *ConfidenceScorer) ScoreStep(step RemediationStep, context RemediationContext) RemediationStep {
	score := cs.CalculateConfidence(step, context)

	// Add confidence score to the step's tags
	step.Tags = append(step.Tags, fmt.Sprintf("confidence:%.2f", score.Overall))

	return step
}

// CalculateConfidence calculates detailed confidence score
func (cs *ConfidenceScorer) CalculateConfidence(step RemediationStep, context RemediationContext) ConfidenceScore {
	factorScores := make(map[string]float64)
	var weightedSum float64
	var totalWeight float64

	// Calculate scores for each factor
	for _, factor := range cs.factors {
		score := factor.Calculate(step, context)
		weight := factor.Weight()
		factorScores[factor.Name()] = score
		weightedSum += score * weight
		totalWeight += weight
	}

	// Calculate overall score
	overall := weightedSum / totalWeight
	if overall > 1.0 {
		overall = 1.0
	}
	if overall < 0.0 {
		overall = 0.0
	}

	// Generate detailed confidence information
	details := cs.calculateDetails(step, context, factorScores)
	explanation := cs.generateExplanation(overall, factorScores, details)
	warnings := cs.generateWarnings(step, overall, factorScores)

	return ConfidenceScore{
		Overall:     overall,
		Factors:     factorScores,
		Details:     details,
		Explanation: explanation,
		Warnings:    warnings,
		Metadata: ConfidenceMetadata{
			CalculatedAt: time.Now(),
			Version:      "1.0.0",
			FactorsUsed:  cs.getFactorNames(),
			DataSources:  []string{"analysis_results", "remediation_history", "context"},
			Assumptions:  []string{"historical_data_reliability", "environment_similarity"},
			Limitations:  []string{"limited_historical_data", "context_inference"},
		},
	}
}

// SuccessRateFactor calculates confidence based on historical success rate
type SuccessRateFactor struct{}

func (f *SuccessRateFactor) Name() string {
	return "success_rate"
}

func (f *SuccessRateFactor) Weight() float64 {
	return 0.25
}

func (f *SuccessRateFactor) Calculate(step RemediationStep, context RemediationContext) float64 {
	// In a real implementation, this would query historical data
	// For now, we'll use heuristics based on step characteristics

	baseScore := 0.7 // Default success rate assumption

	// Adjust based on difficulty
	switch step.Difficulty {
	case DifficultyEasy:
		baseScore = 0.9
	case DifficultyModerate:
		baseScore = 0.8
	case DifficultyHard:
		baseScore = 0.6
	case DifficultyExpert:
		baseScore = 0.4
	}

	// Adjust based on category (some categories are more reliable)
	switch step.Category {
	case CategoryConfiguration:
		baseScore += 0.1 // Configuration changes are usually reliable
	case CategoryResource:
		baseScore += 0.05 // Resource changes are moderately reliable
	case CategorySecurity:
		baseScore -= 0.05 // Security changes can be tricky
	case CategoryNetwork:
		baseScore -= 0.1 // Network changes can have side effects
	}

	// Adjust based on automation level
	if step.Command != nil || step.Script != nil {
		baseScore += 0.1 // Automated steps are more reliable
	}
	if step.Manual != nil {
		baseScore -= 0.05 // Manual steps have human error risk
	}

	// Ensure within bounds
	if baseScore > 1.0 {
		baseScore = 1.0
	}
	if baseScore < 0.0 {
		baseScore = 0.0
	}

	return baseScore
}

// EvidenceQualityFactor calculates confidence based on quality of evidence
type EvidenceQualityFactor struct{}

func (f *EvidenceQualityFactor) Name() string {
	return "evidence_quality"
}

func (f *EvidenceQualityFactor) Weight() float64 {
	return 0.20
}

func (f *EvidenceQualityFactor) Calculate(step RemediationStep, context RemediationContext) float64 {
	score := 0.5 // Base evidence score

	// Check if there's verification capability
	if step.Verification != nil {
		score += 0.3
	}

	// Check if there's rollback capability
	if step.Rollback != nil {
		score += 0.2
	}

	// Check documentation quality
	if len(step.Documentation) > 0 {
		score += 0.1
		// Bonus for official documentation
		for _, doc := range step.Documentation {
			if doc.Type == DocTypeOfficial {
				score += 0.1
				break
			}
		}
	}

	// Check evidence from analysis results
	evidenceCount := 0
	for _, result := range context.AnalysisResults {
		if len(result.Evidence) > 0 {
			evidenceCount++
		}
	}

	if evidenceCount > 0 {
		evidenceRatio := float64(evidenceCount) / float64(len(context.AnalysisResults))
		score += evidenceRatio * 0.2
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}

// SourceReliabilityFactor calculates confidence based on source reliability
type SourceReliabilityFactor struct{}

func (f *SourceReliabilityFactor) Name() string {
	return "source_reliability"
}

func (f *SourceReliabilityFactor) Weight() float64 {
	return 0.15
}

func (f *SourceReliabilityFactor) Calculate(step RemediationStep, context RemediationContext) float64 {
	score := 0.6 // Base reliability score

	// Check for official documentation
	hasOfficialDocs := false
	for _, doc := range step.Documentation {
		if doc.Type == DocTypeOfficial {
			hasOfficialDocs = true
			break
		}
	}
	if hasOfficialDocs {
		score += 0.3
	}

	// Check for automated verification
	if step.Verification != nil {
		score += 0.2
	}

	// Check if it's a built-in remediation (more reliable)
	if contains(step.Tags, "builtin") || contains(step.Tags, "official") {
		score += 0.1
	}

	// Check environment reliability indicators
	if context.Environment.Platform != "" {
		score += 0.1 // Known platform is more reliable
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}

// ContextMatchFactor calculates confidence based on context matching
type ContextMatchFactor struct{}

func (f *ContextMatchFactor) Name() string {
	return "context_match"
}

func (f *ContextMatchFactor) Weight() float64 {
	return 0.15
}

func (f *ContextMatchFactor) Calculate(step RemediationStep, context RemediationContext) float64 {
	score := 0.5 // Base context match score

	// Check platform match
	if context.Environment.Platform != "" {
		// Check if step is compatible with platform
		stepDescription := strings.ToLower(step.Description + " " + strings.Join(step.Tags, " "))
		if strings.Contains(stepDescription, strings.ToLower(context.Environment.Platform)) {
			score += 0.2
		}
	}

	// Check cloud provider match
	if context.Environment.CloudProvider != "" {
		stepText := strings.ToLower(step.Description + " " + strings.Join(step.Tags, " "))
		if strings.Contains(stepText, strings.ToLower(context.Environment.CloudProvider)) {
			score += 0.2
		}
	}

	// Check capability match
	requiredCapabilities := f.extractRequiredCapabilities(step)
	matchingCapabilities := 0
	for _, required := range requiredCapabilities {
		for _, available := range context.Environment.Capabilities {
			if strings.Contains(strings.ToLower(available), strings.ToLower(required)) {
				matchingCapabilities++
				break
			}
		}
	}

	if len(requiredCapabilities) > 0 {
		capabilityMatch := float64(matchingCapabilities) / float64(len(requiredCapabilities))
		score += capabilityMatch * 0.3
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}

func (f *ContextMatchFactor) extractRequiredCapabilities(step RemediationStep) []string {
	var capabilities []string

	if step.Command != nil {
		capabilities = append(capabilities, step.Command.Command)
	}

	if step.Script != nil {
		switch step.Script.Language {
		case LanguageBash:
			capabilities = append(capabilities, "bash")
		case LanguagePowerShell:
			capabilities = append(capabilities, "powershell")
		case LanguagePython:
			capabilities = append(capabilities, "python")
		}
	}

	// Extract from tags
	for _, tag := range step.Tags {
		if strings.Contains(tag, "kubectl") || strings.Contains(tag, "kubernetes") {
			capabilities = append(capabilities, "kubectl")
		}
		if strings.Contains(tag, "helm") {
			capabilities = append(capabilities, "helm")
		}
		if strings.Contains(tag, "docker") {
			capabilities = append(capabilities, "docker")
		}
	}

	return capabilities
}

// ComplexityFactor calculates confidence penalty based on complexity
type ComplexityFactor struct{}

func (f *ComplexityFactor) Name() string {
	return "complexity"
}

func (f *ComplexityFactor) Weight() float64 {
	return 0.10
}

func (f *ComplexityFactor) Calculate(step RemediationStep, context RemediationContext) float64 {
	// Higher complexity reduces confidence
	baseScore := 1.0

	// Difficulty penalty
	switch step.Difficulty {
	case DifficultyEasy:
		baseScore -= 0.0
	case DifficultyModerate:
		baseScore -= 0.2
	case DifficultyHard:
		baseScore -= 0.4
	case DifficultyExpert:
		baseScore -= 0.6
	}

	// Multiple step types increase complexity
	stepTypes := 0
	if step.Command != nil {
		stepTypes++
	}
	if step.Manual != nil {
		stepTypes++
	}
	if step.Script != nil {
		stepTypes++
	}

	if stepTypes > 1 {
		baseScore -= 0.1 * float64(stepTypes-1)
	}

	// Long estimated time indicates complexity
	if step.EstimatedTime > time.Hour {
		baseScore -= 0.2
	} else if step.EstimatedTime > 30*time.Minute {
		baseScore -= 0.1
	}

	// Many dependencies increase complexity
	if len(step.Dependencies) > 3 {
		baseScore -= 0.1
	}

	if baseScore < 0.0 {
		baseScore = 0.0
	}

	return baseScore
}

// TestCoverageFactor calculates confidence based on test coverage
type TestCoverageFactor struct{}

func (f *TestCoverageFactor) Name() string {
	return "test_coverage"
}

func (f *TestCoverageFactor) Weight() float64 {
	return 0.10
}

func (f *TestCoverageFactor) Calculate(step RemediationStep, context RemediationContext) float64 {
	score := 0.3 // Base test coverage score

	// Check for verification step
	if step.Verification != nil {
		score += 0.4

		// Bonus for command-based verification (more reliable)
		if step.Verification.Command != nil {
			score += 0.1
		}

		// Bonus for expected output specification
		if step.Verification.ExpectedOutput != "" {
			score += 0.1
		}
	}

	// Check for rollback capability
	if step.Rollback != nil {
		score += 0.2
	}

	// Check for prerequisite validation
	if len(step.Prerequisites) > 0 {
		score += 0.1
	}

	if score > 1.0 {
		score = 1.0
	}

	return score
}

// ExpertValidationFactor calculates confidence based on expert validation
type ExpertValidationFactor struct{}

func (f *ExpertValidationFactor) Name() string {
	return "expert_validation"
}

func (f *ExpertValidationFactor) Weight() float64 {
	return 0.05
}

func (f *ExpertValidationFactor) Calculate(step RemediationStep, context RemediationContext) float64 {
	score := 0.5 // Base expert validation score

	// Check for official documentation (implies expert review)
	hasOfficialDocs := false
	for _, doc := range step.Documentation {
		if doc.Type == DocTypeOfficial {
			hasOfficialDocs = true
			break
		}
	}
	if hasOfficialDocs {
		score += 0.3
	}

	// Check for validation tags
	validationTags := []string{"validated", "reviewed", "official", "recommended"}
	for _, tag := range step.Tags {
		for _, validationTag := range validationTags {
			if strings.Contains(strings.ToLower(tag), validationTag) {
				score += 0.2
				break
			}
		}
	}

	// Check user skill level alignment
	skillPenalty := 0.0
	switch context.UserPreferences.SkillLevel {
	case SkillBeginner:
		if step.Difficulty == DifficultyHard || step.Difficulty == DifficultyExpert {
			skillPenalty = 0.3
		}
	case SkillIntermediate:
		if step.Difficulty == DifficultyExpert {
			skillPenalty = 0.1
		}
	}
	score -= skillPenalty

	if score > 1.0 {
		score = 1.0
	}
	if score < 0.0 {
		score = 0.0
	}

	return score
}

// Helper methods for ConfidenceScorer

func (cs *ConfidenceScorer) calculateDetails(step RemediationStep, context RemediationContext, factorScores map[string]float64) ConfidenceDetails {
	return ConfidenceDetails{
		EvidenceStrength:  factorScores["evidence_quality"],
		HistoricalSuccess: factorScores["success_rate"],
		EnvironmentMatch:  factorScores["context_match"],
		ComplexityPenalty: 1.0 - factorScores["complexity"], // Invert for penalty
		VerificationScore: factorScores["test_coverage"],
		RiskAssessment:    cs.calculateRiskAssessment(step, context),
	}
}

func (cs *ConfidenceScorer) calculateRiskAssessment(step RemediationStep, context RemediationContext) float64 {
	risk := 0.1 // Base risk

	// Higher difficulty increases risk
	switch step.Difficulty {
	case DifficultyEasy:
		risk += 0.0
	case DifficultyModerate:
		risk += 0.2
	case DifficultyHard:
		risk += 0.4
	case DifficultyExpert:
		risk += 0.6
	}

	// Higher impact categories have higher risk
	switch step.Impact {
	case ImpactHigh:
		risk += 0.2
	case ImpactMedium:
		risk += 0.1
	}

	// Critical priority increases risk
	if step.Priority == "critical" {
		risk += 0.1
	}

	// No rollback increases risk
	if step.Rollback == nil {
		risk += 0.2
	}

	// No verification increases risk
	if step.Verification == nil {
		risk += 0.1
	}

	if risk > 1.0 {
		risk = 1.0
	}

	// Return inverse for assessment score (lower risk = higher score)
	return 1.0 - risk
}

func (cs *ConfidenceScorer) generateExplanation(overall float64, factorScores map[string]float64, details ConfidenceDetails) string {
	var explanation strings.Builder

	// Overall assessment
	if overall >= 0.8 {
		explanation.WriteString("High confidence remediation step. ")
	} else if overall >= 0.6 {
		explanation.WriteString("Moderate confidence remediation step. ")
	} else if overall >= 0.4 {
		explanation.WriteString("Low confidence remediation step. ")
	} else {
		explanation.WriteString("Very low confidence remediation step. ")
	}

	// Key contributing factors
	maxFactor := ""
	maxScore := 0.0
	minFactor := ""
	minScore := 1.0

	for factor, score := range factorScores {
		if score > maxScore {
			maxScore = score
			maxFactor = factor
		}
		if score < minScore {
			minScore = score
			minFactor = factor
		}
	}

	explanation.WriteString(fmt.Sprintf("Strongest factor: %s (%.2f). ", strings.ReplaceAll(maxFactor, "_", " "), maxScore))
	explanation.WriteString(fmt.Sprintf("Weakest factor: %s (%.2f). ", strings.ReplaceAll(minFactor, "_", " "), minScore))

	return explanation.String()
}

func (cs *ConfidenceScorer) generateWarnings(step RemediationStep, overall float64, factorScores map[string]float64) []string {
	var warnings []string

	// Overall confidence warnings
	if overall < 0.5 {
		warnings = append(warnings, "Overall confidence is below 50% - consider alternative approaches")
	}

	// Factor-specific warnings
	if factorScores["success_rate"] < 0.4 {
		warnings = append(warnings, "Low historical success rate - proceed with caution")
	}

	if factorScores["evidence_quality"] < 0.4 {
		warnings = append(warnings, "Limited supporting evidence - verify before execution")
	}

	if factorScores["context_match"] < 0.5 {
		warnings = append(warnings, "Context match is uncertain - verify environment compatibility")
	}

	if factorScores["complexity"] < 0.3 {
		warnings = append(warnings, "High complexity remediation - consider breaking into smaller steps")
	}

	if factorScores["test_coverage"] < 0.4 {
		warnings = append(warnings, "Limited verification capability - test in non-production first")
	}

	// Step-specific warnings
	if step.Rollback == nil {
		warnings = append(warnings, "No rollback procedure defined - create backup plan")
	}

	if step.Verification == nil {
		warnings = append(warnings, "No verification step defined - add success criteria")
	}

	if step.EstimatedTime > 2*time.Hour {
		warnings = append(warnings, "Long execution time estimated - plan for extended maintenance window")
	}

	return warnings
}

func (cs *ConfidenceScorer) getFactorNames() []string {
	var names []string
	for _, factor := range cs.factors {
		names = append(names, factor.Name())
	}
	return names
}
