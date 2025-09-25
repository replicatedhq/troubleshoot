package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// AnalysisEngine orchestrates analysis across multiple agents
type AnalysisEngine interface {
	Analyze(ctx context.Context, bundle *SupportBundle, opts AnalysisOptions) (*AnalysisResult, error)
	GenerateAnalyzers(ctx context.Context, requirements *RequirementSpec) ([]AnalyzerSpec, error)
	RegisterAgent(name string, agent Agent) error
	GetAgent(name string) (Agent, bool)
	ListAgents() []string
	HealthCheck(ctx context.Context) (*EngineHealth, error)
}

// Agent interface for different analysis backends
type Agent interface {
	Name() string
	Analyze(ctx context.Context, data []byte, analyzers []AnalyzerSpec) (*AgentResult, error)
	HealthCheck(ctx context.Context) error
	Capabilities() []string
	IsAvailable() bool
}

// Data structures for analysis results and configuration

type SupportBundle struct {
	Files    map[string][]byte      `json:"files"`
	Metadata *SupportBundleMetadata `json:"metadata"`
}

type SupportBundleMetadata struct {
	CreatedAt   time.Time         `json:"createdAt"`
	Version     string            `json:"version"`
	ClusterInfo *ClusterInfo      `json:"clusterInfo,omitempty"`
	NodeInfo    []NodeInfo        `json:"nodeInfo,omitempty"`
	GeneratedBy string            `json:"generatedBy"`
	Namespace   string            `json:"namespace,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
}

type ClusterInfo struct {
	Version   string `json:"version"`
	Platform  string `json:"platform"`
	NodeCount int    `json:"nodeCount"`
}

type NodeInfo struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	OS           string            `json:"os"`
	Architecture string            `json:"architecture"`
	Labels       map[string]string `json:"labels"`
}

type AnalysisOptions struct {
	Agents             []string                       `json:"agents,omitempty"`
	IncludeRemediation bool                           `json:"includeRemediation"`
	GenerateArtifacts  bool                           `json:"generateArtifacts"`
	CustomAnalyzers    []*troubleshootv1beta2.Analyze `json:"customAnalyzers,omitempty"`
	Timeout            time.Duration                  `json:"timeout,omitempty"`
	Concurrency        int                            `json:"concurrency,omitempty"`
	FilterByNamespace  string                         `json:"filterByNamespace,omitempty"`
	Strict             bool                           `json:"strict"`
}

type AnalysisResult struct {
	Results     []*AnalyzerResult `json:"results"`
	Remediation []RemediationStep `json:"remediation,omitempty"`
	Summary     AnalysisSummary   `json:"summary"`
	Metadata    AnalysisMetadata  `json:"metadata"`
	Errors      []AnalysisError   `json:"errors,omitempty"`
}

type AnalyzerResult struct {
	// Legacy fields from existing AnalyzeResult
	IsPass  bool   `json:"isPass"`
	IsFail  bool   `json:"isFail"`
	IsWarn  bool   `json:"isWarn"`
	Strict  bool   `json:"strict"`
	Title   string `json:"title"`
	Message string `json:"message"`
	URI     string `json:"uri,omitempty"`
	IconKey string `json:"iconKey,omitempty"`
	IconURI string `json:"iconURI,omitempty"`

	// Enhanced fields for agent-based analysis
	AnalyzerType   string                  `json:"analyzerType"`
	AgentName      string                  `json:"agentName"`
	Confidence     float64                 `json:"confidence,omitempty"`
	Category       string                  `json:"category,omitempty"`
	Severity       string                  `json:"severity,omitempty"`
	Remediation    *RemediationStep        `json:"remediation,omitempty"`
	Context        map[string]interface{}  `json:"context,omitempty"`
	InvolvedObject *corev1.ObjectReference `json:"involvedObject,omitempty"`

	// Correlation and insights
	RelatedResults []string `json:"relatedResults,omitempty"`
	Insights       []string `json:"insights,omitempty"`
	Tags           []string `json:"tags,omitempty"`
}

type RemediationStep struct {
	Description   string                 `json:"description"`
	Action        string                 `json:"action,omitempty"`
	Command       string                 `json:"command,omitempty"`
	Documentation string                 `json:"documentation,omitempty"`
	Priority      int                    `json:"priority,omitempty"`
	Category      string                 `json:"category,omitempty"`
	IsAutomatable bool                   `json:"isAutomatable"`
	Context       map[string]interface{} `json:"context,omitempty"`
}

type AnalysisSummary struct {
	TotalAnalyzers int      `json:"totalAnalyzers"`
	PassCount      int      `json:"passCount"`
	WarnCount      int      `json:"warnCount"`
	FailCount      int      `json:"failCount"`
	ErrorCount     int      `json:"errorCount"`
	Confidence     float64  `json:"confidence,omitempty"`
	Duration       string   `json:"duration"`
	AgentsUsed     []string `json:"agentsUsed"`
}

type AnalysisMetadata struct {
	Timestamp       time.Time              `json:"timestamp"`
	EngineVersion   string                 `json:"engineVersion"`
	BundleMetadata  *SupportBundleMetadata `json:"bundleMetadata,omitempty"`
	AnalysisOptions AnalysisOptions        `json:"analysisOptions"`
	Agents          []AgentMetadata        `json:"agents"`
	Correlations    []Correlation          `json:"correlations,omitempty"`
}

type AgentMetadata struct {
	Name         string   `json:"name"`
	Version      string   `json:"version,omitempty"`
	Capabilities []string `json:"capabilities"`
	Duration     string   `json:"duration"`
	ResultCount  int      `json:"resultCount"`
	ErrorCount   int      `json:"errorCount"`
}

type Correlation struct {
	ResultIDs   []string `json:"resultIds"`
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Confidence  float64  `json:"confidence"`
}

type AnalysisError struct {
	Agent       string    `json:"agent,omitempty"`
	Analyzer    string    `json:"analyzer,omitempty"`
	Error       string    `json:"error"`
	Category    string    `json:"category"`
	Timestamp   time.Time `json:"timestamp"`
	Recoverable bool      `json:"recoverable"`
}

type AgentResult struct {
	Results  []*AnalyzerResult   `json:"results"`
	Metadata AgentResultMetadata `json:"metadata"`
	Errors   []string            `json:"errors,omitempty"`
}

type AgentResultMetadata struct {
	Duration      time.Duration `json:"duration"`
	AnalyzerCount int           `json:"analyzerCount"`
	Version       string        `json:"version,omitempty"`
}

type EngineHealth struct {
	Status      string        `json:"status"`
	Agents      []AgentHealth `json:"agents"`
	LastChecked time.Time     `json:"lastChecked"`
}

type AgentHealth struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	Available bool      `json:"available"`
	LastCheck time.Time `json:"lastCheck"`
}

// Requirements-to-analyzers structures
type RequirementSpec struct {
	APIVersion string                 `json:"apiVersion"`
	Kind       string                 `json:"kind"`
	Metadata   RequirementMetadata    `json:"metadata"`
	Spec       RequirementSpecDetails `json:"spec"`
}

type RequirementMetadata struct {
	Name        string            `json:"name"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type RequirementSpecDetails struct {
	Kubernetes KubernetesRequirements `json:"kubernetes,omitempty"`
	Resources  ResourceRequirements   `json:"resources,omitempty"`
	Storage    StorageRequirements    `json:"storage,omitempty"`
	Network    NetworkRequirements    `json:"network,omitempty"`
	Custom     []CustomRequirement    `json:"custom,omitempty"`
}

type KubernetesRequirements struct {
	MinVersion string   `json:"minVersion,omitempty"`
	MaxVersion string   `json:"maxVersion,omitempty"`
	Required   []string `json:"required,omitempty"`
	Forbidden  []string `json:"forbidden,omitempty"`
}

type ResourceRequirements struct {
	CPU    ResourceRequirement `json:"cpu,omitempty"`
	Memory ResourceRequirement `json:"memory,omitempty"`
	Disk   ResourceRequirement `json:"disk,omitempty"`
}

type ResourceRequirement struct {
	Min string `json:"min,omitempty"`
	Max string `json:"max,omitempty"`
}

type StorageRequirements struct {
	Classes     []string `json:"classes,omitempty"`
	MinCapacity string   `json:"minCapacity,omitempty"`
	AccessModes []string `json:"accessModes,omitempty"`
}

type NetworkRequirements struct {
	Ports        []PortRequirement `json:"ports,omitempty"`
	Connectivity []string          `json:"connectivity,omitempty"`
}

type PortRequirement struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Required bool   `json:"required"`
}

type CustomRequirement struct {
	Name      string                 `json:"name"`
	Type      string                 `json:"type"`
	Condition string                 `json:"condition"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

type AnalyzerSpec struct {
	Name     string                 `json:"name"`
	Type     string                 `json:"type"`
	Config   map[string]interface{} `json:"config"`
	Priority int                    `json:"priority,omitempty"`
	Category string                 `json:"category,omitempty"`
}

// DefaultAnalysisEngine implements AnalysisEngine
type DefaultAnalysisEngine struct {
	agents        map[string]Agent
	agentsMutex   sync.RWMutex
	defaultAgents []string
}

// NewAnalysisEngine creates a new analysis engine with default configuration
func NewAnalysisEngine() AnalysisEngine {
	engine := &DefaultAnalysisEngine{
		agents:        make(map[string]Agent),
		defaultAgents: []string{"local"},
	}

	return engine
}

// RegisterAgent registers a new analysis agent
func (e *DefaultAnalysisEngine) RegisterAgent(name string, agent Agent) error {
	if name == "" {
		return errors.New("agent name cannot be empty")
	}
	if agent == nil {
		return errors.New("agent cannot be nil")
	}

	e.agentsMutex.Lock()
	defer e.agentsMutex.Unlock()

	if _, exists := e.agents[name]; exists {
		return errors.Errorf("agent %s already registered", name)
	}

	e.agents[name] = agent
	return nil
}

// GetAgent retrieves an agent by name
func (e *DefaultAnalysisEngine) GetAgent(name string) (Agent, bool) {
	e.agentsMutex.RLock()
	defer e.agentsMutex.RUnlock()

	agent, exists := e.agents[name]
	return agent, exists
}

// ListAgents returns names of all registered agents
func (e *DefaultAnalysisEngine) ListAgents() []string {
	e.agentsMutex.RLock()
	defer e.agentsMutex.RUnlock()

	var names []string
	for name := range e.agents {
		names = append(names, name)
	}
	return names
}

// Analyze performs analysis using configured agents
func (e *DefaultAnalysisEngine) Analyze(ctx context.Context, bundle *SupportBundle, opts AnalysisOptions) (*AnalysisResult, error) {
	startTime := time.Now()

	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "AnalysisEngine.Analyze")
	defer span.End()

	if bundle == nil {
		return nil, errors.New("bundle cannot be nil")
	}

	// Determine which agents to use
	agentNames := opts.Agents
	if len(agentNames) == 0 {
		agentNames = e.defaultAgents
	}

	// Validate agents exist and are available
	availableAgents := make([]Agent, 0, len(agentNames))
	agentMetadata := make([]AgentMetadata, 0, len(agentNames))

	for _, name := range agentNames {
		agent, exists := e.GetAgent(name)
		if !exists {
			span.SetStatus(codes.Error, fmt.Sprintf("agent %s not found", name))
			return nil, errors.Errorf("agent %s not registered", name)
		}

		if !agent.IsAvailable() {
			span.AddEvent(fmt.Sprintf("agent %s not available, skipping", name))
			continue
		}

		availableAgents = append(availableAgents, agent)
	}

	if len(availableAgents) == 0 {
		return nil, errors.New("no available agents found")
	}

	// Prepare bundle data for agents
	bundleData, err := json.Marshal(bundle)
	if err != nil {
		span.SetStatus(codes.Error, "failed to marshal bundle")
		return nil, errors.Wrap(err, "failed to marshal bundle data")
	}

	// Generate analyzer specs from requirements (if any)
	var analyzers []AnalyzerSpec
	var conversionFailures []AnalyzerResult
	if len(opts.CustomAnalyzers) > 0 {
		// Convert existing analyzers to specs for agents
		for i, analyzer := range opts.CustomAnalyzers {
			spec, err := e.convertAnalyzerToSpec(analyzer)
			if err != nil {
				klog.Errorf("Failed to convert custom analyzer %d to spec: %v", i, err)
				klog.Warningf("Creating failure result for analyzer %d. Supported types: ClusterVersion, DeploymentStatus", i)
				klog.Warningf("To fix: Check your analyzer configuration and ensure it uses supported types")

				// Create a failure result instead of skipping
				failureResult := AnalyzerResult{
					IsFail:     true,
					Title:      fmt.Sprintf("Custom Analyzer %d - Conversion Failed", i),
					Message:    fmt.Sprintf("Failed to convert analyzer to supported format: %v", err),
					Category:   "configuration",
					Confidence: 1.0,
					AgentName:  "analyzer-converter",
				}
				conversionFailures = append(conversionFailures, failureResult)
				continue
			}
			analyzers = append(analyzers, spec)
		}
	}

	// Run analysis across agents
	results := &AnalysisResult{
		Results: make([]*AnalyzerResult, 0),
		Summary: AnalysisSummary{
			AgentsUsed: make([]string, 0, len(availableAgents)),
		},
		Metadata: AnalysisMetadata{
			Timestamp:       time.Now(),
			EngineVersion:   "1.0.0",
			BundleMetadata:  bundle.Metadata,
			AnalysisOptions: opts,
			Agents:          agentMetadata,
		},
		Errors: make([]AnalysisError, 0),
	}

	// Execute analysis on each agent
	for _, agent := range availableAgents {
		agentStart := time.Now()

		agentResult, err := e.runAgentAnalysis(ctx, agent, bundleData, analyzers)
		agentDuration := time.Since(agentStart)

		metadata := AgentMetadata{
			Name:         agent.Name(),
			Capabilities: agent.Capabilities(),
			Duration:     agentDuration.String(),
		}

		if err != nil {
			metadata.ErrorCount = 1
			results.Errors = append(results.Errors, AnalysisError{
				Agent:       agent.Name(),
				Error:       err.Error(),
				Category:    "agent_execution",
				Timestamp:   time.Now(),
				Recoverable: true,
			})
		} else {
			metadata.ResultCount = len(agentResult.Results)
			results.Results = append(results.Results, agentResult.Results...)
		}

		results.Metadata.Agents = append(results.Metadata.Agents, metadata)
		results.Summary.AgentsUsed = append(results.Summary.AgentsUsed, agent.Name())
	}

	// Add conversion failures to results (analyzers that failed to convert)
	for _, failure := range conversionFailures {
		results.Results = append(results.Results, &failure)
	}

	// Calculate summary statistics
	e.calculateSummary(results)
	results.Summary.Duration = time.Since(startTime).String()

	// Generate remediation if requested
	if opts.IncludeRemediation {
		e.generateRemediation(ctx, results)
	}

	// Apply correlations and insights
	e.applyCorrelations(results)

	span.SetAttributes(
		attribute.Int("total_results", len(results.Results)),
		attribute.Int("agents_used", len(availableAgents)),
		attribute.String("duration", results.Summary.Duration),
	)

	return results, nil
}

// runAgentAnalysis executes analysis on a specific agent
func (e *DefaultAnalysisEngine) runAgentAnalysis(ctx context.Context, agent Agent, bundleData []byte, analyzers []AnalyzerSpec) (*AgentResult, error) {
	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, fmt.Sprintf("Agent.%s.Analyze", agent.Name()))
	defer span.End()

	result, err := agent.Analyze(ctx, bundleData, analyzers)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, errors.Wrapf(err, "agent %s analysis failed", agent.Name())
	}

	// Add agent name to all results
	for _, r := range result.Results {
		r.AgentName = agent.Name()
	}

	return result, nil
}

// calculateSummary computes summary statistics for analysis results
func (e *DefaultAnalysisEngine) calculateSummary(results *AnalysisResult) {
	summary := &results.Summary
	summary.TotalAnalyzers = len(results.Results)

	var confidenceSum float64
	confidenceCount := 0

	for _, result := range results.Results {
		if result.IsPass {
			summary.PassCount++
		} else if result.IsWarn {
			summary.WarnCount++
		} else if result.IsFail {
			summary.FailCount++
		}

		if result.Confidence > 0 {
			confidenceSum += result.Confidence
			confidenceCount++
		}
	}

	summary.ErrorCount = len(results.Errors)

	if confidenceCount > 0 {
		summary.Confidence = confidenceSum / float64(confidenceCount)
	}
}

// generateRemediation creates remediation suggestions
func (e *DefaultAnalysisEngine) generateRemediation(ctx context.Context, results *AnalysisResult) {
	var remediationSteps []RemediationStep

	for _, result := range results.Results {
		if result.IsFail && result.Remediation != nil {
			remediationSteps = append(remediationSteps, *result.Remediation)
		}
	}

	// Sort by priority (higher priority first)
	// TODO: Implement sorting logic

	results.Remediation = remediationSteps
}

// applyCorrelations identifies relationships between analysis results
func (e *DefaultAnalysisEngine) applyCorrelations(results *AnalysisResult) {
	// TODO: Implement correlation logic
	// This could identify patterns like:
	// - Multiple pod failures in same namespace
	// - Resource constraint patterns
	// - Network connectivity issues
}

// convertAnalyzerToSpec converts legacy analyzer to new spec format
func (e *DefaultAnalysisEngine) convertAnalyzerToSpec(analyzer *troubleshootv1beta2.Analyze) (AnalyzerSpec, error) {
	if analyzer == nil {
		return AnalyzerSpec{}, errors.New("analyzer cannot be nil")
	}

	spec := AnalyzerSpec{
		Config: make(map[string]interface{}),
	}

	// Determine analyzer type and convert configuration - Supporting ALL 33+ analyzer types
	switch {
	// ✅ Cluster-level analyzers
	case analyzer.ClusterVersion != nil:
		spec.Name = "cluster-version"
		spec.Type = "cluster"
		spec.Config["analyzer"] = analyzer.ClusterVersion
	case analyzer.ContainerRuntime != nil:
		spec.Name = "container-runtime"
		spec.Type = "cluster"
		spec.Config["analyzer"] = analyzer.ContainerRuntime
	case analyzer.Distribution != nil:
		spec.Name = "distribution"
		spec.Type = "cluster"
		spec.Config["analyzer"] = analyzer.Distribution
	case analyzer.NodeResources != nil:
		spec.Name = "node-resources"
		spec.Type = "cluster"
		spec.Config["analyzer"] = analyzer.NodeResources
		spec.Config["filePath"] = "cluster-resources/nodes.json" // Enhanced method expects this
	case analyzer.NodeMetrics != nil:
		spec.Name = "node-metrics"
		spec.Type = "cluster"
		spec.Config["analyzer"] = analyzer.NodeMetrics

	// ✅ Workload analyzers
	case analyzer.DeploymentStatus != nil:
		spec.Name = "deployment-status"
		spec.Type = "workload"
		spec.Config["analyzer"] = analyzer.DeploymentStatus
		// Set default filePath based on namespace if available
		if analyzer.DeploymentStatus.Namespace != "" {
			spec.Config["filePath"] = fmt.Sprintf("cluster-resources/deployments/%s.json", analyzer.DeploymentStatus.Namespace)
		} else {
			spec.Config["filePath"] = "cluster-resources/deployments.json"
		}
	case analyzer.StatefulsetStatus != nil:
		spec.Name = "statefulset-status"
		spec.Type = "workload"
		spec.Config["analyzer"] = analyzer.StatefulsetStatus
	case analyzer.JobStatus != nil:
		spec.Name = "job-status"
		spec.Type = "workload"
		spec.Config["analyzer"] = analyzer.JobStatus
	case analyzer.ReplicaSetStatus != nil:
		spec.Name = "replicaset-status"
		spec.Type = "workload"
		spec.Config["analyzer"] = analyzer.ReplicaSetStatus
	case analyzer.ClusterPodStatuses != nil:
		spec.Name = "cluster-pod-statuses"
		spec.Type = "workload"
		spec.Config["analyzer"] = analyzer.ClusterPodStatuses
	case analyzer.ClusterContainerStatuses != nil:
		spec.Name = "cluster-container-statuses"
		spec.Type = "workload"
		spec.Config["analyzer"] = analyzer.ClusterContainerStatuses

	// ✅ Configuration analyzers
	case analyzer.Secret != nil:
		spec.Name = "secret"
		spec.Type = "configuration"
		spec.Config["analyzer"] = analyzer.Secret
	case analyzer.ConfigMap != nil:
		spec.Name = "configmap"
		spec.Type = "configuration"
		spec.Config["analyzer"] = analyzer.ConfigMap
	case analyzer.ImagePullSecret != nil:
		spec.Name = "image-pull-secret"
		spec.Type = "configuration"
		spec.Config["analyzer"] = analyzer.ImagePullSecret
	case analyzer.StorageClass != nil:
		spec.Name = "storage-class"
		spec.Type = "configuration"
		spec.Config["analyzer"] = analyzer.StorageClass
	case analyzer.CustomResourceDefinition != nil:
		spec.Name = "crd"
		spec.Type = "configuration"
		spec.Config["analyzer"] = analyzer.CustomResourceDefinition
	case analyzer.ClusterResource != nil:
		spec.Name = "cluster-resource"
		spec.Type = "configuration"
		spec.Config["analyzer"] = analyzer.ClusterResource

	// ✅ Network analyzers
	case analyzer.Ingress != nil:
		spec.Name = "ingress"
		spec.Type = "network"
		spec.Config["analyzer"] = analyzer.Ingress
	case analyzer.HTTP != nil:
		spec.Name = "http"
		spec.Type = "network"
		spec.Config["analyzer"] = analyzer.HTTP

	// ✅ Data analysis
	case analyzer.TextAnalyze != nil:
		spec.Name = "text-analyze"
		spec.Type = "data"
		spec.Config["analyzer"] = analyzer.TextAnalyze
		// Enhanced method will auto-detect log files from TextAnalyze configuration
	case analyzer.YamlCompare != nil:
		spec.Name = "yaml-compare"
		spec.Type = "data"
		spec.Config["analyzer"] = analyzer.YamlCompare
	case analyzer.JsonCompare != nil:
		spec.Name = "json-compare"
		spec.Type = "data"
		spec.Config["analyzer"] = analyzer.JsonCompare

	// ✅ Database analyzers
	case analyzer.Postgres != nil:
		spec.Name = "postgres"
		spec.Type = "database"
		spec.Config["analyzer"] = analyzer.Postgres
	case analyzer.Mysql != nil:
		spec.Name = "mysql"
		spec.Type = "database"
		spec.Config["analyzer"] = analyzer.Mysql
	case analyzer.Mssql != nil:
		spec.Name = "mssql"
		spec.Type = "database"
		spec.Config["analyzer"] = analyzer.Mssql
	case analyzer.Redis != nil:
		spec.Name = "redis"
		spec.Type = "database"
		spec.Config["analyzer"] = analyzer.Redis

	// ✅ Storage analyzers
	case analyzer.CephStatus != nil:
		spec.Name = "ceph-status"
		spec.Type = "storage"
		spec.Config["analyzer"] = analyzer.CephStatus
	case analyzer.Longhorn != nil:
		spec.Name = "longhorn"
		spec.Type = "storage"
		spec.Config["analyzer"] = analyzer.Longhorn
	case analyzer.Velero != nil:
		spec.Name = "velero"
		spec.Type = "storage"
		spec.Config["analyzer"] = analyzer.Velero

	// ✅ Infrastructure analyzers
	case analyzer.RegistryImages != nil:
		spec.Name = "registry-images"
		spec.Type = "infrastructure"
		spec.Config["analyzer"] = analyzer.RegistryImages
	case analyzer.WeaveReport != nil:
		spec.Name = "weave-report"
		spec.Type = "infrastructure"
		spec.Config["analyzer"] = analyzer.WeaveReport
	case analyzer.Goldpinger != nil:
		spec.Name = "goldpinger"
		spec.Type = "infrastructure"
		spec.Config["analyzer"] = analyzer.Goldpinger
	case analyzer.Sysctl != nil:
		spec.Name = "sysctl"
		spec.Type = "infrastructure"
		spec.Config["analyzer"] = analyzer.Sysctl
	case analyzer.Certificates != nil:
		spec.Name = "certificates"
		spec.Type = "infrastructure"
		spec.Config["analyzer"] = analyzer.Certificates
	case analyzer.Event != nil:
		spec.Name = "event"
		spec.Type = "infrastructure"
		spec.Config["analyzer"] = analyzer.Event

	default:
		return spec, errors.New("unknown analyzer type - this should not happen as all known types are now supported")
	}

	return spec, nil
}

// GenerateAnalyzers creates analyzers from requirement specifications
func (e *DefaultAnalysisEngine) GenerateAnalyzers(ctx context.Context, requirements *RequirementSpec) ([]AnalyzerSpec, error) {
	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "AnalysisEngine.GenerateAnalyzers")
	defer span.End()

	if requirements == nil {
		return nil, errors.New("requirements cannot be nil")
	}

	var specs []AnalyzerSpec

	// Generate Kubernetes version analyzers
	if requirements.Spec.Kubernetes.MinVersion != "" || requirements.Spec.Kubernetes.MaxVersion != "" {
		specs = append(specs, AnalyzerSpec{
			Name:     "kubernetes-version-check",
			Type:     "cluster",
			Category: "kubernetes",
			Priority: 10,
			Config: map[string]interface{}{
				"minVersion": requirements.Spec.Kubernetes.MinVersion,
				"maxVersion": requirements.Spec.Kubernetes.MaxVersion,
			},
		})
	}

	// Generate resource requirement analyzers
	if requirements.Spec.Resources.CPU.Min != "" || requirements.Spec.Resources.Memory.Min != "" {
		specs = append(specs, AnalyzerSpec{
			Name:     "resource-requirements-check",
			Type:     "resources",
			Category: "capacity",
			Priority: 8,
			Config: map[string]interface{}{
				"cpu":    requirements.Spec.Resources.CPU,
				"memory": requirements.Spec.Resources.Memory,
				"disk":   requirements.Spec.Resources.Disk,
			},
		})
	}

	// Generate storage analyzers
	if len(requirements.Spec.Storage.Classes) > 0 {
		specs = append(specs, AnalyzerSpec{
			Name:     "storage-class-check",
			Type:     "storage",
			Category: "storage",
			Priority: 6,
			Config: map[string]interface{}{
				"classes":     requirements.Spec.Storage.Classes,
				"minCapacity": requirements.Spec.Storage.MinCapacity,
				"accessModes": requirements.Spec.Storage.AccessModes,
			},
		})
	}

	// Generate network analyzers
	if len(requirements.Spec.Network.Ports) > 0 {
		specs = append(specs, AnalyzerSpec{
			Name:     "network-connectivity-check",
			Type:     "network",
			Category: "networking",
			Priority: 7,
			Config: map[string]interface{}{
				"ports":        requirements.Spec.Network.Ports,
				"connectivity": requirements.Spec.Network.Connectivity,
			},
		})
	}

	// Generate custom analyzers
	for _, custom := range requirements.Spec.Custom {
		specs = append(specs, AnalyzerSpec{
			Name:     custom.Name,
			Type:     custom.Type,
			Category: "custom",
			Priority: 5,
			Config: map[string]interface{}{
				"condition": custom.Condition,
				"context":   custom.Context,
			},
		})
	}

	span.SetAttributes(
		attribute.Int("generated_analyzers", len(specs)),
		attribute.String("requirements_name", requirements.Metadata.Name),
	)

	return specs, nil
}

// HealthCheck performs health check on the engine and all agents
func (e *DefaultAnalysisEngine) HealthCheck(ctx context.Context) (*EngineHealth, error) {
	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "AnalysisEngine.HealthCheck")
	defer span.End()

	health := &EngineHealth{
		Status:      "healthy",
		Agents:      make([]AgentHealth, 0),
		LastChecked: time.Now(),
	}

	e.agentsMutex.RLock()
	agents := make(map[string]Agent, len(e.agents))
	for name, agent := range e.agents {
		agents[name] = agent
	}
	e.agentsMutex.RUnlock()

	hasUnhealthyAgent := false

	for name, agent := range agents {
		agentHealth := AgentHealth{
			Name:      name,
			Available: agent.IsAvailable(),
			LastCheck: time.Now(),
		}

		err := agent.HealthCheck(ctx)
		if err != nil {
			agentHealth.Status = "unhealthy"
			agentHealth.Error = err.Error()
			hasUnhealthyAgent = true
		} else {
			agentHealth.Status = "healthy"
		}

		health.Agents = append(health.Agents, agentHealth)
	}

	if hasUnhealthyAgent {
		health.Status = "degraded"
	}

	return health, nil
}
