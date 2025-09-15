package local

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"k8s.io/klog/v2"
)

// LocalAgent implements the Agent interface using built-in analyzers
type LocalAgent struct {
	name         string
	version      string
	capabilities []string
	plugins      map[string]AnalyzerPlugin
	enabled      bool
}

// AnalyzerPlugin interface for custom analyzer plugins
type AnalyzerPlugin interface {
	Name() string
	Analyze(ctx context.Context, data map[string][]byte, config map[string]interface{}) (*analyzer.AnalyzerResult, error)
	Supports(analyzerType string) bool
}

// LocalAgentOptions configures the local agent
type LocalAgentOptions struct {
	EnablePlugins  bool
	PluginDir      string
	MaxConcurrency int
}

// NewLocalAgent creates a new local analysis agent
func NewLocalAgent(opts *LocalAgentOptions) *LocalAgent {
	if opts == nil {
		opts = &LocalAgentOptions{
			EnablePlugins:  false,
			MaxConcurrency: 10,
		}
	}

	agent := &LocalAgent{
		name:    "local",
		version: "1.0.0",
		capabilities: []string{
			"cluster-analysis",
			"host-analysis",
			"workload-analysis",
			"configuration-analysis",
			"log-analysis",
			"offline-analysis",
		},
		plugins: make(map[string]AnalyzerPlugin),
		enabled: true,
	}

	return agent
}

// Name returns the agent name
func (a *LocalAgent) Name() string {
	return a.name
}

// IsAvailable checks if the agent is available for analysis
func (a *LocalAgent) IsAvailable() bool {
	return a.enabled
}

// Capabilities returns the agent's capabilities
func (a *LocalAgent) Capabilities() []string {
	return append([]string{}, a.capabilities...)
}

// HealthCheck verifies the agent is functioning correctly
func (a *LocalAgent) HealthCheck(ctx context.Context) error {
	if !a.enabled {
		return errors.New("local agent is disabled")
	}
	return nil
}

// RegisterPlugin registers a custom analyzer plugin
func (a *LocalAgent) RegisterPlugin(plugin AnalyzerPlugin) error {
	if plugin == nil {
		return errors.New("plugin cannot be nil")
	}

	name := plugin.Name()
	if name == "" {
		return errors.New("plugin name cannot be empty")
	}

	if _, exists := a.plugins[name]; exists {
		return errors.Errorf("plugin %s already registered", name)
	}

	a.plugins[name] = plugin
	return nil
}

// Analyze performs analysis using built-in analyzers and plugins
func (a *LocalAgent) Analyze(ctx context.Context, data []byte, analyzers []analyzer.AnalyzerSpec) (*analyzer.AgentResult, error) {
	startTime := time.Now()

	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "LocalAgent.Analyze")
	defer span.End()

	if !a.enabled {
		return nil, errors.New("local agent is not enabled")
	}

	// Parse the bundle data
	bundle := &analyzer.SupportBundle{}
	if err := json.Unmarshal(data, bundle); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal bundle data")
	}

	results := &analyzer.AgentResult{
		Results: make([]*analyzer.AnalyzerResult, 0),
		Metadata: analyzer.AgentResultMetadata{
			AnalyzerCount: len(analyzers),
			Version:       a.version,
		},
		Errors: make([]string, 0),
	}

	// If no specific analyzers provided, run built-in discovery
	if len(analyzers) == 0 {
		discoveredAnalyzers := a.discoverAnalyzers(bundle)
		analyzers = append(analyzers, discoveredAnalyzers...)
	}

	// Process each analyzer specification
	for _, analyzerSpec := range analyzers {
		result, err := a.runAnalyzer(ctx, bundle, analyzerSpec)
		if err != nil {
			klog.Errorf("Failed to run analyzer %s: %v", analyzerSpec.Name, err)
			results.Errors = append(results.Errors, fmt.Sprintf("analyzer %s failed: %v", analyzerSpec.Name, err))
			continue
		}

		if result != nil {
			// Enhance result with local agent metadata
			result.AgentName = a.name
			result.AnalyzerType = analyzerSpec.Type
			result.Category = analyzerSpec.Category
			result.Confidence = 0.9 // High confidence for built-in analyzers

			results.Results = append(results.Results, result)
		}
	}

	results.Metadata.Duration = time.Since(startTime)

	span.SetAttributes(
		attribute.Int("total_analyzers", len(analyzers)),
		attribute.Int("successful_results", len(results.Results)),
		attribute.Int("errors", len(results.Errors)),
	)

	return results, nil
}

// discoverAnalyzers automatically discovers analyzers to run based on bundle contents
func (a *LocalAgent) discoverAnalyzers(bundle *analyzer.SupportBundle) []analyzer.AnalyzerSpec {
	var specs []analyzer.AnalyzerSpec

	// Check for common Kubernetes resources and add appropriate analyzers
	for filePath := range bundle.Files {
		filePath = strings.ToLower(filePath)

		switch {
		case strings.Contains(filePath, "pods") && strings.HasSuffix(filePath, ".json"):
			specs = append(specs, analyzer.AnalyzerSpec{
				Name:     "pod-status-check",
				Type:     "workload",
				Category: "pods",
				Priority: 10,
				Config:   map[string]interface{}{"filePath": filePath},
			})

		case strings.Contains(filePath, "deployments") && strings.HasSuffix(filePath, ".json"):
			specs = append(specs, analyzer.AnalyzerSpec{
				Name:     "deployment-status-check",
				Type:     "workload",
				Category: "deployments",
				Priority: 9,
				Config:   map[string]interface{}{"filePath": filePath},
			})

		case strings.Contains(filePath, "services") && strings.HasSuffix(filePath, ".json"):
			specs = append(specs, analyzer.AnalyzerSpec{
				Name:     "service-check",
				Type:     "network",
				Category: "services",
				Priority: 8,
				Config:   map[string]interface{}{"filePath": filePath},
			})

		case strings.Contains(filePath, "events") && strings.HasSuffix(filePath, ".json"):
			specs = append(specs, analyzer.AnalyzerSpec{
				Name:     "event-analysis",
				Type:     "cluster",
				Category: "events",
				Priority: 7,
				Config:   map[string]interface{}{"filePath": filePath},
			})

		case strings.Contains(filePath, "nodes") && strings.HasSuffix(filePath, ".json"):
			specs = append(specs, analyzer.AnalyzerSpec{
				Name:     "node-resources-check",
				Type:     "cluster",
				Category: "nodes",
				Priority: 9,
				Config:   map[string]interface{}{"filePath": filePath},
			})

		case strings.Contains(filePath, "logs") && strings.HasSuffix(filePath, ".log"):
			specs = append(specs, analyzer.AnalyzerSpec{
				Name:     "log-analysis",
				Type:     "logs",
				Category: "logging",
				Priority: 6,
				Config:   map[string]interface{}{"filePath": filePath},
			})
		}
	}

	return specs
}

// runAnalyzer executes a specific analyzer based on the spec
func (a *LocalAgent) runAnalyzer(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, fmt.Sprintf("LocalAgent.%s", spec.Name))
	defer span.End()

	// Check if a plugin can handle this analyzer
	for _, plugin := range a.plugins {
		if plugin.Supports(spec.Type) {
			return plugin.Analyze(ctx, bundle.Files, spec.Config)
		}
	}

	// Use built-in analyzer logic based on type
	switch spec.Type {
	case "workload":
		return a.analyzeWorkload(ctx, bundle, spec)
	case "cluster":
		return a.analyzeCluster(ctx, bundle, spec)
	case "network":
		return a.analyzeNetwork(ctx, bundle, spec)
	case "logs":
		return a.analyzeLogs(ctx, bundle, spec)
	case "storage":
		return a.analyzeStorage(ctx, bundle, spec)
	case "resources":
		return a.analyzeResources(ctx, bundle, spec)
	case "custom":
		return a.analyzeCustom(ctx, bundle, spec)
	default:
		return nil, errors.Errorf("unsupported analyzer type: %s", spec.Type)
	}
}

// analyzeWorkload analyzes workload-related resources (pods, deployments, etc.)
func (a *LocalAgent) analyzeWorkload(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	result := &analyzer.AnalyzerResult{
		Title:      spec.Name,
		Category:   spec.Category,
		Confidence: 0.9,
	}

	switch spec.Name {
	case "pod-status-check":
		return a.analyzePodStatus(ctx, bundle, spec)
	case "deployment-status-check":
		return a.analyzeDeploymentStatus(ctx, bundle, spec)
	default:
		result.IsWarn = true
		result.Message = fmt.Sprintf("Workload analyzer %s not implemented yet", spec.Name)
		return result, nil
	}
}

// analyzePodStatus analyzes pod status and health
func (a *LocalAgent) analyzePodStatus(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	result := &analyzer.AnalyzerResult{
		Title:      "Pod Status Analysis",
		Category:   "pods",
		Confidence: 0.9,
	}

	filePath, ok := spec.Config["filePath"].(string)
	if !ok {
		return nil, errors.New("filePath not specified in analyzer config")
	}

	podData, exists := bundle.Files[filePath]
	if !exists {
		result.IsWarn = true
		result.Message = fmt.Sprintf("Pod data file not found: %s", filePath)
		return result, nil
	}

	// Try to parse as pod list first, then as single pod
	var pods []interface{}
	var podList map[string]interface{}

	if err := json.Unmarshal(podData, &podList); err == nil {
		if items, ok := podList["items"]; ok {
			if itemsArray, ok := items.([]interface{}); ok {
				pods = itemsArray
			}
		}
	}

	if len(pods) == 0 {
		// Try parsing as array directly
		if err := json.Unmarshal(podData, &pods); err != nil {
			result.IsFail = true
			result.Message = "Failed to parse pod data"
			return result, nil
		}
	}

	if len(pods) == 0 {
		result.IsWarn = true
		result.Message = "No pods found in the bundle"
		return result, nil
	}

	failedPods := 0
	pendingPods := 0
	runningPods := 0

	for _, podInterface := range pods {
		pod, ok := podInterface.(map[string]interface{})
		if !ok {
			continue
		}

		status, ok := pod["status"].(map[string]interface{})
		if !ok {
			continue
		}

		phase, _ := status["phase"].(string)
		switch phase {
		case "Running":
			runningPods++
		case "Pending":
			pendingPods++
		case "Failed", "Unknown":
			failedPods++
		}
	}

	totalPods := len(pods)

	if failedPods > 0 {
		result.IsFail = true
		result.Message = fmt.Sprintf("Found %d failed pods out of %d total pods", failedPods, totalPods)
		result.Remediation = &analyzer.RemediationStep{
			Description:   "Investigate failed pod logs and events",
			Action:        "check-logs",
			Command:       "kubectl logs <pod-name> -n <namespace>",
			Documentation: "https://kubernetes.io/docs/tasks/debug-application-cluster/debug-pods/",
			Priority:      9,
			Category:      "troubleshooting",
			IsAutomatable: false,
		}
	} else if pendingPods > totalPods/2 {
		result.IsWarn = true
		result.Message = fmt.Sprintf("Found %d pending pods out of %d total pods - may indicate scheduling issues", pendingPods, totalPods)
		result.Remediation = &analyzer.RemediationStep{
			Description:   "Check node resources and scheduling constraints",
			Action:        "check-scheduling",
			Command:       "kubectl describe pods <pod-name> -n <namespace>",
			Documentation: "https://kubernetes.io/docs/concepts/scheduling-eviction/",
			Priority:      6,
			Category:      "scheduling",
			IsAutomatable: false,
		}
	} else {
		result.IsPass = true
		result.Message = fmt.Sprintf("All %d pods are in healthy state (%d running, %d pending)", totalPods, runningPods, pendingPods)
	}

	result.Context = map[string]interface{}{
		"totalPods":   totalPods,
		"runningPods": runningPods,
		"pendingPods": pendingPods,
		"failedPods":  failedPods,
	}

	return result, nil
}

// analyzeDeploymentStatus analyzes deployment status and health
func (a *LocalAgent) analyzeDeploymentStatus(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	result := &analyzer.AnalyzerResult{
		Title:      "Deployment Status Analysis",
		Category:   "deployments",
		Confidence: 0.9,
	}

	filePath, ok := spec.Config["filePath"].(string)
	if !ok {
		return nil, errors.New("filePath not specified in analyzer config")
	}

	deploymentData, exists := bundle.Files[filePath]
	if !exists {
		result.IsWarn = true
		result.Message = fmt.Sprintf("Deployment data file not found: %s", filePath)
		return result, nil
	}

	var deployments []interface{}
	var deploymentList map[string]interface{}

	if err := json.Unmarshal(deploymentData, &deploymentList); err == nil {
		if items, ok := deploymentList["items"]; ok {
			if itemsArray, ok := items.([]interface{}); ok {
				deployments = itemsArray
			}
		}
	}

	if len(deployments) == 0 {
		if err := json.Unmarshal(deploymentData, &deployments); err != nil {
			result.IsFail = true
			result.Message = "Failed to parse deployment data"
			return result, nil
		}
	}

	if len(deployments) == 0 {
		result.IsWarn = true
		result.Message = "No deployments found in the bundle"
		return result, nil
	}

	unhealthyDeployments := 0
	totalDeployments := len(deployments)

	for _, deploymentInterface := range deployments {
		deployment, ok := deploymentInterface.(map[string]interface{})
		if !ok {
			continue
		}

		status, ok := deployment["status"].(map[string]interface{})
		if !ok {
			unhealthyDeployments++
			continue
		}

		replicas, _ := status["replicas"].(float64)
		readyReplicas, _ := status["readyReplicas"].(float64)

		if readyReplicas < replicas {
			unhealthyDeployments++
		}
	}

	if unhealthyDeployments > 0 {
		result.IsFail = true
		result.Message = fmt.Sprintf("Found %d unhealthy deployments out of %d total", unhealthyDeployments, totalDeployments)
		result.Remediation = &analyzer.RemediationStep{
			Description:   "Check deployment events and pod status",
			Action:        "check-deployment",
			Command:       "kubectl describe deployment <deployment-name> -n <namespace>",
			Documentation: "https://kubernetes.io/docs/concepts/workloads/controllers/deployment/",
			Priority:      8,
			Category:      "troubleshooting",
			IsAutomatable: false,
		}
	} else {
		result.IsPass = true
		result.Message = fmt.Sprintf("All %d deployments are healthy", totalDeployments)
	}

	result.Context = map[string]interface{}{
		"totalDeployments":     totalDeployments,
		"unhealthyDeployments": unhealthyDeployments,
	}

	return result, nil
}

// analyzeCluster analyzes cluster-level resources and configuration
func (a *LocalAgent) analyzeCluster(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	result := &analyzer.AnalyzerResult{
		Title:      fmt.Sprintf("Cluster Analysis: %s", spec.Name),
		Category:   spec.Category,
		Confidence: 0.8,
	}

	switch spec.Name {
	case "node-resources-check":
		return a.analyzeNodeResources(ctx, bundle, spec)
	case "event-analysis":
		return a.analyzeEvents(ctx, bundle, spec)
	default:
		result.IsWarn = true
		result.Message = fmt.Sprintf("Cluster analyzer %s not implemented yet", spec.Name)
		return result, nil
	}
}

// analyzeNodeResources analyzes node resource usage and capacity
func (a *LocalAgent) analyzeNodeResources(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	result := &analyzer.AnalyzerResult{
		Title:      "Node Resources Analysis",
		Category:   "nodes",
		Confidence: 0.9,
	}

	filePath, ok := spec.Config["filePath"].(string)
	if !ok {
		return nil, errors.New("filePath not specified in analyzer config")
	}

	nodeData, exists := bundle.Files[filePath]
	if !exists {
		result.IsWarn = true
		result.Message = fmt.Sprintf("Node data file not found: %s", filePath)
		return result, nil
	}

	var nodes []interface{}
	var nodeList map[string]interface{}

	if err := json.Unmarshal(nodeData, &nodeList); err == nil {
		if items, ok := nodeList["items"]; ok {
			if itemsArray, ok := items.([]interface{}); ok {
				nodes = itemsArray
			}
		}
	}

	if len(nodes) == 0 {
		if err := json.Unmarshal(nodeData, &nodes); err != nil {
			result.IsFail = true
			result.Message = "Failed to parse node data"
			return result, nil
		}
	}

	if len(nodes) == 0 {
		result.IsWarn = true
		result.Message = "No nodes found in the bundle"
		return result, nil
	}

	notReadyNodes := 0
	totalNodes := len(nodes)

	for _, nodeInterface := range nodes {
		node, ok := nodeInterface.(map[string]interface{})
		if !ok {
			continue
		}

		status, ok := node["status"].(map[string]interface{})
		if !ok {
			notReadyNodes++
			continue
		}

		conditions, ok := status["conditions"].([]interface{})
		if !ok {
			notReadyNodes++
			continue
		}

		nodeReady := false
		for _, condInterface := range conditions {
			cond, ok := condInterface.(map[string]interface{})
			if !ok {
				continue
			}

			if condType, ok := cond["type"].(string); ok && condType == "Ready" {
				if condStatus, ok := cond["status"].(string); ok && condStatus == "True" {
					nodeReady = true
					break
				}
			}
		}

		if !nodeReady {
			notReadyNodes++
		}
	}

	if notReadyNodes > 0 {
		result.IsFail = true
		result.Message = fmt.Sprintf("Found %d not ready nodes out of %d total nodes", notReadyNodes, totalNodes)
		result.Remediation = &analyzer.RemediationStep{
			Description:   "Investigate node conditions and events",
			Action:        "check-nodes",
			Command:       "kubectl describe nodes",
			Documentation: "https://kubernetes.io/docs/concepts/architecture/nodes/",
			Priority:      10,
			Category:      "infrastructure",
			IsAutomatable: false,
		}
	} else {
		result.IsPass = true
		result.Message = fmt.Sprintf("All %d nodes are ready", totalNodes)
	}

	result.Context = map[string]interface{}{
		"totalNodes":    totalNodes,
		"notReadyNodes": notReadyNodes,
	}

	return result, nil
}

// analyzeEvents analyzes cluster events for issues
func (a *LocalAgent) analyzeEvents(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	result := &analyzer.AnalyzerResult{
		Title:      "Event Analysis",
		Category:   "events",
		Confidence: 0.8,
	}

	filePath, ok := spec.Config["filePath"].(string)
	if !ok {
		return nil, errors.New("filePath not specified in analyzer config")
	}

	eventData, exists := bundle.Files[filePath]
	if !exists {
		result.IsWarn = true
		result.Message = fmt.Sprintf("Event data file not found: %s", filePath)
		return result, nil
	}

	var events []interface{}
	var eventList map[string]interface{}

	if err := json.Unmarshal(eventData, &eventList); err == nil {
		if items, ok := eventList["items"]; ok {
			if itemsArray, ok := items.([]interface{}); ok {
				events = itemsArray
			}
		}
	}

	if len(events) == 0 {
		if err := json.Unmarshal(eventData, &events); err != nil {
			result.IsFail = true
			result.Message = "Failed to parse event data"
			return result, nil
		}
	}

	warningEvents := 0
	errorEvents := 0

	for _, eventInterface := range events {
		event, ok := eventInterface.(map[string]interface{})
		if !ok {
			continue
		}

		eventType, _ := event["type"].(string)
		reason, _ := event["reason"].(string)

		switch eventType {
		case "Warning":
			warningEvents++
			if strings.Contains(strings.ToLower(reason), "failed") ||
				strings.Contains(strings.ToLower(reason), "error") ||
				strings.Contains(strings.ToLower(reason), "unhealthy") {
				errorEvents++
			}
		}
	}

	totalEvents := len(events)

	if errorEvents > 5 {
		result.IsFail = true
		result.Message = fmt.Sprintf("Found %d error events out of %d total events", errorEvents, totalEvents)
		result.Remediation = &analyzer.RemediationStep{
			Description:   "Review error events and their causes",
			Action:        "check-events",
			Command:       "kubectl get events --sort-by=.metadata.creationTimestamp",
			Documentation: "https://kubernetes.io/docs/tasks/debug-application-cluster/debug-cluster/",
			Priority:      7,
			Category:      "troubleshooting",
			IsAutomatable: false,
		}
	} else if warningEvents > 10 {
		result.IsWarn = true
		result.Message = fmt.Sprintf("Found %d warning events - may indicate potential issues", warningEvents)
		result.Remediation = &analyzer.RemediationStep{
			Description:   "Review warning events for potential issues",
			Action:        "review-warnings",
			Command:       "kubectl get events --field-selector type=Warning",
			Documentation: "https://kubernetes.io/docs/tasks/debug-application-cluster/debug-cluster/",
			Priority:      5,
			Category:      "monitoring",
			IsAutomatable: false,
		}
	} else {
		result.IsPass = true
		result.Message = fmt.Sprintf("Event analysis looks good (%d total events, %d warnings)", totalEvents, warningEvents)
	}

	result.Context = map[string]interface{}{
		"totalEvents":   totalEvents,
		"warningEvents": warningEvents,
		"errorEvents":   errorEvents,
	}

	return result, nil
}

// analyzeNetwork analyzes network-related resources
func (a *LocalAgent) analyzeNetwork(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	result := &analyzer.AnalyzerResult{
		Title:      fmt.Sprintf("Network Analysis: %s", spec.Name),
		Category:   spec.Category,
		Confidence: 0.7,
	}

	// Placeholder for network analysis
	result.IsWarn = true
	result.Message = fmt.Sprintf("Network analyzer %s not implemented yet", spec.Name)
	return result, nil
}

// analyzeLogs analyzes log files for issues
func (a *LocalAgent) analyzeLogs(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	result := &analyzer.AnalyzerResult{
		Title:      "Log Analysis",
		Category:   "logging",
		Confidence: 0.8,
	}

	filePath, ok := spec.Config["filePath"].(string)
	if !ok {
		return nil, errors.New("filePath not specified in analyzer config")
	}

	logData, exists := bundle.Files[filePath]
	if !exists {
		result.IsWarn = true
		result.Message = fmt.Sprintf("Log file not found: %s", filePath)
		return result, nil
	}

	logContent := string(logData)
	lines := strings.Split(logContent, "\n")

	errorCount := 0
	warningCount := 0

	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "error") || strings.Contains(lowerLine, "fatal") {
			errorCount++
		} else if strings.Contains(lowerLine, "warn") {
			warningCount++
		}
	}

	totalLines := len(lines)

	if errorCount > 10 {
		result.IsFail = true
		result.Message = fmt.Sprintf("Found %d error lines in log file (total %d lines)", errorCount, totalLines)
		result.Remediation = &analyzer.RemediationStep{
			Description:   "Review error messages in logs",
			Action:        "review-logs",
			Documentation: "https://kubernetes.io/docs/concepts/cluster-administration/logging/",
			Priority:      6,
			Category:      "troubleshooting",
			IsAutomatable: false,
		}
	} else if warningCount > 20 {
		result.IsWarn = true
		result.Message = fmt.Sprintf("Found %d warning lines in logs - monitor for issues", warningCount)
	} else {
		result.IsPass = true
		result.Message = fmt.Sprintf("Log analysis looks good (%d total lines, %d warnings, %d errors)", totalLines, warningCount, errorCount)
	}

	result.Context = map[string]interface{}{
		"totalLines":   totalLines,
		"errorCount":   errorCount,
		"warningCount": warningCount,
		"fileName":     filePath,
	}

	return result, nil
}

// analyzeStorage analyzes storage-related resources
func (a *LocalAgent) analyzeStorage(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	result := &analyzer.AnalyzerResult{
		Title:      fmt.Sprintf("Storage Analysis: %s", spec.Name),
		Category:   spec.Category,
		Confidence: 0.7,
	}

	// Placeholder for storage analysis
	result.IsWarn = true
	result.Message = fmt.Sprintf("Storage analyzer %s not implemented yet", spec.Name)
	return result, nil
}

// analyzeResources analyzes resource usage and requirements
func (a *LocalAgent) analyzeResources(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	result := &analyzer.AnalyzerResult{
		Title:      fmt.Sprintf("Resource Analysis: %s", spec.Name),
		Category:   spec.Category,
		Confidence: 0.7,
	}

	// Placeholder for resource analysis
	result.IsWarn = true
	result.Message = fmt.Sprintf("Resource analyzer %s not implemented yet", spec.Name)
	return result, nil
}

// analyzeCustom handles custom analyzer specifications
func (a *LocalAgent) analyzeCustom(ctx context.Context, bundle *analyzer.SupportBundle, spec analyzer.AnalyzerSpec) (*analyzer.AnalyzerResult, error) {
	result := &analyzer.AnalyzerResult{
		Title:      fmt.Sprintf("Custom Analysis: %s", spec.Name),
		Category:   spec.Category,
		Confidence: 0.5,
	}

	// Placeholder for custom analysis
	result.IsWarn = true
	result.Message = fmt.Sprintf("Custom analyzer %s not implemented yet", spec.Name)
	return result, nil
}
