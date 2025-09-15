package remediation

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// RemediationEngine manages the generation and execution of remediation steps
type RemediationEngine struct {
	providers   []RemediationProvider
	templates   []RemediationTemplate
	prioritizer *RemediationPrioritizer
	categorizer *RemediationCategorizer
	executor    *RemediationExecutor
	correlation *CorrelationEngine
	confidence  *ConfidenceScorer
}

// RemediationPrioritizer handles prioritization of remediation steps
type RemediationPrioritizer struct {
	rules []PrioritizationRule
}

// RemediationCategorizer handles categorization of remediation steps
type RemediationCategorizer struct {
	categories map[RemediationCategory]CategoryInfo
}

// RemediationExecutor handles execution of remediation plans (future framework)
type RemediationExecutor struct {
	dryRun   bool
	timeout  time.Duration
	parallel bool
}

// PrioritizationRule defines how to prioritize remediation steps
type PrioritizationRule struct {
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Condition   string              `json:"condition"`
	Priority    RemediationPriority `json:"priority"`
	Weight      float64             `json:"weight"`
	Factors     []PriorityFactor    `json:"factors"`
}

// PriorityFactor represents factors that influence remediation priority
type PriorityFactor struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

// CategoryInfo provides information about remediation categories
type CategoryInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
	Priority    int      `json:"priority"`
	Icon        string   `json:"icon,omitempty"`
	Color       string   `json:"color,omitempty"`
}

// RemediationOptions configures remediation generation
type RemediationOptions struct {
	IncludeManual      bool                  `json:"include_manual"`
	IncludeAutomated   bool                  `json:"include_automated"`
	MaxSteps           int                   `json:"max_steps"`
	MaxTime            time.Duration         `json:"max_time"`
	AllowedCategories  []RemediationCategory `json:"allowed_categories"`
	ExcludedCategories []RemediationCategory `json:"excluded_categories"`
	MinPriority        RemediationPriority   `json:"min_priority"`
	MaxDifficulty      RemediationDifficulty `json:"max_difficulty"`
	SkillLevel         SkillLevel            `json:"skill_level"`
	DryRun             bool                  `json:"dry_run"`
}

// RemediationGenerationResult contains the results of remediation generation
type RemediationGenerationResult struct {
	Plans        []RemediationPlan    `json:"plans"`
	Steps        []RemediationStep    `json:"steps"`
	Insights     []RemediationInsight `json:"insights"`
	Correlations []Correlation        `json:"correlations"`
	Summary      GenerationSummary    `json:"summary"`
	Warnings     []string             `json:"warnings"`
	Errors       []string             `json:"errors"`
}

// RemediationInsight represents an insight about remediation opportunities
type RemediationInsight struct {
	ID          string            `json:"id"`
	Type        string            `json:"type"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Impact      RemediationImpact `json:"impact"`
	Confidence  float64           `json:"confidence"`
	Evidence    []string          `json:"evidence"`
	Suggestions []RemediationStep `json:"suggestions"`
	Tags        []string          `json:"tags"`
}

// Correlation represents a correlation between analysis results
type Correlation struct {
	ID          string          `json:"id"`
	Type        CorrelationType `json:"type"`
	Description string          `json:"description"`
	Strength    float64         `json:"strength"`   // 0-1, correlation strength
	Confidence  float64         `json:"confidence"` // 0-1, confidence in correlation
	Results     []string        `json:"results"`    // Analysis result IDs
	Evidence    []string        `json:"evidence"`
	Impact      string          `json:"impact"`
}

// CorrelationType defines types of correlations
type CorrelationType string

const (
	CorrelationCausal     CorrelationType = "causal"     // One result causes another
	CorrelationTemporal   CorrelationType = "temporal"   // Results occur together in time
	CorrelationSpatial    CorrelationType = "spatial"    // Results occur in same location
	CorrelationFunctional CorrelationType = "functional" // Results affect same function
	CorrelationResource   CorrelationType = "resource"   // Results affect same resource
)

// GenerationSummary provides a summary of remediation generation
type GenerationSummary struct {
	TotalSteps        int                           `json:"total_steps"`
	StepsByCategory   map[RemediationCategory]int   `json:"steps_by_category"`
	StepsByPriority   map[RemediationPriority]int   `json:"steps_by_priority"`
	StepsByDifficulty map[RemediationDifficulty]int `json:"steps_by_difficulty"`
	EstimatedTime     time.Duration                 `json:"estimated_time"`
	AutomatableSteps  int                           `json:"automatable_steps"`
	ManualSteps       int                           `json:"manual_steps"`
	CriticalSteps     int                           `json:"critical_steps"`
	GenerationTime    time.Duration                 `json:"generation_time"`
	InsightCount      int                           `json:"insight_count"`
	CorrelationCount  int                           `json:"correlation_count"`
}

// NewRemediationEngine creates a new remediation engine
func NewRemediationEngine() *RemediationEngine {
	return &RemediationEngine{
		providers:   getDefaultProviders(),
		templates:   getDefaultTemplates(),
		prioritizer: NewRemediationPrioritizer(),
		categorizer: NewRemediationCategorizer(),
		executor:    NewRemediationExecutor(),
		correlation: NewCorrelationEngine(),
		confidence:  NewConfidenceScorer(),
	}
}

// GenerateRemediation generates remediation steps from analysis results
func (e *RemediationEngine) GenerateRemediation(ctx context.Context, analysisResults []AnalysisResult, options RemediationOptions) (*RemediationGenerationResult, error) {
	startTime := time.Now()

	result := &RemediationGenerationResult{
		Plans:        []RemediationPlan{},
		Steps:        []RemediationStep{},
		Insights:     []RemediationInsight{},
		Correlations: []Correlation{},
		Warnings:     []string{},
		Errors:       []string{},
	}

	// Create remediation context
	remediationCtx := RemediationContext{
		AnalysisResults: analysisResults,
		Environment:     e.inferEnvironmentContext(analysisResults),
		UserPreferences: e.mapOptionsToPreferences(options),
		Constraints:     e.inferConstraints(analysisResults, options),
	}

	// Generate remediation steps using templates
	steps, err := e.generateStepsFromTemplates(ctx, remediationCtx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Template generation failed: %v", err))
	}
	result.Steps = append(result.Steps, steps...)

	// Generate provider-specific remediation steps
	providerSteps, err := e.generateStepsFromProviders(ctx, remediationCtx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("Provider generation failed: %v", err))
	}
	result.Steps = append(result.Steps, providerSteps...)

	// Filter steps based on options
	result.Steps = e.filterSteps(result.Steps, options)

	// Prioritize and categorize steps
	result.Steps = e.prioritizer.PrioritizeSteps(result.Steps, remediationCtx)
	result.Steps = e.categorizer.CategorizeSteps(result.Steps)

	// Generate correlations
	result.Correlations = e.correlation.FindCorrelations(analysisResults, result.Steps)

	// Generate insights
	result.Insights = e.generateInsights(ctx, analysisResults, result.Steps, result.Correlations)

	// Create remediation plans
	result.Plans = e.createRemediationPlans(result.Steps, result.Correlations)

	// Calculate confidence scores
	for i := range result.Steps {
		result.Steps[i] = e.confidence.ScoreStep(result.Steps[i], remediationCtx)
	}

	// Generate summary
	result.Summary = e.generateSummary(result, time.Since(startTime))

	return result, nil
}

// generateStepsFromTemplates generates remediation steps using templates
func (e *RemediationEngine) generateStepsFromTemplates(ctx context.Context, remediationCtx RemediationContext) ([]RemediationStep, error) {
	var steps []RemediationStep

	for _, template := range e.templates {
		for _, analysisResult := range remediationCtx.AnalysisResults {
			if e.matchesTemplate(template, analysisResult) {
				step, err := e.instantiateTemplate(template, analysisResult, remediationCtx)
				if err != nil {
					continue // Skip failed template instantiations
				}
				steps = append(steps, step)
			}
		}
	}

	return steps, nil
}

// generateStepsFromProviders generates remediation steps using providers
func (e *RemediationEngine) generateStepsFromProviders(ctx context.Context, remediationCtx RemediationContext) ([]RemediationStep, error) {
	var steps []RemediationStep

	// For now, we'll generate some built-in remediation steps
	// In a full implementation, this would call external providers
	builtinSteps := e.generateBuiltinSteps(remediationCtx)
	steps = append(steps, builtinSteps...)

	return steps, nil
}

// generateBuiltinSteps generates built-in remediation steps
func (e *RemediationEngine) generateBuiltinSteps(ctx RemediationContext) []RemediationStep {
	var steps []RemediationStep

	for _, result := range ctx.AnalysisResults {
		switch strings.ToLower(result.Category) {
		case "resource":
			steps = append(steps, e.generateResourceSteps(result)...)
		case "storage":
			steps = append(steps, e.generateStorageSteps(result)...)
		case "network":
			steps = append(steps, e.generateNetworkSteps(result)...)
		case "security":
			steps = append(steps, e.generateSecuritySteps(result)...)
		case "configuration":
			steps = append(steps, e.generateConfigurationSteps(result)...)
		}
	}

	return steps
}

// generateResourceSteps generates resource-related remediation steps
func (e *RemediationEngine) generateResourceSteps(result AnalysisResult) []RemediationStep {
	var steps []RemediationStep

	if strings.Contains(strings.ToLower(result.Description), "cpu") ||
		strings.Contains(strings.ToLower(result.Description), "memory") {
		steps = append(steps, RemediationStep{
			ID:            uuid.New().String(),
			Title:         "Scale Resources",
			Description:   fmt.Sprintf("Scale resources to address %s", result.Title),
			Category:      CategoryResource,
			Priority:      e.mapSeverityToPriority(result.Severity),
			Impact:        ImpactMedium,
			Difficulty:    DifficultyEasy,
			EstimatedTime: 10 * time.Minute,
			Command: &CommandStep{
				Command: "kubectl",
				Args:    []string{"scale", "deployment", "your-deployment", "--replicas=3"},
			},
			Verification: &VerificationStep{
				Command: &CommandStep{
					Command: "kubectl",
					Args:    []string{"get", "deployment", "your-deployment"},
				},
			},
			Tags:      []string{"scaling", "resource", "kubernetes"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

	return steps
}

// generateStorageSteps generates storage-related remediation steps
func (e *RemediationEngine) generateStorageSteps(result AnalysisResult) []RemediationStep {
	var steps []RemediationStep

	if strings.Contains(strings.ToLower(result.Description), "disk") ||
		strings.Contains(strings.ToLower(result.Description), "storage") {
		steps = append(steps, RemediationStep{
			ID:            uuid.New().String(),
			Title:         "Expand Storage",
			Description:   fmt.Sprintf("Expand storage to address %s", result.Title),
			Category:      CategoryStorage,
			Priority:      e.mapSeverityToPriority(result.Severity),
			Impact:        ImpactHigh,
			Difficulty:    DifficultyModerate,
			EstimatedTime: 30 * time.Minute,
			Manual: &ManualStep{
				Instructions: []string{
					"Review current storage usage and capacity",
					"Identify storage class and provisioner",
					"Expand persistent volume claim size",
					"Monitor storage expansion progress",
				},
				Checklist: []ChecklistItem{
					{ID: "1", Description: "Backup critical data", Required: true},
					{ID: "2", Description: "Check storage class supports expansion", Required: true},
					{ID: "3", Description: "Update PVC size", Required: true},
					{ID: "4", Description: "Verify expansion completed", Required: true},
				},
			},
			Documentation: []DocumentationLink{
				{
					Title: "Kubernetes Storage Expansion",
					URL:   "https://kubernetes.io/docs/concepts/storage/persistent-volumes/#expanding-persistent-volumes-claims",
					Type:  DocTypeOfficial,
				},
			},
			Tags:      []string{"storage", "expansion", "kubernetes"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

	return steps
}

// generateNetworkSteps generates network-related remediation steps
func (e *RemediationEngine) generateNetworkSteps(result AnalysisResult) []RemediationStep {
	var steps []RemediationStep

	if strings.Contains(strings.ToLower(result.Description), "connectivity") ||
		strings.Contains(strings.ToLower(result.Description), "network") {
		steps = append(steps, RemediationStep{
			ID:            uuid.New().String(),
			Title:         "Fix Network Connectivity",
			Description:   fmt.Sprintf("Resolve network connectivity issue: %s", result.Title),
			Category:      CategoryNetwork,
			Priority:      PriorityHigh,
			Impact:        ImpactHigh,
			Difficulty:    DifficultyModerate,
			EstimatedTime: 20 * time.Minute,
			Script: &ScriptStep{
				Language: LanguageBash,
				Content: `#!/bin/bash
# Network connectivity troubleshooting script
echo "Checking network connectivity..."
kubectl get services
kubectl get endpoints
kubectl describe service $SERVICE_NAME
kubectl get networkpolicies
`,
				Environment: map[string]string{
					"SERVICE_NAME": "your-service",
				},
			},
			Tags:      []string{"network", "connectivity", "troubleshooting"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

	return steps
}

// generateSecuritySteps generates security-related remediation steps
func (e *RemediationEngine) generateSecuritySteps(result AnalysisResult) []RemediationStep {
	var steps []RemediationStep

	if strings.Contains(strings.ToLower(result.Description), "rbac") ||
		strings.Contains(strings.ToLower(result.Description), "security") {
		steps = append(steps, RemediationStep{
			ID:            uuid.New().String(),
			Title:         "Apply Security Policy",
			Description:   fmt.Sprintf("Apply security policy to address %s", result.Title),
			Category:      CategorySecurity,
			Priority:      PriorityCritical,
			Impact:        ImpactHigh,
			Difficulty:    DifficultyHard,
			EstimatedTime: 45 * time.Minute,
			Manual: &ManualStep{
				Instructions: []string{
					"Review current RBAC configuration",
					"Identify required permissions for the application",
					"Create appropriate roles and role bindings",
					"Apply principle of least privilege",
					"Test application functionality with new permissions",
				},
			},
			Documentation: []DocumentationLink{
				{
					Title: "Kubernetes RBAC Authorization",
					URL:   "https://kubernetes.io/docs/reference/access-authn-authz/rbac/",
					Type:  DocTypeOfficial,
				},
			},
			Tags:      []string{"security", "rbac", "authorization"},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		})
	}

	return steps
}

// generateConfigurationSteps generates configuration-related remediation steps
func (e *RemediationEngine) generateConfigurationSteps(result AnalysisResult) []RemediationStep {
	var steps []RemediationStep

	steps = append(steps, RemediationStep{
		ID:            uuid.New().String(),
		Title:         "Update Configuration",
		Description:   fmt.Sprintf("Update configuration to resolve %s", result.Title),
		Category:      CategoryConfiguration,
		Priority:      e.mapSeverityToPriority(result.Severity),
		Impact:        ImpactMedium,
		Difficulty:    DifficultyEasy,
		EstimatedTime: 15 * time.Minute,
		Manual: &ManualStep{
			Instructions: []string{
				"Review current configuration settings",
				"Identify the configuration parameter that needs to be changed",
				"Update the configuration file or ConfigMap",
				"Restart the affected component if necessary",
				"Verify the configuration change has taken effect",
			},
		},
		Tags:      []string{"configuration", "update"},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})

	return steps
}

// Helper methods

func (e *RemediationEngine) filterSteps(steps []RemediationStep, options RemediationOptions) []RemediationStep {
	var filtered []RemediationStep

	for _, step := range steps {
		// Filter by allowed/excluded categories
		if len(options.AllowedCategories) > 0 && !e.containsCategory(options.AllowedCategories, step.Category) {
			continue
		}
		if e.containsCategory(options.ExcludedCategories, step.Category) {
			continue
		}

		// Filter by minimum priority
		if !e.isPriorityHigherOrEqual(step.Priority, options.MinPriority) {
			continue
		}

		// Filter by maximum difficulty
		if !e.isDifficultyLowerOrEqual(step.Difficulty, options.MaxDifficulty) {
			continue
		}

		// Filter by step type (manual/automated)
		if !options.IncludeManual && step.Manual != nil {
			continue
		}
		if !options.IncludeAutomated && (step.Command != nil || step.Script != nil) {
			continue
		}

		filtered = append(filtered, step)
	}

	// Limit number of steps
	if options.MaxSteps > 0 && len(filtered) > options.MaxSteps {
		filtered = filtered[:options.MaxSteps]
	}

	return filtered
}

func (e *RemediationEngine) containsCategory(categories []RemediationCategory, category RemediationCategory) bool {
	for _, c := range categories {
		if c == category {
			return true
		}
	}
	return false
}

func (e *RemediationEngine) isPriorityHigherOrEqual(priority, minPriority RemediationPriority) bool {
	priorities := map[RemediationPriority]int{
		PriorityCritical: 4,
		PriorityHigh:     3,
		PriorityMedium:   2,
		PriorityLow:      1,
		PriorityInfo:     0,
	}
	return priorities[priority] >= priorities[minPriority]
}

func (e *RemediationEngine) isDifficultyLowerOrEqual(difficulty, maxDifficulty RemediationDifficulty) bool {
	difficulties := map[RemediationDifficulty]int{
		DifficultyEasy:     1,
		DifficultyModerate: 2,
		DifficultyHard:     3,
		DifficultyExpert:   4,
	}
	return difficulties[difficulty] <= difficulties[maxDifficulty]
}

func (e *RemediationEngine) mapSeverityToPriority(severity string) RemediationPriority {
	switch strings.ToLower(severity) {
	case "critical":
		return PriorityCritical
	case "high":
		return PriorityHigh
	case "medium":
		return PriorityMedium
	case "low":
		return PriorityLow
	default:
		return PriorityInfo
	}
}

func (e *RemediationEngine) inferEnvironmentContext(results []AnalysisResult) EnvironmentContext {
	// Simple environment inference based on analysis results
	env := EnvironmentContext{
		Platform:     "kubernetes",
		Capabilities: []string{"kubectl", "helm"},
	}

	// Infer more details from analysis results
	for _, result := range results {
		if strings.Contains(strings.ToLower(result.Description), "aws") {
			env.CloudProvider = "aws"
		} else if strings.Contains(strings.ToLower(result.Description), "azure") {
			env.CloudProvider = "azure"
		} else if strings.Contains(strings.ToLower(result.Description), "gcp") {
			env.CloudProvider = "gcp"
		}
	}

	return env
}

func (e *RemediationEngine) mapOptionsToPreferences(options RemediationOptions) UserPreferences {
	return UserPreferences{
		MaxRisk:           ImpactMedium,
		PreferredMethods:  options.AllowedCategories,
		AutoExecute:       !options.DryRun,
		NotificationLevel: options.MinPriority,
		SkillLevel:        options.SkillLevel,
	}
}

func (e *RemediationEngine) inferConstraints(results []AnalysisResult, options RemediationOptions) []Constraint {
	var constraints []Constraint

	if options.MaxTime > 0 {
		constraints = append(constraints, Constraint{
			Type:        ConstraintTime,
			Description: "Maximum execution time constraint",
			Value:       options.MaxTime,
			Severity:    "high",
		})
	}

	return constraints
}

func (e *RemediationEngine) matchesTemplate(template RemediationTemplate, result AnalysisResult) bool {
	// Simple pattern matching - in practice this would be more sophisticated
	return strings.Contains(strings.ToLower(result.Category), strings.ToLower(string(template.Category)))
}

func (e *RemediationEngine) instantiateTemplate(template RemediationTemplate, result AnalysisResult, ctx RemediationContext) (RemediationStep, error) {
	// Create a step from the template
	step := template.Template
	step.ID = uuid.New().String()
	step.Title = fmt.Sprintf(step.Title, result.Title)
	step.Description = fmt.Sprintf(step.Description, result.Description)
	step.CreatedAt = time.Now()
	step.UpdatedAt = time.Now()

	return step, nil
}

func (e *RemediationEngine) generateInsights(ctx context.Context, results []AnalysisResult, steps []RemediationStep, correlations []Correlation) []RemediationInsight {
	var insights []RemediationInsight

	// Generate pattern-based insights
	insights = append(insights, e.generatePatternInsights(results, steps)...)

	// Generate correlation-based insights
	insights = append(insights, e.generateCorrelationInsights(correlations, steps)...)

	// Generate optimization insights
	insights = append(insights, e.generateOptimizationInsights(results, steps)...)

	return insights
}

func (e *RemediationEngine) generatePatternInsights(results []AnalysisResult, steps []RemediationStep) []RemediationInsight {
	var insights []RemediationInsight

	// Group results by category to find patterns
	categoryGroups := make(map[string][]AnalysisResult)
	for _, result := range results {
		categoryGroups[result.Category] = append(categoryGroups[result.Category], result)
	}

	for category, categoryResults := range categoryGroups {
		if len(categoryResults) > 2 { // Pattern detected
			insights = append(insights, RemediationInsight{
				ID:          uuid.New().String(),
				Type:        "pattern",
				Title:       fmt.Sprintf("Multiple %s Issues Detected", category),
				Description: fmt.Sprintf("Found %d issues in the %s category, suggesting a systemic problem", len(categoryResults), category),
				Impact:      ImpactHigh,
				Confidence:  0.8,
				Evidence:    []string{fmt.Sprintf("%d issues in %s category", len(categoryResults), category)},
				Suggestions: e.getRelevantSteps(steps, category),
				Tags:        []string{"pattern", category},
			})
		}
	}

	return insights
}

func (e *RemediationEngine) generateCorrelationInsights(correlations []Correlation, steps []RemediationStep) []RemediationInsight {
	var insights []RemediationInsight

	for _, correlation := range correlations {
		if correlation.Strength > 0.7 { // Strong correlation
			insights = append(insights, RemediationInsight{
				ID:          uuid.New().String(),
				Type:        "correlation",
				Title:       "Related Issues Identified",
				Description: correlation.Description,
				Impact:      ImpactMedium,
				Confidence:  correlation.Confidence,
				Evidence:    correlation.Evidence,
				Suggestions: []RemediationStep{}, // Would be populated based on correlation
				Tags:        []string{"correlation", string(correlation.Type)},
			})
		}
	}

	return insights
}

func (e *RemediationEngine) generateOptimizationInsights(results []AnalysisResult, steps []RemediationStep) []RemediationInsight {
	var insights []RemediationInsight

	// Look for optimization opportunities
	resourceIssues := 0
	for _, result := range results {
		if strings.Contains(strings.ToLower(result.Category), "resource") {
			resourceIssues++
		}
	}

	if resourceIssues > 1 {
		insights = append(insights, RemediationInsight{
			ID:          uuid.New().String(),
			Type:        "optimization",
			Title:       "Resource Optimization Opportunity",
			Description: "Multiple resource-related issues suggest opportunities for resource optimization",
			Impact:      ImpactMedium,
			Confidence:  0.7,
			Evidence:    []string{fmt.Sprintf("%d resource-related issues", resourceIssues)},
			Suggestions: e.getRelevantSteps(steps, "resource"),
			Tags:        []string{"optimization", "resource"},
		})
	}

	return insights
}

func (e *RemediationEngine) getRelevantSteps(steps []RemediationStep, category string) []RemediationStep {
	var relevant []RemediationStep
	for _, step := range steps {
		if strings.Contains(strings.ToLower(string(step.Category)), strings.ToLower(category)) {
			relevant = append(relevant, step)
		}
	}
	return relevant
}

func (e *RemediationEngine) createRemediationPlans(steps []RemediationStep, correlations []Correlation) []RemediationPlan {
	// Group steps by category and priority to create logical plans
	planMap := make(map[string][]RemediationStep)

	for _, step := range steps {
		key := string(step.Category)
		planMap[key] = append(planMap[key], step)
	}

	var plans []RemediationPlan
	for category, categorySteps := range planMap {
		if len(categorySteps) == 0 {
			continue
		}

		// Sort steps by priority within category
		sort.Slice(categorySteps, func(i, j int) bool {
			return e.isPriorityHigherOrEqual(categorySteps[i].Priority, categorySteps[j].Priority)
		})

		totalTime := time.Duration(0)
		for _, step := range categorySteps {
			totalTime += step.EstimatedTime
		}

		plan := RemediationPlan{
			ID:          uuid.New().String(),
			Title:       fmt.Sprintf("%s Remediation Plan", strings.Title(category)),
			Description: fmt.Sprintf("Remediation plan for %s-related issues", category),
			Steps:       categorySteps,
			TotalTime:   totalTime,
			Priority:    categorySteps[0].Priority, // Highest priority step
			Impact:      e.calculatePlanImpact(categorySteps),
			Categories:  []RemediationCategory{RemediationCategory(category)},
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		plans = append(plans, plan)
	}

	return plans
}

func (e *RemediationEngine) calculatePlanImpact(steps []RemediationStep) RemediationImpact {
	// Calculate overall impact based on individual step impacts
	highCount := 0
	mediumCount := 0
	lowCount := 0

	for _, step := range steps {
		switch step.Impact {
		case ImpactHigh:
			highCount++
		case ImpactMedium:
			mediumCount++
		case ImpactLow:
			lowCount++
		}
	}

	if highCount > 0 {
		return ImpactHigh
	}
	if mediumCount > 0 {
		return ImpactMedium
	}
	if lowCount > 0 {
		return ImpactLow
	}
	return ImpactUnknown
}

func (e *RemediationEngine) generateSummary(result *RemediationGenerationResult, generationTime time.Duration) GenerationSummary {
	summary := GenerationSummary{
		TotalSteps:        len(result.Steps),
		StepsByCategory:   make(map[RemediationCategory]int),
		StepsByPriority:   make(map[RemediationPriority]int),
		StepsByDifficulty: make(map[RemediationDifficulty]int),
		EstimatedTime:     0,
		AutomatableSteps:  0,
		ManualSteps:       0,
		CriticalSteps:     0,
		GenerationTime:    generationTime,
		InsightCount:      len(result.Insights),
		CorrelationCount:  len(result.Correlations),
	}

	for _, step := range result.Steps {
		summary.StepsByCategory[step.Category]++
		summary.StepsByPriority[step.Priority]++
		summary.StepsByDifficulty[step.Difficulty]++
		summary.EstimatedTime += step.EstimatedTime

		if step.Command != nil || step.Script != nil {
			summary.AutomatableSteps++
		}
		if step.Manual != nil {
			summary.ManualSteps++
		}
		if step.Priority == PriorityCritical {
			summary.CriticalSteps++
		}
	}

	return summary
}

// Factory functions for sub-components

func NewRemediationPrioritizer() *RemediationPrioritizer {
	return &RemediationPrioritizer{
		rules: getDefaultPrioritizationRules(),
	}
}

// PrioritizeSteps prioritizes remediation steps based on rules
func (p *RemediationPrioritizer) PrioritizeSteps(steps []RemediationStep, ctx RemediationContext) []RemediationStep {
	// Simple implementation - just return steps as-is for now
	// TODO: Implement actual prioritization logic
	return steps
}

func NewRemediationCategorizer() *RemediationCategorizer {
	return &RemediationCategorizer{
		categories: getDefaultCategoryInfo(),
	}
}

// CategorizeSteps categorizes remediation steps
func (c *RemediationCategorizer) CategorizeSteps(steps []RemediationStep) []RemediationStep {
	// Simple implementation - just return steps as-is for now
	// TODO: Implement actual categorization logic
	return steps
}

func NewRemediationExecutor() *RemediationExecutor {
	return &RemediationExecutor{
		dryRun:   true,
		timeout:  time.Hour,
		parallel: false,
	}
}

// Default data functions (placeholders for comprehensive defaults)

func getDefaultProviders() []RemediationProvider {
	return []RemediationProvider{
		{
			ID:          "builtin",
			Name:        "Built-in Remediation Provider",
			Description: "Default built-in remediation suggestions",
			Version:     "1.0.0",
			Categories:  []RemediationCategory{CategoryResource, CategoryStorage, CategoryNetwork, CategorySecurity, CategoryConfiguration},
			Enabled:     true,
		},
	}
}

func getDefaultTemplates() []RemediationTemplate {
	return []RemediationTemplate{
		{
			ID:          "resource-scaling",
			Name:        "Resource Scaling Template",
			Description: "Template for scaling resource-related issues",
			Category:    CategoryResource,
			Pattern:     "resource.*insufficient",
			Template: RemediationStep{
				Title:       "Scale %s Resources",
				Description: "Scale resources to address %s",
				Category:    CategoryResource,
				Priority:    PriorityHigh,
				Impact:      ImpactMedium,
				Difficulty:  DifficultyEasy,
			},
		},
	}
}

func getDefaultPrioritizationRules() []PrioritizationRule {
	return []PrioritizationRule{
		{
			Name:        "critical_security",
			Description: "Prioritize critical security issues",
			Condition:   "category == 'security' && severity == 'critical'",
			Priority:    PriorityCritical,
			Weight:      1.0,
		},
		{
			Name:        "high_impact_resource",
			Description: "Prioritize high-impact resource issues",
			Condition:   "category == 'resource' && impact == 'high'",
			Priority:    PriorityHigh,
			Weight:      0.8,
		},
	}
}

func getDefaultCategoryInfo() map[RemediationCategory]CategoryInfo {
	return map[RemediationCategory]CategoryInfo{
		CategoryConfiguration: {
			Name:        "Configuration",
			Description: "Configuration-related remediation steps",
			Keywords:    []string{"config", "configuration", "setting", "parameter"},
			Priority:    1,
			Icon:        "⚙️",
			Color:       "#4CAF50",
		},
		CategoryResource: {
			Name:        "Resource",
			Description: "Resource-related remediation steps",
			Keywords:    []string{"resource", "cpu", "memory", "disk", "scaling"},
			Priority:    2,
			Icon:        "📊",
			Color:       "#FF9800",
		},
		CategorySecurity: {
			Name:        "Security",
			Description: "Security-related remediation steps",
			Keywords:    []string{"security", "rbac", "permission", "auth", "tls"},
			Priority:    3,
			Icon:        "🔒",
			Color:       "#F44336",
		},
		CategoryNetwork: {
			Name:        "Network",
			Description: "Network-related remediation steps",
			Keywords:    []string{"network", "connectivity", "dns", "port", "service"},
			Priority:    4,
			Icon:        "🌐",
			Color:       "#2196F3",
		},
		CategoryStorage: {
			Name:        "Storage",
			Description: "Storage-related remediation steps",
			Keywords:    []string{"storage", "disk", "volume", "pvc", "capacity"},
			Priority:    5,
			Icon:        "💾",
			Color:       "#9C27B0",
		},
	}
}
