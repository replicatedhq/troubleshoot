package artifacts

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"gopkg.in/yaml.v2"
)

// JSONValidator validates JSON artifact content
type JSONValidator struct{}

func (v *JSONValidator) Validate(ctx context.Context, data []byte) error {
	// Check if it's valid JSON
	var result analyzer.AnalysisResult
	if err := json.Unmarshal(data, &result); err != nil {
		return errors.Wrap(err, "invalid JSON format")
	}

	// Validate required fields
	if err := v.validateAnalysisResult(&result); err != nil {
		return errors.Wrap(err, "analysis result validation failed")
	}

	return nil
}

func (v *JSONValidator) validateAnalysisResult(result *analyzer.AnalysisResult) error {
	// Check required fields
	if result.Results == nil {
		return errors.New("results field is required")
	}

	if result.Metadata.Timestamp.IsZero() {
		return errors.New("metadata timestamp is required")
	}

	if result.Metadata.EngineVersion == "" {
		return errors.New("metadata engine version is required")
	}

	// Validate individual results
	for i, r := range result.Results {
		if err := v.validateAnalyzerResult(r, i); err != nil {
			return err
		}
	}

	// Validate remediation steps
	for i, step := range result.Remediation {
		if err := v.validateRemediationStep(&step, i); err != nil {
			return err
		}
	}

	// Validate summary consistency
	if err := v.validateSummary(&result.Summary, len(result.Results)); err != nil {
		return err
	}

	return nil
}

func (v *JSONValidator) validateAnalyzerResult(result *analyzer.AnalyzerResult, index int) error {
	if result.Title == "" {
		return errors.Errorf("result at index %d: title is required", index)
	}

	// Check that only one status is true
	statusCount := 0
	if result.IsPass {
		statusCount++
	}
	if result.IsWarn {
		statusCount++
	}
	if result.IsFail {
		statusCount++
	}

	if statusCount != 1 {
		return errors.Errorf("result at index %d: exactly one status (pass/warn/fail) must be true", index)
	}

	// Validate confidence range if specified
	if result.Confidence < 0 || result.Confidence > 1 {
		return errors.Errorf("result at index %d: confidence must be between 0 and 1", index)
	}

	return nil
}

func (v *JSONValidator) validateRemediationStep(step *analyzer.RemediationStep, index int) error {
	if step.Description == "" {
		return errors.Errorf("remediation step at index %d: description is required", index)
	}

	if step.Priority < 1 || step.Priority > 10 {
		return errors.Errorf("remediation step at index %d: priority must be between 1 and 10", index)
	}

	return nil
}

func (v *JSONValidator) validateSummary(summary *analyzer.AnalysisSummary, totalResults int) error {
	// Check that counts add up
	expectedTotal := summary.PassCount + summary.WarnCount + summary.FailCount
	if expectedTotal != totalResults {
		return errors.Errorf("summary counts (%d) don't match total results (%d)",
			expectedTotal, totalResults)
	}

	if summary.TotalAnalyzers != totalResults {
		return errors.Errorf("summary total analyzers (%d) doesn't match actual results (%d)",
			summary.TotalAnalyzers, totalResults)
	}

	return nil
}

func (v *JSONValidator) Schema() string {
	return "analysis-result-v1.0.json"
}

// YAMLValidator validates YAML artifact content
type YAMLValidator struct{}

func (v *YAMLValidator) Validate(ctx context.Context, data []byte) error {
	// Check if it's valid YAML
	var result analyzer.AnalysisResult
	if err := yaml.Unmarshal(data, &result); err != nil {
		return errors.Wrap(err, "invalid YAML format")
	}

	// Use the same validation logic as JSON
	jsonValidator := &JSONValidator{}
	return jsonValidator.validateAnalysisResult(&result)
}

func (v *YAMLValidator) Schema() string {
	return "analysis-result-v1.0.yaml"
}

// SummaryValidator validates summary artifacts
type SummaryValidator struct{}

func (v *SummaryValidator) Validate(ctx context.Context, data []byte) error {
	var summary struct {
		Overview        analyzer.AnalysisSummary   `json:"overview"`
		TopIssues       []*analyzer.AnalyzerResult `json:"topIssues"`
		Categories      map[string]int             `json:"categories"`
		Agents          []analyzer.AgentMetadata   `json:"agents"`
		Recommendations []string                   `json:"recommendations"`
	}

	if err := json.Unmarshal(data, &summary); err != nil {
		return errors.Wrap(err, "invalid summary JSON format")
	}

	// Validate overview
	if summary.Overview.TotalAnalyzers < 0 {
		return errors.New("total analyzers cannot be negative")
	}

	// Validate top issues
	for i, issue := range summary.TopIssues {
		if !issue.IsFail {
			return errors.Errorf("top issue at index %d must be a failed result", i)
		}
	}

	// Validate categories
	for category, count := range summary.Categories {
		if category == "" {
			return errors.New("category name cannot be empty")
		}
		if count < 0 {
			return errors.Errorf("category %s count cannot be negative", category)
		}
	}

	return nil
}

func (v *SummaryValidator) Schema() string {
	return "summary-v1.0.json"
}

// InsightsValidator validates insights artifacts
type InsightsValidator struct{}

func (v *InsightsValidator) Validate(ctx context.Context, data []byte) error {
	var insights struct {
		KeyFindings     []string               `json:"keyFindings"`
		Patterns        []Pattern              `json:"patterns"`
		Correlations    []analyzer.Correlation `json:"correlations"`
		Trends          []Trend                `json:"trends"`
		Recommendations []RemediationInsight   `json:"recommendations"`
	}

	if err := json.Unmarshal(data, &insights); err != nil {
		return errors.Wrap(err, "invalid insights JSON format")
	}

	// Validate patterns
	for i, pattern := range insights.Patterns {
		if err := v.validatePattern(&pattern, i); err != nil {
			return err
		}
	}

	// Validate correlations
	for i, correlation := range insights.Correlations {
		if err := v.validateCorrelation(&correlation, i); err != nil {
			return err
		}
	}

	// Validate trends
	for i, trend := range insights.Trends {
		if err := v.validateTrend(&trend, i); err != nil {
			return err
		}
	}

	return nil
}

func (v *InsightsValidator) validatePattern(pattern *Pattern, index int) error {
	if pattern.Type == "" {
		return errors.Errorf("pattern at index %d: type is required", index)
	}

	if pattern.Count < 0 {
		return errors.Errorf("pattern at index %d: count cannot be negative", index)
	}

	if pattern.Confidence < 0 || pattern.Confidence > 1 {
		return errors.Errorf("pattern at index %d: confidence must be between 0 and 1", index)
	}

	return nil
}

func (v *InsightsValidator) validateCorrelation(correlation *analyzer.Correlation, index int) error {
	if correlation.Type == "" {
		return errors.Errorf("correlation at index %d: type is required", index)
	}

	if len(correlation.ResultIDs) < 2 {
		return errors.Errorf("correlation at index %d: must have at least 2 result IDs", index)
	}

	if correlation.Confidence < 0 || correlation.Confidence > 1 {
		return errors.Errorf("correlation at index %d: confidence must be between 0 and 1", index)
	}

	return nil
}

func (v *InsightsValidator) validateTrend(trend *Trend, index int) error {
	if trend.Category == "" {
		return errors.Errorf("trend at index %d: category is required", index)
	}

	validDirections := []string{"improving", "degrading", "stable"}
	validDirection := false
	for _, valid := range validDirections {
		if trend.Direction == valid {
			validDirection = true
			break
		}
	}

	if !validDirection {
		return errors.Errorf("trend at index %d: direction must be one of %v", index, validDirections)
	}

	if trend.Confidence < 0 || trend.Confidence > 1 {
		return errors.Errorf("trend at index %d: confidence must be between 0 and 1", index)
	}

	return nil
}

func (v *InsightsValidator) Schema() string {
	return "insights-v1.0.json"
}

// RemediationValidator validates remediation guide artifacts
type RemediationValidator struct{}

func (v *RemediationValidator) Validate(ctx context.Context, data []byte) error {
	var guide struct {
		Summary         string                                `json:"summary"`
		PriorityActions []analyzer.RemediationStep            `json:"priorityActions"`
		Categories      map[string][]analyzer.RemediationStep `json:"categories"`
		Prerequisites   []string                              `json:"prerequisites"`
		Automation      AutomationGuide                       `json:"automation"`
	}

	if err := json.Unmarshal(data, &guide); err != nil {
		return errors.Wrap(err, "invalid remediation guide JSON format")
	}

	// Validate priority actions
	for i, action := range guide.PriorityActions {
		if action.Description == "" {
			return errors.Errorf("priority action at index %d: description is required", i)
		}
		if action.Priority < 1 || action.Priority > 10 {
			return errors.Errorf("priority action at index %d: priority must be between 1 and 10", i)
		}
	}

	// Validate categories
	for category, steps := range guide.Categories {
		if category == "" {
			return errors.New("category name cannot be empty")
		}
		for i, step := range steps {
			if step.Description == "" {
				return errors.Errorf("step at index %d in category %s: description is required", i, category)
			}
		}
	}

	// Validate automation guide
	if guide.Automation.AutomatableSteps < 0 {
		return errors.New("automatable steps count cannot be negative")
	}
	if guide.Automation.ManualSteps < 0 {
		return errors.New("manual steps count cannot be negative")
	}

	for i, script := range guide.Automation.Scripts {
		if script.Name == "" {
			return errors.Errorf("script at index %d: name is required", i)
		}
		if script.Content == "" {
			return errors.Errorf("script at index %d: content is required", i)
		}
	}

	return nil
}

func (v *RemediationValidator) Schema() string {
	return "remediation-guide-v1.0.json"
}

// CorrelationValidator validates correlation artifacts
type CorrelationValidator struct{}

func (v *CorrelationValidator) Validate(ctx context.Context, data []byte) error {
	var correlations map[string]interface{}

	if err := json.Unmarshal(data, &correlations); err != nil {
		return errors.Wrap(err, "invalid correlation JSON format")
	}

	// Validate that it's a proper map structure
	if len(correlations) == 0 {
		return errors.New("correlations map cannot be empty")
	}

	// Basic structure validation - in a real implementation,
	// this would have more specific validation based on correlation types
	for key, value := range correlations {
		if key == "" {
			return errors.New("correlation key cannot be empty")
		}
		if value == nil {
			return errors.Errorf("correlation value for key %s cannot be nil", key)
		}
	}

	return nil
}

func (v *CorrelationValidator) Schema() string {
	return "correlations-v1.0.json"
}

// ValidatorRegistry manages all validators
type ValidatorRegistry struct {
	validators map[string]ArtifactValidator
}

// NewValidatorRegistry creates a new validator registry
func NewValidatorRegistry() *ValidatorRegistry {
	registry := &ValidatorRegistry{
		validators: make(map[string]ArtifactValidator),
	}

	// Register default validators
	registry.RegisterValidator("json", &JSONValidator{})
	registry.RegisterValidator("yaml", &YAMLValidator{})
	registry.RegisterValidator("summary", &SummaryValidator{})
	registry.RegisterValidator("insights", &InsightsValidator{})
	registry.RegisterValidator("remediation", &RemediationValidator{})
	registry.RegisterValidator("correlations", &CorrelationValidator{})

	return registry
}

// RegisterValidator registers a new validator
func (r *ValidatorRegistry) RegisterValidator(name string, validator ArtifactValidator) {
	r.validators[name] = validator
}

// GetValidator gets a validator by name
func (r *ValidatorRegistry) GetValidator(name string) (ArtifactValidator, bool) {
	validator, exists := r.validators[name]
	return validator, exists
}

// ValidateArtifact validates an artifact using the appropriate validator
func (r *ValidatorRegistry) ValidateArtifact(ctx context.Context, artifact *Artifact) error {
	validator, exists := r.GetValidator(artifact.Format)
	if !exists {
		return errors.Errorf("no validator found for format: %s", artifact.Format)
	}

	return validator.Validate(ctx, artifact.Content)
}

// ValidateAllArtifacts validates a collection of artifacts
func (r *ValidatorRegistry) ValidateAllArtifacts(ctx context.Context, artifacts []*Artifact) []error {
	var errors []error

	for i, artifact := range artifacts {
		if err := r.ValidateArtifact(ctx, artifact); err != nil {
			errors = append(errors, fmt.Errorf("artifact %d (%s): %v", i, artifact.Name, err))
		}
	}

	return errors
}
