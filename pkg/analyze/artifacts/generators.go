package artifacts

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

// SummaryGenerator generates summary artifacts
type SummaryGenerator struct{}

func (g *SummaryGenerator) Generate(ctx context.Context, result *analyzer.AnalysisResult) (*Artifact, error) {
	summary := struct {
		Overview        analyzer.AnalysisSummary   `json:"overview"`
		TopIssues       []*analyzer.AnalyzerResult `json:"topIssues"`
		Categories      map[string]int             `json:"categories"`
		Agents          []analyzer.AgentMetadata   `json:"agents"`
		Recommendations []string                   `json:"recommendations"`
		GeneratedAt     time.Time                  `json:"generatedAt"`
	}{
		Overview:        result.Summary,
		Categories:      g.categorizeResults(result.Results),
		Agents:          result.Metadata.Agents,
		TopIssues:       g.getTopIssues(result.Results, 10),
		Recommendations: g.generateRecommendations(result),
		GeneratedAt:     time.Now(),
	}

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return nil, err
	}

	artifact := &Artifact{
		Name:        "summary.json",
		Type:        "summary",
		Format:      "json",
		ContentType: "application/json",
		Size:        int64(len(data)),
		Content:     data,
		Metadata: ArtifactMetadata{
			CreatedAt: time.Now(),
			Generator: "SummaryGenerator",
			Version:   "1.0.0",
			Tags:      []string{"summary", "overview"},
		},
	}

	return artifact, nil
}

func (g *SummaryGenerator) Name() string {
	return "Summary Generator"
}

func (g *SummaryGenerator) Description() string {
	return "Generates high-level summary artifacts from analysis results"
}

func (g *SummaryGenerator) categorizeResults(results []*analyzer.AnalyzerResult) map[string]int {
	categories := make(map[string]int)
	for _, result := range results {
		if result.Category != "" {
			categories[result.Category]++
		}
	}
	return categories
}

func (g *SummaryGenerator) getTopIssues(results []*analyzer.AnalyzerResult, limit int) []*analyzer.AnalyzerResult {
	var failedResults []*analyzer.AnalyzerResult
	for _, result := range results {
		if result.IsFail {
			failedResults = append(failedResults, result)
		}
	}

	sort.Slice(failedResults, func(i, j int) bool {
		return failedResults[i].Confidence > failedResults[j].Confidence
	})

	if len(failedResults) > limit {
		return failedResults[:limit]
	}
	return failedResults
}

func (g *SummaryGenerator) generateRecommendations(result *analyzer.AnalysisResult) []string {
	var recommendations []string

	if result.Summary.FailCount > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Address %d failed checks to improve system health", result.Summary.FailCount))
	}

	if result.Summary.WarnCount > result.Summary.PassCount {
		recommendations = append(recommendations,
			"Review warning conditions to prevent potential issues")
	}

	categories := g.categorizeResults(result.Results)
	for category, count := range categories {
		if count >= 5 {
			recommendations = append(recommendations,
				fmt.Sprintf("Focus attention on %s category (%d issues)", category, count))
		}
	}

	return recommendations
}

// InsightsGenerator generates insights and correlation artifacts
type InsightsGenerator struct{}

func (g *InsightsGenerator) Generate(ctx context.Context, result *analyzer.AnalysisResult) (*Artifact, error) {
	insights := struct {
		KeyFindings     []string               `json:"keyFindings"`
		Patterns        []Pattern              `json:"patterns"`
		Correlations    []analyzer.Correlation `json:"correlations"`
		Trends          []Trend                `json:"trends"`
		Recommendations []RemediationInsight   `json:"recommendations"`
		GeneratedAt     time.Time              `json:"generatedAt"`
	}{
		KeyFindings:     g.extractKeyFindings(result.Results),
		Patterns:        g.identifyPatterns(result.Results),
		Correlations:    result.Metadata.Correlations,
		Trends:          g.analyzeTrends(result.Results),
		Recommendations: g.generateRemediationInsights(result.Remediation),
		GeneratedAt:     time.Now(),
	}

	data, err := json.MarshalIndent(insights, "", "  ")
	if err != nil {
		return nil, err
	}

	artifact := &Artifact{
		Name:        "insights.json",
		Type:        "insights",
		Format:      "json",
		ContentType: "application/json",
		Size:        int64(len(data)),
		Content:     data,
		Metadata: ArtifactMetadata{
			CreatedAt: time.Now(),
			Generator: "InsightsGenerator",
			Version:   "1.0.0",
			Tags:      []string{"insights", "patterns", "correlations"},
		},
	}

	return artifact, nil
}

func (g *InsightsGenerator) Name() string {
	return "Insights Generator"
}

func (g *InsightsGenerator) Description() string {
	return "Generates insights, patterns, and correlation artifacts"
}

func (g *InsightsGenerator) extractKeyFindings(results []*analyzer.AnalyzerResult) []string {
	var findings []string

	for _, result := range results {
		if result.IsFail && result.Confidence > 0.8 {
			findings = append(findings, result.Message)
		}
	}

	if len(findings) > 10 {
		findings = findings[:10]
	}

	return findings
}

func (g *InsightsGenerator) identifyPatterns(results []*analyzer.AnalyzerResult) []Pattern {
	var patterns []Pattern

	// Pattern: Multiple failures in same category
	categoryFailures := make(map[string]int)
	for _, result := range results {
		if result.IsFail && result.Category != "" {
			categoryFailures[result.Category]++
		}
	}

	for category, count := range categoryFailures {
		if count >= 3 {
			patterns = append(patterns, Pattern{
				Type:        "category-failure-cluster",
				Description: fmt.Sprintf("Multiple failures in %s category", category),
				Count:       count,
				Confidence:  0.8,
			})
		}
	}

	// Pattern: Agent-specific issues
	agentFailures := make(map[string]int)
	for _, result := range results {
		if result.IsFail && result.AgentName != "" {
			agentFailures[result.AgentName]++
		}
	}

	for agent, count := range agentFailures {
		if count >= 2 {
			patterns = append(patterns, Pattern{
				Type:        "agent-failure-pattern",
				Description: fmt.Sprintf("Multiple failures detected by %s agent", agent),
				Count:       count,
				Confidence:  0.7,
			})
		}
	}

	return patterns
}

func (g *InsightsGenerator) analyzeTrends(results []*analyzer.AnalyzerResult) []Trend {
	// Placeholder for trend analysis
	// In a real implementation, this would compare with historical data
	totalResults := len(results)
	failedResults := 0
	for _, result := range results {
		if result.IsFail {
			failedResults++
		}
	}

	var direction string
	var confidence float64

	failureRate := float64(failedResults) / float64(totalResults)
	if failureRate < 0.1 {
		direction = "stable"
		confidence = 0.8
	} else if failureRate < 0.3 {
		direction = "stable"
		confidence = 0.6
	} else {
		direction = "degrading"
		confidence = 0.7
	}

	return []Trend{
		{
			Category:    "overall",
			Direction:   direction,
			Confidence:  confidence,
			Description: fmt.Sprintf("System health appears %s based on current analysis", direction),
		},
	}
}

func (g *InsightsGenerator) generateRemediationInsights(steps []analyzer.RemediationStep) []RemediationInsight {
	var insights []RemediationInsight

	// Group by category and generate insights
	categories := make(map[string][]analyzer.RemediationStep)
	for _, step := range steps {
		category := step.Category
		if category == "" {
			category = "general"
		}
		categories[category] = append(categories[category], step)
	}

	for category, categorySteps := range categories {
		highPriorityCount := 0
		automatableCount := 0

		for _, step := range categorySteps {
			if step.Priority >= 8 {
				highPriorityCount++
			}
			if step.IsAutomatable {
				automatableCount++
			}
		}

		var impact, effort string
		if highPriorityCount > len(categorySteps)/2 {
			impact = "high"
		} else {
			impact = "medium"
		}

		if automatableCount > len(categorySteps)/2 {
			effort = "low"
		} else {
			effort = "medium"
		}

		insights = append(insights, RemediationInsight{
			Category: category,
			Priority: highPriorityCount,
			Impact:   impact,
			Effort:   effort,
			Description: fmt.Sprintf("%d steps in %s category, %d high priority",
				len(categorySteps), category, highPriorityCount),
		})
	}

	return insights
}

// RemediationGenerator generates remediation guide artifacts
type RemediationGenerator struct{}

func (g *RemediationGenerator) Generate(ctx context.Context, result *analyzer.AnalysisResult) (*Artifact, error) {
	guide := struct {
		Summary         string                                `json:"summary"`
		PriorityActions []analyzer.RemediationStep            `json:"priorityActions"`
		Categories      map[string][]analyzer.RemediationStep `json:"categories"`
		Prerequisites   []string                              `json:"prerequisites"`
		Automation      AutomationGuide                       `json:"automation"`
		GeneratedAt     time.Time                             `json:"generatedAt"`
	}{
		Summary:         g.generateRemediationSummary(result.Remediation),
		PriorityActions: g.getPriorityActions(result.Remediation, 5),
		Categories:      g.categorizeRemediationSteps(result.Remediation),
		Prerequisites:   g.identifyPrerequisites(result.Remediation),
		Automation:      g.generateAutomationGuide(result.Remediation),
		GeneratedAt:     time.Now(),
	}

	data, err := json.MarshalIndent(guide, "", "  ")
	if err != nil {
		return nil, err
	}

	artifact := &Artifact{
		Name:        "remediation-guide.json",
		Type:        "remediation",
		Format:      "json",
		ContentType: "application/json",
		Size:        int64(len(data)),
		Content:     data,
		Metadata: ArtifactMetadata{
			CreatedAt: time.Now(),
			Generator: "RemediationGenerator",
			Version:   "1.0.0",
			Tags:      []string{"remediation", "guide", "actions"},
		},
	}

	return artifact, nil
}

func (g *RemediationGenerator) Name() string {
	return "Remediation Generator"
}

func (g *RemediationGenerator) Description() string {
	return "Generates detailed remediation guide artifacts"
}

func (g *RemediationGenerator) generateRemediationSummary(steps []analyzer.RemediationStep) string {
	if len(steps) == 0 {
		return "No remediation steps required"
	}

	automatable := 0
	highPriority := 0

	for _, step := range steps {
		if step.IsAutomatable {
			automatable++
		}
		if step.Priority >= 8 {
			highPriority++
		}
	}

	return fmt.Sprintf("Found %d remediation steps: %d high priority, %d automatable",
		len(steps), highPriority, automatable)
}

func (g *RemediationGenerator) getPriorityActions(steps []analyzer.RemediationStep, limit int) []analyzer.RemediationStep {
	sorted := make([]analyzer.RemediationStep, len(steps))
	copy(sorted, steps)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority > sorted[j].Priority
	})

	if len(sorted) > limit {
		return sorted[:limit]
	}
	return sorted
}

func (g *RemediationGenerator) categorizeRemediationSteps(steps []analyzer.RemediationStep) map[string][]analyzer.RemediationStep {
	categories := make(map[string][]analyzer.RemediationStep)

	for _, step := range steps {
		category := step.Category
		if category == "" {
			category = "general"
		}
		categories[category] = append(categories[category], step)
	}

	return categories
}

func (g *RemediationGenerator) identifyPrerequisites(steps []analyzer.RemediationStep) []string {
	var prerequisites []string

	categoryPrereqs := map[string]string{
		"infrastructure": "Admin access to cluster nodes",
		"networking":     "Network configuration permissions",
		"storage":        "Storage admin permissions",
		"security":       "Security policy modification rights",
	}

	categories := make(map[string]bool)
	for _, step := range steps {
		if step.Category != "" {
			categories[step.Category] = true
		}
	}

	for category := range categories {
		if prereq, exists := categoryPrereqs[category]; exists {
			prerequisites = append(prerequisites, prereq)
		}
	}

	return prerequisites
}

func (g *RemediationGenerator) generateAutomationGuide(steps []analyzer.RemediationStep) AutomationGuide {
	automatable := 0
	manual := 0

	for _, step := range steps {
		if step.IsAutomatable {
			automatable++
		} else {
			manual++
		}
	}

	var scripts []Script
	if automatable > 0 {
		scripts = append(scripts, Script{
			Name:          "automated-remediation.sh",
			Description:   "Automated remediation script for detected issues",
			Language:      "bash",
			Content:       g.generateRemediationScript(steps),
			Prerequisites: []string{"kubectl", "admin access", "bash"},
		})
	}

	return AutomationGuide{
		AutomatableSteps: automatable,
		ManualSteps:      manual,
		Scripts:          scripts,
	}
}

func (g *RemediationGenerator) generateRemediationScript(steps []analyzer.RemediationStep) string {
	script := `#!/bin/bash
# Automated Remediation Script
# Generated by Troubleshoot Analysis Engine

set -e

echo "Starting automated remediation..."

`

	for i, step := range steps {
		if step.IsAutomatable && step.Command != "" {
			script += fmt.Sprintf(`
# Step %d: %s
echo "Executing: %s"
if %s; then
    echo "✅ Step %d completed successfully"
else
    echo "❌ Step %d failed - manual intervention required"
fi

`, i+1, step.Description, step.Description, step.Command, i+1, i+1)
		}
	}

	script += `
echo "Automated remediation completed. Please review any failed steps manually."
`

	return script
}

// CorrelationGenerator generates correlation matrix artifacts
type CorrelationGenerator struct{}

func (g *CorrelationGenerator) Generate(ctx context.Context, result *analyzer.AnalysisResult) (*Artifact, error) {
	correlations := g.buildCorrelationMatrix(result.Results)

	data, err := json.MarshalIndent(correlations, "", "  ")
	if err != nil {
		return nil, err
	}

	artifact := &Artifact{
		Name:        "correlations.json",
		Type:        "correlations",
		Format:      "json",
		ContentType: "application/json",
		Size:        int64(len(data)),
		Content:     data,
		Metadata: ArtifactMetadata{
			CreatedAt: time.Now(),
			Generator: "CorrelationGenerator",
			Version:   "1.0.0",
			Tags:      []string{"correlations", "relationships"},
		},
	}

	return artifact, nil
}

func (g *CorrelationGenerator) Name() string {
	return "Correlation Generator"
}

func (g *CorrelationGenerator) Description() string {
	return "Generates correlation matrix and relationship artifacts"
}

func (g *CorrelationGenerator) buildCorrelationMatrix(results []*analyzer.AnalyzerResult) map[string]interface{} {
	correlations := make(map[string]interface{})

	// Namespace-based correlations
	namespaceFailures := make(map[string][]string)
	namespaceWarnings := make(map[string][]string)

	for _, result := range results {
		if result.InvolvedObject != nil && result.InvolvedObject.Namespace != "" {
			namespace := result.InvolvedObject.Namespace
			if result.IsFail {
				namespaceFailures[namespace] = append(namespaceFailures[namespace], result.Title)
			} else if result.IsWarn {
				namespaceWarnings[namespace] = append(namespaceWarnings[namespace], result.Title)
			}
		}
	}

	correlations["namespace_failures"] = namespaceFailures
	correlations["namespace_warnings"] = namespaceWarnings

	// Category-based correlations
	categoryCorrelations := make(map[string]map[string]int)
	for _, result := range results {
		if result.Category != "" {
			if categoryCorrelations[result.Category] == nil {
				categoryCorrelations[result.Category] = make(map[string]int)
			}

			status := "unknown"
			if result.IsPass {
				status = "pass"
			} else if result.IsWarn {
				status = "warn"
			} else if result.IsFail {
				status = "fail"
			}

			categoryCorrelations[result.Category][status]++
		}
	}

	correlations["category_status_distribution"] = categoryCorrelations

	// Agent-based correlations
	agentResults := make(map[string]map[string]int)
	for _, result := range results {
		if result.AgentName != "" {
			if agentResults[result.AgentName] == nil {
				agentResults[result.AgentName] = make(map[string]int)
			}

			if result.IsPass {
				agentResults[result.AgentName]["pass"]++
			} else if result.IsWarn {
				agentResults[result.AgentName]["warn"]++
			} else if result.IsFail {
				agentResults[result.AgentName]["fail"]++
			}
		}
	}

	correlations["agent_performance"] = agentResults

	// Confidence correlations
	confidenceRanges := map[string]int{
		"high (>0.8)":      0,
		"medium (0.5-0.8)": 0,
		"low (<0.5)":       0,
		"unspecified":      0,
	}

	for _, result := range results {
		if result.Confidence > 0.8 {
			confidenceRanges["high (>0.8)"]++
		} else if result.Confidence > 0.5 {
			confidenceRanges["medium (0.5-0.8)"]++
		} else if result.Confidence > 0 {
			confidenceRanges["low (<0.5)"]++
		} else {
			confidenceRanges["unspecified"]++
		}
	}

	correlations["confidence_distribution"] = confidenceRanges

	return correlations
}

// GeneratorRegistry manages all artifact generators
type GeneratorRegistry struct {
	generators map[string]ArtifactGenerator
}

// NewGeneratorRegistry creates a new generator registry
func NewGeneratorRegistry() *GeneratorRegistry {
	registry := &GeneratorRegistry{
		generators: make(map[string]ArtifactGenerator),
	}

	// Register default generators
	registry.RegisterGenerator("summary", &SummaryGenerator{})
	registry.RegisterGenerator("insights", &InsightsGenerator{})
	registry.RegisterGenerator("remediation", &RemediationGenerator{})
	registry.RegisterGenerator("correlations", &CorrelationGenerator{})

	return registry
}

// RegisterGenerator registers a new generator
func (r *GeneratorRegistry) RegisterGenerator(name string, generator ArtifactGenerator) {
	r.generators[name] = generator
}

// GetGenerator gets a generator by name
func (r *GeneratorRegistry) GetGenerator(name string) (ArtifactGenerator, bool) {
	generator, exists := r.generators[name]
	return generator, exists
}

// GenerateArtifact generates an artifact using the specified generator
func (r *GeneratorRegistry) GenerateArtifact(ctx context.Context, generatorName string, result *analyzer.AnalysisResult) (*Artifact, error) {
	generator, exists := r.GetGenerator(generatorName)
	if !exists {
		return nil, fmt.Errorf("no generator found with name: %s", generatorName)
	}

	return generator.Generate(ctx, result)
}

// ListGenerators returns all available generator names
func (r *GeneratorRegistry) ListGenerators() []string {
	var names []string
	for name := range r.generators {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
