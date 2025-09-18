package artifacts

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"k8s.io/klog/v2"
)

// ArtifactManager handles generation and management of analysis artifacts
type ArtifactManager struct {
	outputDir   string
	templateDir string
	formatters  map[string]ArtifactFormatter
	generators  map[string]ArtifactGenerator
	validators  map[string]ArtifactValidator
}

// ArtifactFormatter formats analysis results into different output formats
type ArtifactFormatter interface {
	Format(ctx context.Context, result *analyzer.AnalysisResult) ([]byte, error)
	ContentType() string
	FileExtension() string
}

// ArtifactGenerator generates specific types of artifacts
type ArtifactGenerator interface {
	Generate(ctx context.Context, result *analyzer.AnalysisResult) (*Artifact, error)
	Name() string
	Description() string
}

// ArtifactValidator validates artifact content
type ArtifactValidator interface {
	Validate(ctx context.Context, data []byte) error
	Schema() string
}

// Artifact represents a generated analysis artifact
type Artifact struct {
	Name        string           `json:"name"`
	Type        string           `json:"type"`
	Format      string           `json:"format"`
	ContentType string           `json:"contentType"`
	Size        int64            `json:"size"`
	Path        string           `json:"path"`
	Metadata    ArtifactMetadata `json:"metadata"`
	Content     []byte           `json:"-"`
}

// ArtifactMetadata provides additional information about the artifact
type ArtifactMetadata struct {
	CreatedAt time.Time         `json:"createdAt"`
	Generator string            `json:"generator"`
	Version   string            `json:"version"`
	Summary   ArtifactSummary   `json:"summary"`
	Tags      []string          `json:"tags,omitempty"`
	Labels    map[string]string `json:"labels,omitempty"`
	Checksum  string            `json:"checksum,omitempty"`
}

// ArtifactSummary provides a high-level summary of the artifact contents
type ArtifactSummary struct {
	TotalResults   int      `json:"totalResults"`
	PassCount      int      `json:"passCount"`
	WarnCount      int      `json:"warnCount"`
	FailCount      int      `json:"failCount"`
	ErrorCount     int      `json:"errorCount"`
	Confidence     float64  `json:"confidence,omitempty"`
	AgentsUsed     []string `json:"agentsUsed"`
	TopCategories  []string `json:"topCategories,omitempty"`
	CriticalIssues int      `json:"criticalIssues"`
}

// ArtifactOptions configures artifact generation
type ArtifactOptions struct {
	OutputDir           string
	Formats             []string // e.g., ["json", "html", "yaml"]
	IncludeMetadata     bool
	IncludeRaw          bool
	IncludeCorrelations bool
	CompressOutput      bool
	Templates           map[string]string
	CustomFields        map[string]interface{}
}

// NewArtifactManager creates a new artifact manager
func NewArtifactManager(outputDir string) *ArtifactManager {
	am := &ArtifactManager{
		outputDir:  outputDir,
		formatters: make(map[string]ArtifactFormatter),
		generators: make(map[string]ArtifactGenerator),
		validators: make(map[string]ArtifactValidator),
	}

	// Register default formatters
	am.registerDefaultFormatters()

	// Register default generators
	am.registerDefaultGenerators()

	// Register default validators
	am.registerDefaultValidators()

	return am
}

// GenerateArtifacts generates all configured artifacts from analysis results
func (am *ArtifactManager) GenerateArtifacts(ctx context.Context, result *analyzer.AnalysisResult, opts *ArtifactOptions) ([]*Artifact, error) {
	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "ArtifactManager.GenerateArtifacts")
	defer span.End()

	if result == nil {
		return nil, errors.New("analysis result cannot be nil")
	}

	if opts == nil {
		opts = &ArtifactOptions{
			Formats:             []string{"json"},
			IncludeMetadata:     true,
			IncludeCorrelations: true,
		}
	}

	if opts.OutputDir != "" {
		am.outputDir = opts.OutputDir
	}

	// Ensure output directory exists
	if err := os.MkdirAll(am.outputDir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create output directory")
	}

	var artifacts []*Artifact

	// Generate primary analysis.json artifact
	analysisArtifact, err := am.generateAnalysisJSON(ctx, result, opts)
	if err != nil {
		klog.Errorf("Failed to generate analysis.json: %v", err)
	} else {
		artifacts = append(artifacts, analysisArtifact)
	}

	// Generate format-specific artifacts
	for _, format := range opts.Formats {
		if format == "json" {
			continue // Already generated above
		}

		artifact, err := am.generateFormatArtifact(ctx, result, format, opts)
		if err != nil {
			klog.Errorf("Failed to generate %s artifact: %v", format, err)
			continue
		}

		if artifact != nil {
			artifacts = append(artifacts, artifact)
		}
	}

	// Generate supplementary artifacts
	if supplementaryArtifacts, err := am.generateSupplementaryArtifacts(ctx, result, opts); err == nil {
		artifacts = append(artifacts, supplementaryArtifacts...)
	}

	// Generate remediation guide if requested
	if opts.IncludeMetadata {
		if remediationArtifact, err := am.generateRemediationGuide(ctx, result, opts); err == nil {
			artifacts = append(artifacts, remediationArtifact)
		}
	}

	span.SetAttributes(
		attribute.Int("total_artifacts", len(artifacts)),
		attribute.StringSlice("formats", opts.Formats),
	)

	return artifacts, nil
}

// generateAnalysisJSON creates the primary analysis.json artifact
func (am *ArtifactManager) generateAnalysisJSON(ctx context.Context, result *analyzer.AnalysisResult, opts *ArtifactOptions) (*Artifact, error) {
	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "ArtifactManager.generateAnalysisJSON")
	defer span.End()

	// Enhance the analysis result with additional metadata for the artifact
	enhancedResult := am.enhanceAnalysisResult(result, opts)

	// Format as JSON
	data, err := json.MarshalIndent(enhancedResult, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal analysis result")
	}

	// Validate JSON structure
	if validator, exists := am.validators["json"]; exists {
		if err := validator.Validate(ctx, data); err != nil {
			klog.Warningf("Analysis JSON validation failed: %v", err)
		}
	}

	// Create artifact
	artifact := &Artifact{
		Name:        "analysis.json",
		Type:        "analysis",
		Format:      "json",
		ContentType: "application/json",
		Size:        int64(len(data)),
		Content:     data,
		Metadata: ArtifactMetadata{
			CreatedAt: time.Now(),
			Generator: "troubleshoot-analysis-engine",
			Version:   "1.0.0",
			Summary:   am.generateSummary(result),
			Tags:      []string{"analysis", "primary"},
		},
	}

	// Write to file
	artifactPath := filepath.Join(am.outputDir, artifact.Name)
	if err := am.writeArtifact(artifact, artifactPath); err != nil {
		return nil, errors.Wrap(err, "failed to write analysis.json")
	}

	artifact.Path = artifactPath

	span.SetAttributes(
		attribute.String("artifact_name", artifact.Name),
		attribute.Int64("artifact_size", artifact.Size),
	)

	return artifact, nil
}

// generateFormatArtifact creates artifacts in specific formats
func (am *ArtifactManager) generateFormatArtifact(ctx context.Context, result *analyzer.AnalysisResult, format string, opts *ArtifactOptions) (*Artifact, error) {
	formatter, exists := am.formatters[format]
	if !exists {
		return nil, errors.Errorf("unsupported format: %s", format)
	}

	data, err := formatter.Format(ctx, result)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to format as %s", format)
	}

	filename := fmt.Sprintf("analysis.%s", formatter.FileExtension())
	artifact := &Artifact{
		Name:        filename,
		Type:        "analysis",
		Format:      format,
		ContentType: formatter.ContentType(),
		Size:        int64(len(data)),
		Content:     data,
		Metadata: ArtifactMetadata{
			CreatedAt: time.Now(),
			Generator: "troubleshoot-analysis-engine",
			Version:   "1.0.0",
			Summary:   am.generateSummary(result),
			Tags:      []string{"analysis", format},
		},
	}

	// Write to file
	artifactPath := filepath.Join(am.outputDir, filename)
	if err := am.writeArtifact(artifact, artifactPath); err != nil {
		return nil, errors.Wrapf(err, "failed to write %s artifact", format)
	}

	artifact.Path = artifactPath
	return artifact, nil
}

// generateSupplementaryArtifacts creates additional helpful artifacts
func (am *ArtifactManager) generateSupplementaryArtifacts(ctx context.Context, result *analyzer.AnalysisResult, opts *ArtifactOptions) ([]*Artifact, error) {
	var artifacts []*Artifact

	// Generate summary artifact
	summaryArtifact, err := am.generateSummaryArtifact(ctx, result, opts)
	if err == nil {
		artifacts = append(artifacts, summaryArtifact)
	}

	// Generate insights artifact
	insightsArtifact, err := am.generateInsightsArtifact(ctx, result, opts)
	if err == nil {
		artifacts = append(artifacts, insightsArtifact)
	}

	// Generate correlation matrix if requested
	if opts.IncludeCorrelations {
		correlationArtifact, err := am.generateCorrelationArtifact(ctx, result, opts)
		if err == nil {
			artifacts = append(artifacts, correlationArtifact)
		}
	}

	return artifacts, nil
}

// generateSummaryArtifact creates a high-level summary artifact
func (am *ArtifactManager) generateSummaryArtifact(ctx context.Context, result *analyzer.AnalysisResult, opts *ArtifactOptions) (*Artifact, error) {
	summary := struct {
		Overview        analyzer.AnalysisSummary   `json:"overview"`
		TopIssues       []*analyzer.AnalyzerResult `json:"topIssues"`
		Categories      map[string]int             `json:"categories"`
		Agents          []analyzer.AgentMetadata   `json:"agents"`
		Recommendations []string                   `json:"recommendations"`
		GeneratedAt     time.Time                  `json:"generatedAt"`
	}{
		Overview:        result.Summary,
		Categories:      am.categorizeResults(result.Results),
		Agents:          result.Metadata.Agents,
		TopIssues:       am.getTopIssues(result.Results, 10),
		Recommendations: am.generateRecommendations(result),
		GeneratedAt:     time.Now(),
	}

	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal summary")
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
			Generator: "troubleshoot-analysis-engine",
			Version:   "1.0.0",
			Summary:   am.generateSummary(result),
			Tags:      []string{"summary", "overview"},
		},
	}

	artifactPath := filepath.Join(am.outputDir, artifact.Name)
	if err := am.writeArtifact(artifact, artifactPath); err != nil {
		return nil, errors.Wrap(err, "failed to write summary artifact")
	}

	artifact.Path = artifactPath
	return artifact, nil
}

// generateInsightsArtifact creates an insights and correlation artifact
func (am *ArtifactManager) generateInsightsArtifact(ctx context.Context, result *analyzer.AnalysisResult, opts *ArtifactOptions) (*Artifact, error) {
	insights := struct {
		KeyFindings     []string               `json:"keyFindings"`
		Patterns        []Pattern              `json:"patterns"`
		Correlations    []analyzer.Correlation `json:"correlations"`
		Trends          []Trend                `json:"trends"`
		Recommendations []RemediationInsight   `json:"recommendations"`
		GeneratedAt     time.Time              `json:"generatedAt"`
	}{
		KeyFindings:     am.extractKeyFindings(result.Results),
		Patterns:        am.identifyPatterns(result.Results),
		Correlations:    result.Metadata.Correlations,
		Trends:          am.analyzeTrends(result.Results),
		Recommendations: am.generateRemediationInsights(result.Remediation),
		GeneratedAt:     time.Now(),
	}

	data, err := json.MarshalIndent(insights, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal insights")
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
			Generator: "troubleshoot-analysis-engine",
			Version:   "1.0.0",
			Summary:   am.generateSummary(result),
			Tags:      []string{"insights", "patterns", "correlations"},
		},
	}

	artifactPath := filepath.Join(am.outputDir, artifact.Name)
	if err := am.writeArtifact(artifact, artifactPath); err != nil {
		return nil, errors.Wrap(err, "failed to write insights artifact")
	}

	artifact.Path = artifactPath
	return artifact, nil
}

// generateCorrelationArtifact creates a correlation matrix artifact
func (am *ArtifactManager) generateCorrelationArtifact(ctx context.Context, result *analyzer.AnalysisResult, opts *ArtifactOptions) (*Artifact, error) {
	correlations := am.buildCorrelationMatrix(result.Results)

	data, err := json.MarshalIndent(correlations, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal correlations")
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
			Generator: "troubleshoot-analysis-engine",
			Version:   "1.0.0",
			Summary:   am.generateSummary(result),
			Tags:      []string{"correlations", "relationships"},
		},
	}

	artifactPath := filepath.Join(am.outputDir, artifact.Name)
	if err := am.writeArtifact(artifact, artifactPath); err != nil {
		return nil, errors.Wrap(err, "failed to write correlations artifact")
	}

	artifact.Path = artifactPath
	return artifact, nil
}

// generateRemediationGuide creates a detailed remediation guide
func (am *ArtifactManager) generateRemediationGuide(ctx context.Context, result *analyzer.AnalysisResult, opts *ArtifactOptions) (*Artifact, error) {
	guide := struct {
		Summary         string                                `json:"summary"`
		PriorityActions []analyzer.RemediationStep            `json:"priorityActions"`
		Categories      map[string][]analyzer.RemediationStep `json:"categories"`
		Prerequisites   []string                              `json:"prerequisites"`
		Automation      AutomationGuide                       `json:"automation"`
		GeneratedAt     time.Time                             `json:"generatedAt"`
	}{
		Summary:         am.generateRemediationSummary(result.Remediation),
		PriorityActions: am.getPriorityActions(result.Remediation, 5),
		Categories:      am.categorizeRemediationSteps(result.Remediation),
		Prerequisites:   am.identifyPrerequisites(result.Remediation),
		Automation:      am.generateAutomationGuide(result.Remediation),
		GeneratedAt:     time.Now(),
	}

	data, err := json.MarshalIndent(guide, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal remediation guide")
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
			Generator: "troubleshoot-analysis-engine",
			Version:   "1.0.0",
			Summary:   am.generateSummary(result),
			Tags:      []string{"remediation", "guide", "actions"},
		},
	}

	artifactPath := filepath.Join(am.outputDir, artifact.Name)
	if err := am.writeArtifact(artifact, artifactPath); err != nil {
		return nil, errors.Wrap(err, "failed to write remediation guide")
	}

	artifact.Path = artifactPath
	return artifact, nil
}

// Helper types for insights and patterns

type Pattern struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Count       int      `json:"count"`
	Confidence  float64  `json:"confidence"`
	Examples    []string `json:"examples,omitempty"`
}

type Trend struct {
	Category    string  `json:"category"`
	Direction   string  `json:"direction"` // "improving", "degrading", "stable"
	Confidence  float64 `json:"confidence"`
	Description string  `json:"description"`
}

type RemediationInsight struct {
	Category    string `json:"category"`
	Priority    int    `json:"priority"`
	Impact      string `json:"impact"`
	Effort      string `json:"effort"`
	Description string `json:"description"`
}

type AutomationGuide struct {
	AutomatableSteps int      `json:"automatableSteps"`
	ManualSteps      int      `json:"manualSteps"`
	Scripts          []Script `json:"scripts,omitempty"`
}

type Script struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Language      string   `json:"language"`
	Content       string   `json:"content"`
	Prerequisites []string `json:"prerequisites,omitempty"`
}

// Helper methods for analysis and insights

func (am *ArtifactManager) enhanceAnalysisResult(result *analyzer.AnalysisResult, opts *ArtifactOptions) *analyzer.AnalysisResult {
	// Create a copy to avoid modifying the original
	enhanced := &analyzer.AnalysisResult{
		Results:     result.Results,
		Remediation: result.Remediation,
		Summary:     result.Summary,
		Metadata:    result.Metadata,
		Errors:      result.Errors,
	}

	// Add artifact-specific metadata
	enhanced.Metadata.Timestamp = time.Now()

	// Add custom fields if provided
	if opts.CustomFields != nil {
		// Note: In a real implementation, you'd need to extend the struct
		// or use a more flexible data structure
	}

	return enhanced
}

func (am *ArtifactManager) generateSummary(result *analyzer.AnalysisResult) ArtifactSummary {
	summary := ArtifactSummary{
		TotalResults:   len(result.Results),
		PassCount:      result.Summary.PassCount,
		WarnCount:      result.Summary.WarnCount,
		FailCount:      result.Summary.FailCount,
		ErrorCount:     result.Summary.ErrorCount,
		Confidence:     result.Summary.Confidence,
		AgentsUsed:     result.Summary.AgentsUsed,
		TopCategories:  am.getTopCategories(result.Results, 5),
		CriticalIssues: am.countCriticalIssues(result.Results),
	}

	return summary
}

func (am *ArtifactManager) categorizeResults(results []*analyzer.AnalyzerResult) map[string]int {
	categories := make(map[string]int)

	for _, result := range results {
		if result.Category != "" {
			categories[result.Category]++
		}
	}

	return categories
}

func (am *ArtifactManager) getTopIssues(results []*analyzer.AnalyzerResult, limit int) []*analyzer.AnalyzerResult {
	// Filter for failed results
	var failedResults []*analyzer.AnalyzerResult
	for _, result := range results {
		if result.IsFail {
			failedResults = append(failedResults, result)
		}
	}

	// Sort by confidence (higher first)
	sort.Slice(failedResults, func(i, j int) bool {
		return failedResults[i].Confidence > failedResults[j].Confidence
	})

	// Return top N
	if len(failedResults) > limit {
		return failedResults[:limit]
	}
	return failedResults
}

func (am *ArtifactManager) getTopCategories(results []*analyzer.AnalyzerResult, limit int) []string {
	categories := am.categorizeResults(results)

	// Convert to slice for sorting
	type categoryCount struct {
		name  string
		count int
	}

	var categoryCounts []categoryCount
	for name, count := range categories {
		categoryCounts = append(categoryCounts, categoryCount{name, count})
	}

	// Sort by count (descending)
	sort.Slice(categoryCounts, func(i, j int) bool {
		return categoryCounts[i].count > categoryCounts[j].count
	})

	// Extract top category names
	var topCategories []string
	for i, cc := range categoryCounts {
		if i >= limit {
			break
		}
		topCategories = append(topCategories, cc.name)
	}

	return topCategories
}

func (am *ArtifactManager) countCriticalIssues(results []*analyzer.AnalyzerResult) int {
	count := 0
	for _, result := range results {
		if result.IsFail && strings.Contains(strings.ToLower(result.Severity), "critical") {
			count++
		}
	}
	return count
}

func (am *ArtifactManager) generateRecommendations(result *analyzer.AnalysisResult) []string {
	var recommendations []string

	// Generate high-level recommendations based on analysis results
	if result.Summary.FailCount > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Address %d failed checks to improve system health", result.Summary.FailCount))
	}

	if result.Summary.WarnCount > result.Summary.PassCount {
		recommendations = append(recommendations,
			"Review warning conditions to prevent potential issues")
	}

	// Category-specific recommendations
	categories := am.categorizeResults(result.Results)
	for category, count := range categories {
		if count >= 5 {
			recommendations = append(recommendations,
				fmt.Sprintf("Focus attention on %s category (%d issues)", category, count))
		}
	}

	return recommendations
}

func (am *ArtifactManager) extractKeyFindings(results []*analyzer.AnalyzerResult) []string {
	var findings []string

	for _, result := range results {
		if result.IsFail && result.Confidence > 0.8 {
			findings = append(findings, result.Message)
		}
	}

	// Limit to most important findings
	if len(findings) > 10 {
		findings = findings[:10]
	}

	return findings
}

func (am *ArtifactManager) identifyPatterns(results []*analyzer.AnalyzerResult) []Pattern {
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

	return patterns
}

func (am *ArtifactManager) analyzeTrends(results []*analyzer.AnalyzerResult) []Trend {
	// Placeholder for trend analysis
	// In a real implementation, this would compare with historical data
	return []Trend{
		{
			Category:    "overall",
			Direction:   "stable",
			Confidence:  0.7,
			Description: "System health appears stable",
		},
	}
}

func (am *ArtifactManager) buildCorrelationMatrix(results []*analyzer.AnalyzerResult) map[string]interface{} {
	// Placeholder for correlation analysis
	correlations := make(map[string]interface{})

	// Simple correlation example: failures in same namespace
	namespaceFailures := make(map[string][]string)
	for _, result := range results {
		if result.IsFail && result.InvolvedObject != nil {
			ns := result.InvolvedObject.Namespace
			if ns != "" {
				namespaceFailures[ns] = append(namespaceFailures[ns], result.Title)
			}
		}
	}

	correlations["namespace_failures"] = namespaceFailures
	return correlations
}

func (am *ArtifactManager) generateRemediationSummary(steps []analyzer.RemediationStep) string {
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

func (am *ArtifactManager) getPriorityActions(steps []analyzer.RemediationStep, limit int) []analyzer.RemediationStep {
	// Sort by priority (higher first)
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

func (am *ArtifactManager) categorizeRemediationSteps(steps []analyzer.RemediationStep) map[string][]analyzer.RemediationStep {
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

func (am *ArtifactManager) identifyPrerequisites(steps []analyzer.RemediationStep) []string {
	var prerequisites []string

	// Common prerequisites based on remediation categories
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

func (am *ArtifactManager) generateAutomationGuide(steps []analyzer.RemediationStep) AutomationGuide {
	automatable := 0
	manual := 0

	for _, step := range steps {
		if step.IsAutomatable {
			automatable++
		} else {
			manual++
		}
	}

	// Generate sample scripts for automatable steps
	var scripts []Script
	if automatable > 0 {
		scripts = append(scripts, Script{
			Name:          "automated-remediation.sh",
			Description:   "Automated remediation script",
			Language:      "bash",
			Content:       "#!/bin/bash\n# Automated remediation steps\necho 'Running automated fixes...'\n",
			Prerequisites: []string{"kubectl", "admin access"},
		})
	}

	return AutomationGuide{
		AutomatableSteps: automatable,
		ManualSteps:      manual,
		Scripts:          scripts,
	}
}

func (am *ArtifactManager) generateRemediationInsights(steps []analyzer.RemediationStep) []RemediationInsight {
	var insights []RemediationInsight

	// Group by category and generate insights
	categories := am.categorizeRemediationSteps(steps)

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

func (am *ArtifactManager) writeArtifact(artifact *Artifact, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return errors.Wrap(err, "failed to create artifact file")
	}
	defer file.Close()

	_, err = file.Write(artifact.Content)
	if err != nil {
		return errors.Wrap(err, "failed to write artifact content")
	}

	return nil
}

// Registration methods for formatters, generators, and validators

func (am *ArtifactManager) registerDefaultFormatters() {
	am.formatters["json"] = &JSONFormatter{}
	am.formatters["yaml"] = &YAMLFormatter{}
	am.formatters["html"] = &HTMLFormatter{}
	am.formatters["text"] = &TextFormatter{}
}

func (am *ArtifactManager) registerDefaultGenerators() {
	// Register specific artifact generators
	am.generators["summary"] = &SummaryGenerator{}
	am.generators["insights"] = &InsightsGenerator{}
	am.generators["remediation"] = &RemediationGenerator{}
}

func (am *ArtifactManager) registerDefaultValidators() {
	am.validators["json"] = &JSONValidator{}
	am.validators["yaml"] = &YAMLValidator{}
}

// RegisterFormatter registers a custom formatter
func (am *ArtifactManager) RegisterFormatter(name string, formatter ArtifactFormatter) {
	am.formatters[name] = formatter
}

// RegisterGenerator registers a custom generator
func (am *ArtifactManager) RegisterGenerator(name string, generator ArtifactGenerator) {
	am.generators[name] = generator
}

// RegisterValidator registers a custom validator
func (am *ArtifactManager) RegisterValidator(name string, validator ArtifactValidator) {
	am.validators[name] = validator
}

// WriteTo writes an artifact to a specific writer
func (am *ArtifactManager) WriteTo(artifact *Artifact, writer io.Writer) error {
	_, err := writer.Write(artifact.Content)
	return err
}
