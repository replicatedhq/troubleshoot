package local

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

// LocalAgent wraps the existing built-in analyzers with enhanced capabilities
type LocalAgent struct {
	name         string
	version      string
	capabilities []string
}

// NewLocalAgent creates a new local agent
func NewLocalAgent() *LocalAgent {
	return &LocalAgent{
		name:    "local",
		version: "1.0.0",
		capabilities: []string{
			"cluster-analysis",
			"host-analysis",
			"offline-analysis",
			"built-in-analyzers",
			"fast-execution",
		},
	}
}

// Name returns the agent name
func (a *LocalAgent) Name() string {
	return a.name
}

// Version returns the agent version
func (a *LocalAgent) Version() string {
	return a.version
}

// Capabilities returns the agent capabilities
func (a *LocalAgent) Capabilities() []string {
	return a.capabilities
}

// HealthCheck verifies the agent is working
func (a *LocalAgent) HealthCheck(ctx context.Context) error {
	// Verify we can create file content providers
	tmpDir, err := os.MkdirTemp("", "local-agent-health")
	if err != nil {
		return errors.Wrap(err, "cannot create temporary directory")
	}
	defer os.RemoveAll(tmpDir)

	return nil
}

// Analyze performs analysis using built-in analyzers with enhancements
func (a *LocalAgent) Analyze(ctx context.Context, bundle *analyzer.SupportBundle, analyzerSpecs []analyzer.AnalyzerSpec) (*analyzer.AgentResult, error) {
	startTime := time.Now()

	if bundle == nil {
		return nil, errors.New("bundle cannot be nil")
	}

	klog.V(2).Infof("LocalAgent starting analysis of bundle: %s", bundle.Path)

	// Discover existing analyzers from the bundle
	existingAnalyzers, hostAnalyzers, err := a.discoverAnalyzers(bundle)
	if err != nil {
		return nil, errors.Wrap(err, "failed to discover analyzers")
	}

	klog.V(2).Infof("Found %d cluster analyzers and %d host analyzers",
		len(existingAnalyzers), len(hostAnalyzers))

	// Run the existing analysis system
	basicResults, err := a.runExistingAnalyzers(ctx, bundle, existingAnalyzers, hostAnalyzers)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run existing analyzers")
	}

	// Enhance results with additional intelligence
	enhancedResults := a.enhanceResults(basicResults)

	// Generate insights from the enhanced results
	insights := a.generateInsights(enhancedResults)

	result := &analyzer.AgentResult{
		AgentName:      a.name,
		Results:        enhancedResults,
		Insights:       insights,
		ProcessingTime: time.Since(startTime),
		Metadata: map[string]interface{}{
			"analyzersRun":        len(existingAnalyzers) + len(hostAnalyzers),
			"bundlePath":          bundle.Path,
			"enhancementsApplied": true,
		},
	}

	klog.V(1).Infof("LocalAgent completed analysis: %d results, %d insights in %v",
		len(enhancedResults), len(insights), result.ProcessingTime)

	return result, nil
}

// Discover analyzers from the bundle or use default analyzers
func (a *LocalAgent) discoverAnalyzers(bundle *analyzer.SupportBundle) ([]*troubleshootv1beta2.Analyze, []*troubleshootv1beta2.HostAnalyze, error) {
	// Try to find analyzer specs in the bundle
	analyzers, hostAnalyzers := a.findAnalyzerSpecs(bundle)

	// If no analyzers found in bundle, use default set
	if len(analyzers) == 0 && len(hostAnalyzers) == 0 {
		klog.V(2).Info("No analyzer specs found in bundle, using default analyzers")
		return a.getDefaultAnalyzers()
	}

	return analyzers, hostAnalyzers, nil
}

// Find analyzer specifications in the bundle
func (a *LocalAgent) findAnalyzerSpecs(bundle *analyzer.SupportBundle) ([]*troubleshootv1beta2.Analyze, []*troubleshootv1beta2.HostAnalyze) {
	var analyzers []*troubleshootv1beta2.Analyze
	var hostAnalyzers []*troubleshootv1beta2.HostAnalyze

	// Look for analyzer specs in common locations
	specFiles := []string{
		"support-bundle-spec.yaml",
		"analyzer-spec.yaml",
		"troubleshoot.yaml",
		"preflight.yaml",
	}

	for _, specFile := range specFiles {
		data, err := bundle.GetFile(specFile)
		if err != nil {
			continue
		}

		// Parse the spec file to extract analyzers
		parsedAnalyzers, parsedHostAnalyzers, err := a.parseAnalyzerSpec(data)
		if err != nil {
			klog.V(2).Infof("Failed to parse %s: %v", specFile, err)
			continue
		}

		analyzers = append(analyzers, parsedAnalyzers...)
		hostAnalyzers = append(hostAnalyzers, parsedHostAnalyzers...)
	}

	return analyzers, hostAnalyzers
}

// Parse analyzer specification from YAML data
func (a *LocalAgent) parseAnalyzerSpec(data []byte) ([]*troubleshootv1beta2.Analyze, []*troubleshootv1beta2.HostAnalyze, error) {
	// This would implement YAML parsing similar to the existing system
	// For now, return empty slices - this will be implemented in detailed phase
	return []*troubleshootv1beta2.Analyze{}, []*troubleshootv1beta2.HostAnalyze{}, nil
}

// Get default analyzers for comprehensive analysis
func (a *LocalAgent) getDefaultAnalyzers() ([]*troubleshootv1beta2.Analyze, []*troubleshootv1beta2.HostAnalyze, error) {
	// Create a set of default analyzers that cover common use cases
	analyzers := []*troubleshootv1beta2.Analyze{
		// Cluster version check
		{
			ClusterVersion: &troubleshootv1beta2.ClusterVersion{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "< 1.20.0",
							Message: "Kubernetes version is below minimum supported version",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "< 1.24.0",
							Message: "Kubernetes version is supported but upgrade recommended",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Kubernetes version is supported",
						},
					},
				},
			},
		},

		// Node resources check
		{
			NodeResources: &troubleshootv1beta2.NodeResources{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "Node Resources",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "count() < 1",
							Message: "No nodes found in cluster",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Cluster has adequate node resources",
						},
					},
				},
			},
		},

		// Storage class check
		{
			StorageClass: &troubleshootv1beta2.StorageClass{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "Default Storage Class",
				},
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "== false",
							Message: "No default storage class found",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Default storage class is available",
						},
					},
				},
			},
		},
	}

	// Default host analyzers
	hostAnalyzers := []*troubleshootv1beta2.HostAnalyze{
		{
			CPU: &troubleshootv1beta2.CPUAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "count < 2",
							Message: "Insufficient CPU cores",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "CPU resources are adequate",
						},
					},
				},
			},
		},
		{
			Memory: &troubleshootv1beta2.MemoryAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "< 4G",
							Message: "Insufficient memory",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Memory resources are adequate",
						},
					},
				},
			},
		},
	}

	return analyzers, hostAnalyzers, nil
}

// Run the existing analyzer system
func (a *LocalAgent) runExistingAnalyzers(ctx context.Context, bundle *analyzer.SupportBundle,
	analyzers []*troubleshootv1beta2.Analyze, hostAnalyzers []*troubleshootv1beta2.HostAnalyze) ([]*analyzer.AnalyzeResult, error) {

	// Use the existing AnalyzeLocal function
	return analyzer.AnalyzeLocal(ctx, bundle.Path, analyzers, hostAnalyzers)
}

// Enhance basic analyzer results with additional intelligence
func (a *LocalAgent) enhanceResults(basicResults []*analyzer.AnalyzeResult) []analyzer.EnhancedAnalyzerResult {
	enhanced := make([]analyzer.EnhancedAnalyzerResult, 0, len(basicResults))

	for i, result := range basicResults {
		if result == nil {
			continue
		}

		enhancedResult := analyzer.EnhancedAnalyzerResult{
			// Copy basic fields
			IsPass:  result.IsPass,
			IsFail:  result.IsFail,
			IsWarn:  result.IsWarn,
			Strict:  result.Strict,
			Title:   result.Title,
			Message: result.Message,
			URI:     result.URI,
			IconKey: result.IconKey,
			IconURI: result.IconURI,

			// Add enhancements
			AgentUsed:  "local",
			Confidence: a.calculateConfidence(result),
			Impact:     a.determineImpact(result),
		}

		// Add explanation based on the result
		enhancedResult.Explanation = a.generateExplanation(result)

		// Add evidence
		enhancedResult.Evidence = a.gatherEvidence(result, basicResults)

		// Add remediation if this is a failure
		if result.IsFail {
			enhancedResult.Remediation = a.generateRemediation(result)
		}

		// Identify related issues
		enhancedResult.RelatedIssues = a.findRelatedIssues(result, basicResults, i)

		enhanced = append(enhanced, enhancedResult)
	}

	return enhanced
}

// Calculate confidence score for a result
func (a *LocalAgent) calculateConfidence(result *analyzer.AnalyzeResult) float64 {
	// Base confidence on result type and specificity
	baseConfidence := 0.8

	// Higher confidence for specific checks
	if strings.Contains(result.Title, "Version") {
		baseConfidence = 0.95
	}

	// Lower confidence for generic checks
	if strings.Contains(result.Message, "Unable to determine") {
		baseConfidence = 0.3
	}

	return baseConfidence
}

// Determine impact level for a result
func (a *LocalAgent) determineImpact(result *analyzer.AnalyzeResult) string {
	if result.IsFail {
		// Critical system components
		if strings.Contains(result.Title, "Version") ||
			strings.Contains(result.Title, "Node") ||
			strings.Contains(result.Title, "Storage") {
			return "HIGH"
		}
		return "MEDIUM"
	}

	if result.IsWarn {
		return "LOW"
	}

	return ""
}

// Generate detailed explanation for a result
func (a *LocalAgent) generateExplanation(result *analyzer.AnalyzeResult) string {
	if result.IsPass {
		return fmt.Sprintf("The %s check passed successfully. %s",
			strings.ToLower(result.Title), result.Message)
	}

	if result.IsFail {
		return fmt.Sprintf("The %s check failed. This indicates that %s. %s",
			strings.ToLower(result.Title),
			a.inferProblem(result),
			result.Message)
	}

	if result.IsWarn {
		return fmt.Sprintf("The %s check produced a warning. %s This may impact performance or reliability.",
			strings.ToLower(result.Title), result.Message)
	}

	return result.Message
}

// Infer the underlying problem from a failed result
func (a *LocalAgent) inferProblem(result *analyzer.AnalyzeResult) string {
	title := strings.ToLower(result.Title)
	message := strings.ToLower(result.Message)

	switch {
	case strings.Contains(title, "version"):
		return "the software version does not meet requirements"
	case strings.Contains(title, "memory") || strings.Contains(message, "memory"):
		return "there are memory-related issues"
	case strings.Contains(title, "cpu") || strings.Contains(message, "cpu"):
		return "there are CPU-related issues"
	case strings.Contains(title, "node") || strings.Contains(title, "resource"):
		return "there are insufficient cluster resources"
	case strings.Contains(title, "storage"):
		return "storage configuration is inadequate"
	case strings.Contains(title, "network") || strings.Contains(message, "connection"):
		return "there are network connectivity issues"
	default:
		return "a system requirement is not met"
	}
}

// Gather evidence supporting this result
func (a *LocalAgent) gatherEvidence(result *analyzer.AnalyzeResult, allResults []*analyzer.AnalyzeResult) []string {
	var evidence []string

	// The result message itself is evidence
	evidence = append(evidence, fmt.Sprintf("Check result: %s", result.Message))

	// Look for related failures that support this evidence
	for _, other := range allResults {
		if other == result || other == nil {
			continue
		}

		if a.areResultsRelated(result, other) && (other.IsFail || other.IsWarn) {
			evidence = append(evidence, fmt.Sprintf("Related issue: %s - %s", other.Title, other.Message))
		}
	}

	return evidence
}

// Check if two results are related
func (a *LocalAgent) areResultsRelated(result1, result2 *analyzer.AnalyzeResult) bool {
	// Don't relate to self
	if result1 == result2 {
		return false
	}

	// Simple heuristic: check if they share common domain-specific keywords
	keywords1 := strings.Fields(strings.ToLower(result1.Title))
	keywords2 := strings.Fields(strings.ToLower(result2.Title))

	// Filter out generic words
	genericWords := map[string]bool{
		"check": true, "status": true, "usage": true, "test": true, "analyze": true,
		"analyzer": true, "result": true, "resource": true, "resources": true,
	}

	for _, k1 := range keywords1 {
		if len(k1) <= 3 || genericWords[k1] {
			continue
		}
		for _, k2 := range keywords2 {
			if len(k2) <= 3 || genericWords[k2] {
				continue
			}
			if k1 == k2 {
				return true
			}
		}
	}

	return false
}

// Generate remediation steps for a failed result
func (a *LocalAgent) generateRemediation(result *analyzer.AnalyzeResult) *analyzer.RemediationStep {
	title := strings.ToLower(result.Title)

	remediation := &analyzer.RemediationStep{
		ID:          fmt.Sprintf("fix-%s", strings.ReplaceAll(title, " ", "-")),
		Title:       fmt.Sprintf("Fix %s Issue", result.Title),
		Description: fmt.Sprintf("Steps to resolve the %s problem", strings.ToLower(result.Title)),
		Category:    "immediate",
		Priority:    a.getPriority(result),
	}

	// Generate specific commands based on the issue type
	switch {
	case strings.Contains(title, "version"):
		remediation.Commands = []string{
			"kubectl version --client",
			"kubectl get nodes -o wide",
		}
		remediation.Manual = []string{
			"Check current Kubernetes version compatibility",
			"Plan upgrade if version is too old",
			"Consult upgrade documentation for your platform",
		}

	case strings.Contains(title, "storage"):
		remediation.Commands = []string{
			"kubectl get storageclass",
			"kubectl get pv",
			"kubectl get pvc --all-namespaces",
		}
		remediation.Manual = []string{
			"Verify storage class configuration",
			"Check available persistent volumes",
			"Create default storage class if missing",
		}

	case strings.Contains(title, "node") || strings.Contains(title, "resource"):
		remediation.Commands = []string{
			"kubectl get nodes",
			"kubectl describe nodes",
			"kubectl top nodes",
		}
		remediation.Manual = []string{
			"Check node status and capacity",
			"Add more nodes if resources are insufficient",
			"Review resource requests and limits",
		}

	default:
		remediation.Manual = []string{
			fmt.Sprintf("Review the %s configuration", strings.ToLower(result.Title)),
			"Check system logs for related errors",
			"Consult documentation for troubleshooting steps",
		}
	}

	// Add validation steps
	remediation.Validation = &analyzer.ValidationStep{
		Description: fmt.Sprintf("Verify that the %s issue is resolved", strings.ToLower(result.Title)),
		Commands:    []string{"Re-run the support bundle analysis"},
		Expected:    "The check should pass without errors",
	}

	return remediation
}

// Get priority for a result (1=highest, 10=lowest)
func (a *LocalAgent) getPriority(result *analyzer.AnalyzeResult) int {
	if result.IsFail {
		title := strings.ToLower(result.Title)

		// Critical infrastructure issues get highest priority
		if strings.Contains(title, "version") || strings.Contains(title, "node") {
			return 1
		}

		// Storage and resource issues are high priority
		if strings.Contains(title, "storage") || strings.Contains(title, "resource") {
			return 2
		}

		// Other failures are medium priority
		return 5
	}

	// Warnings are lower priority
	if result.IsWarn {
		return 7
	}

	// Passes are lowest priority
	if result.IsPass {
		return 10
	}

	// Unknown state
	return 9
}

// Find issues related to this result
func (a *LocalAgent) findRelatedIssues(result *analyzer.AnalyzeResult, allResults []*analyzer.AnalyzeResult, currentIndex int) []string {
	var related []string

	for i, other := range allResults {
		if i == currentIndex || other == nil || result == other {
			continue
		}

		if a.areResultsRelated(result, other) {
			related = append(related, fmt.Sprintf("%d", i)) // Use index as ID for now
		}
	}

	return related
}

// Generate insights from enhanced results
func (a *LocalAgent) generateInsights(results []analyzer.EnhancedAnalyzerResult) []analyzer.AnalysisInsight {
	var insights []analyzer.AnalysisInsight

	// Correlation insights
	correlations := a.findCorrelations(results)
	for _, correlation := range correlations {
		insights = append(insights, correlation)
	}

	// Trend insights (placeholder for now)
	trends := a.identifyTrends(results)
	for _, trend := range trends {
		insights = append(insights, trend)
	}

	// Recommendation insights
	recommendations := a.generateRecommendations(results)
	for _, recommendation := range recommendations {
		insights = append(insights, recommendation)
	}

	return insights
}

// Find correlations between different results
func (a *LocalAgent) findCorrelations(results []analyzer.EnhancedAnalyzerResult) []analyzer.AnalysisInsight {
	var insights []analyzer.AnalysisInsight

	// Simple correlation: if multiple resource-related checks fail
	resourceFailures := 0
	var resourceIssues []string

	for _, result := range results {
		if result.IsFail && (strings.Contains(strings.ToLower(result.Title), "resource") ||
			strings.Contains(strings.ToLower(result.Title), "node") ||
			strings.Contains(strings.ToLower(result.Title), "memory") ||
			strings.Contains(strings.ToLower(result.Title), "cpu")) {
			resourceFailures++
			resourceIssues = append(resourceIssues, result.Title)
		}
	}

	if resourceFailures >= 2 {
		insights = append(insights, analyzer.AnalysisInsight{
			ID:          "resource-correlation-001",
			Title:       "Multiple Resource Issues Detected",
			Description: fmt.Sprintf("Found %d resource-related issues that may be correlated. This suggests a broader infrastructure capacity problem.", resourceFailures),
			Type:        "correlation",
			Confidence:  0.8,
			Evidence:    resourceIssues,
			Impact:      "HIGH",
		})
	}

	return insights
}

// Identify trends (placeholder implementation)
func (a *LocalAgent) identifyTrends(results []analyzer.EnhancedAnalyzerResult) []analyzer.AnalysisInsight {
	// This would analyze historical data if available
	return []analyzer.AnalysisInsight{}
}

// Generate strategic recommendations
func (a *LocalAgent) generateRecommendations(results []analyzer.EnhancedAnalyzerResult) []analyzer.AnalysisInsight {
	var insights []analyzer.AnalysisInsight

	failures := 0
	warnings := 0

	for _, result := range results {
		if result.IsFail {
			failures++
		} else if result.IsWarn {
			warnings++
		}
	}

	// Overall health recommendation
	if failures > 0 || warnings > 3 {
		priority := "MEDIUM"
		if failures > 2 {
			priority = "HIGH"
		}

		insights = append(insights, analyzer.AnalysisInsight{
			ID:          "health-recommendation-001",
			Title:       "System Health Assessment",
			Description: fmt.Sprintf("Your system has %d failures and %d warnings. Consider addressing critical issues first, then systematically working through warnings.", failures, warnings),
			Type:        "recommendation",
			Confidence:  0.9,
			Impact:      priority,
			Evidence:    []string{fmt.Sprintf("%d failed checks", failures), fmt.Sprintf("%d warning checks", warnings)},
		})
	}

	return insights
}
