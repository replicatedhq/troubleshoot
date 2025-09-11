package analyzer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

// Enhanced analysis result with remediation capabilities
type EnhancedAnalysisResult struct {
	Results     []EnhancedAnalyzerResult `json:"results"`
	Remediation []RemediationStep        `json:"remediation"`
	Summary     AnalysisSummary          `json:"summary"`
	Metadata    AnalysisMetadata         `json:"metadata"`
}

// Enhanced individual analyzer result
type EnhancedAnalyzerResult struct {
	// Original fields from AnalyzeResult
	IsPass  bool   `json:"isPass"`
	IsFail  bool   `json:"isFail"`
	IsWarn  bool   `json:"isWarn"`
	Strict  bool   `json:"strict"`
	Title   string `json:"title"`
	Message string `json:"message"`
	URI     string `json:"uri,omitempty"`
	IconKey string `json:"iconKey,omitempty"`
	IconURI string `json:"iconURI,omitempty"`

	// Enhanced fields
	Explanation   string           `json:"explanation,omitempty"`   // Detailed explanation of the issue
	Evidence      []string         `json:"evidence,omitempty"`      // Evidence supporting this result
	RootCause     string           `json:"rootCause,omitempty"`     // Root cause analysis
	Impact        string           `json:"impact,omitempty"`        // Impact assessment (HIGH/MEDIUM/LOW)
	Confidence    float64          `json:"confidence,omitempty"`    // Confidence score 0-1
	Remediation   *RemediationStep `json:"remediation,omitempty"`   // Specific remediation for this issue
	RelatedIssues []string         `json:"relatedIssues,omitempty"` // IDs of related analyzer results
	AgentUsed     string           `json:"agentUsed,omitempty"`     // Which agent produced this result
}

// Remediation step with actionable instructions
type RemediationStep struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Category    string            `json:"category"` // immediate, short-term, long-term
	Priority    int               `json:"priority"` // 1=highest, 10=lowest
	Commands    []string          `json:"commands,omitempty"`
	Manual      []string          `json:"manual,omitempty"`
	Links       []string          `json:"links,omitempty"`
	Validation  *ValidationStep   `json:"validation,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Validation step to verify remediation success
type ValidationStep struct {
	Description string   `json:"description"`
	Commands    []string `json:"commands,omitempty"`
	Expected    string   `json:"expected"`
}

// Summary of overall analysis
type AnalysisSummary struct {
	TotalChecks   int      `json:"totalChecks"`
	PassedChecks  int      `json:"passedChecks"`
	FailedChecks  int      `json:"failedChecks"`
	WarningChecks int      `json:"warningChecks"`
	OverallHealth string   `json:"overallHealth"` // HEALTHY, DEGRADED, CRITICAL
	TopIssues     []string `json:"topIssues,omitempty"`
	Confidence    float64  `json:"confidence"`
}

// Metadata about the analysis process
type AnalysisMetadata struct {
	Timestamp      time.Time         `json:"timestamp"`
	EngineVersion  string            `json:"engineVersion"`
	AgentsUsed     []string          `json:"agentsUsed"`
	BundleInfo     BundleInfo        `json:"bundleInfo"`
	ProcessingTime time.Duration     `json:"processingTime"`
	Configuration  map[string]string `json:"configuration,omitempty"`
}

// Information about the analyzed bundle
type BundleInfo struct {
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	FileCount   int       `json:"fileCount"`
	CollectedAt time.Time `json:"collectedAt,omitempty"`
	Version     string    `json:"version,omitempty"`
}

// Analysis options for configuring the engine
type AnalysisOptions struct {
	// Agent selection and fallback
	PreferredAgents    []string `json:"preferredAgents,omitempty"`
	FallbackAgents     []string `json:"fallbackAgents,omitempty"`
	RequireAllAgents   bool     `json:"requireAllAgents"`
	HybridMode         bool     `json:"hybridMode"`
	AgentFailurePolicy string   `json:"agentFailurePolicy"` // "fail-fast", "continue", "fallback"

	// Analysis configuration
	IncludeRemediation  bool          `json:"includeRemediation"`
	GenerateInsights    bool          `json:"generateInsights"`
	ConfidenceThreshold float64       `json:"confidenceThreshold"`
	MaxProcessingTime   time.Duration `json:"maxProcessingTime"`
	EnableCorrelation   bool          `json:"enableCorrelation"`

	// Data sensitivity and privacy
	DataSensitivityLevel string `json:"dataSensitivityLevel"` // "public", "internal", "confidential", "restricted"
	AllowExternalAPIs    bool   `json:"allowExternalAPIs"`
	RequireLocalOnly     bool   `json:"requireLocalOnly"`

	// Quality and performance
	MinConfidenceScore float64 `json:"minConfidenceScore"`
	MaxCostPerAnalysis float64 `json:"maxCostPerAnalysis"`

	// Custom configuration
	CustomConfig map[string]string `json:"customConfig,omitempty"`
}

// Agent interface for different analysis backends
type Agent interface {
	Name() string
	Analyze(ctx context.Context, bundle *SupportBundle, analyzers []AnalyzerSpec) (*AgentResult, error)
	HealthCheck(ctx context.Context) error
	Capabilities() []string
	Version() string
}

// Result from an agent
type AgentResult struct {
	AgentName      string                   `json:"agentName"`
	Results        []EnhancedAnalyzerResult `json:"results"`
	Insights       []AnalysisInsight        `json:"insights,omitempty"`
	ProcessingTime time.Duration            `json:"processingTime"`
	Metadata       map[string]interface{}   `json:"metadata,omitempty"`
}

// Analysis insight that spans multiple results
type AnalysisInsight struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Type        string   `json:"type"` // correlation, trend, recommendation
	Confidence  float64  `json:"confidence"`
	Evidence    []string `json:"evidence"`
	Impact      string   `json:"impact"`
}

// Support bundle representation for agents
type SupportBundle struct {
	Path      string
	RootDir   string
	Metadata  map[string]interface{}
	GetFile   getCollectedFileContents
	FindFiles getChildCollectedFileContents
}

// Analyzer specification for agents
type AnalyzerSpec struct {
	Name     string                       `json:"name"`
	Type     string                       `json:"type"`
	Config   map[string]interface{}       `json:"config"`
	Original *troubleshootv1beta2.Analyze `json:"-"` // Reference to original analyzer
}

// Main analysis engine interface
type AnalysisEngine interface {
	Analyze(ctx context.Context, bundle *SupportBundle, opts AnalysisOptions) (*EnhancedAnalysisResult, error)
	RegisterAgent(name string, agent Agent) error
	UnregisterAgent(name string) error
	ListAgents() []string
	GetAgent(name string) (Agent, bool)
	SetDefaultAgent(name string) error
}

// Implementation of the analysis engine
type analysisEngine struct {
	agents       map[string]Agent
	defaultAgent string
	mutex        sync.RWMutex
	version      string
}

// Create a new analysis engine
func NewAnalysisEngine() AnalysisEngine {
	return &analysisEngine{
		agents:  make(map[string]Agent),
		version: "1.0.0",
	}
}

// Register an agent with the engine
func (e *analysisEngine) RegisterAgent(name string, agent Agent) error {
	if agent == nil {
		return errors.New("agent cannot be nil")
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.agents[name] = agent

	// Set as default if this is the first agent
	if e.defaultAgent == "" {
		e.defaultAgent = name
	}

	klog.V(2).Infof("Registered analysis agent: %s", name)
	return nil
}

// Unregister an agent
func (e *analysisEngine) UnregisterAgent(name string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if _, exists := e.agents[name]; !exists {
		return fmt.Errorf("agent %s not found", name)
	}

	delete(e.agents, name)

	// Reset default if we removed the default agent
	if e.defaultAgent == name {
		e.defaultAgent = ""
		// Set new default if other agents exist
		for agentName := range e.agents {
			e.defaultAgent = agentName
			break
		}
	}

	klog.V(2).Infof("Unregistered analysis agent: %s", name)
	return nil
}

// List all registered agents
func (e *analysisEngine) ListAgents() []string {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	agents := make([]string, 0, len(e.agents))
	for name := range e.agents {
		agents = append(agents, name)
	}
	return agents
}

// Get a specific agent
func (e *analysisEngine) GetAgent(name string) (Agent, bool) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	agent, exists := e.agents[name]
	return agent, exists
}

// Set the default agent
func (e *analysisEngine) SetDefaultAgent(name string) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if _, exists := e.agents[name]; !exists {
		return fmt.Errorf("agent %s not registered", name)
	}

	e.defaultAgent = name
	return nil
}

// Main analysis method
func (e *analysisEngine) Analyze(ctx context.Context, bundle *SupportBundle, opts AnalysisOptions) (*EnhancedAnalysisResult, error) {
	startTime := time.Now()

	// Validate inputs
	if bundle == nil {
		return nil, errors.New("bundle cannot be nil")
	}

	// Set default options
	opts = e.setDefaultOptions(opts)

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, opts.MaxProcessingTime)
	defer cancel()

	// Intelligent agent selection based on data sensitivity and requirements
	agentPlan, err := e.createAgentExecutionPlan(opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create agent execution plan")
	}

	// Generate analyzer specifications from existing system
	analyzerSpecs, err := e.generateAnalyzerSpecs(bundle)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate analyzer specs")
	}

	// Execute analysis with fallback and hybrid support
	analysisResult, err := e.executeAnalysisPlan(ctx, bundle, analyzerSpecs, agentPlan, opts)
	if err != nil {
		return nil, errors.Wrap(err, "analysis execution failed")
	}

	// Post-process results
	return e.postProcessResults(analysisResult, opts, time.Since(startTime))
}

// Select appropriate agents based on preferences
func (e *analysisEngine) selectAgents(preferred []string) ([]Agent, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	if len(e.agents) == 0 {
		return nil, errors.New("no agents registered")
	}

	var selected []Agent

	// Try preferred agents first
	for _, name := range preferred {
		if agent, exists := e.agents[name]; exists {
			selected = append(selected, agent)
		}
	}

	// If no preferred agents found, use default
	if len(selected) == 0 && e.defaultAgent != "" {
		if agent, exists := e.agents[e.defaultAgent]; exists {
			selected = append(selected, agent)
		}
	}

	// If still no agents, use any available
	if len(selected) == 0 {
		for _, agent := range e.agents {
			selected = append(selected, agent)
			break
		}
	}

	return selected, nil
}

// Generate analyzer specs from the bundle (placeholder for now)
func (e *analysisEngine) generateAnalyzerSpecs(bundle *SupportBundle) ([]AnalyzerSpec, error) {
	// For now, return empty specs - this will be enhanced in later phases
	return []AnalyzerSpec{}, nil
}

// Generate remediation steps from results
func (e *analysisEngine) generateRemediation(results []EnhancedAnalyzerResult, insights []AnalysisInsight) []RemediationStep {
	var remediation []RemediationStep

	// Collect all individual remediations
	for _, result := range results {
		if result.Remediation != nil {
			remediation = append(remediation, *result.Remediation)
		}
	}

	// TODO: Add intelligent remediation ordering and deduplication

	return remediation
}

// Generate analysis summary
func (e *analysisEngine) generateSummary(results []EnhancedAnalyzerResult, insights []AnalysisInsight) AnalysisSummary {
	summary := AnalysisSummary{
		TotalChecks:   len(results),
		OverallHealth: "UNKNOWN",
	}

	var confidenceSum float64
	var confidenceCount int
	var topIssues []string

	for _, result := range results {
		if result.IsPass {
			summary.PassedChecks++
		} else if result.IsFail {
			summary.FailedChecks++
			if result.Impact == "HIGH" || result.Impact == "CRITICAL" {
				topIssues = append(topIssues, result.Title)
			}
		} else if result.IsWarn {
			summary.WarningChecks++
		}

		if result.Confidence > 0 {
			confidenceSum += result.Confidence
			confidenceCount++
		}
	}

	// Calculate overall health
	failureRate := float64(summary.FailedChecks) / float64(summary.TotalChecks)
	if failureRate == 0 {
		summary.OverallHealth = "HEALTHY"
	} else if failureRate < 0.1 {
		summary.OverallHealth = "DEGRADED"
	} else {
		summary.OverallHealth = "CRITICAL"
	}

	// Calculate average confidence
	if confidenceCount > 0 {
		summary.Confidence = confidenceSum / float64(confidenceCount)
	}

	summary.TopIssues = topIssues

	return summary
}

// Extract bundle information
func (e *analysisEngine) extractBundleInfo(bundle *SupportBundle) BundleInfo {
	return BundleInfo{
		Path: bundle.Path,
		// TODO: Extract more detailed bundle information
	}
}

// setDefaultOptions sets sensible defaults for analysis options
func (e *analysisEngine) setDefaultOptions(opts AnalysisOptions) AnalysisOptions {
	if opts.ConfidenceThreshold == 0 {
		opts.ConfidenceThreshold = 0.7
	}
	if opts.MaxProcessingTime == 0 {
		opts.MaxProcessingTime = 5 * time.Minute
	}
	if opts.AgentFailurePolicy == "" {
		opts.AgentFailurePolicy = "fallback"
	}
	if opts.DataSensitivityLevel == "" {
		opts.DataSensitivityLevel = "internal"
	}
	if opts.MinConfidenceScore == 0 {
		opts.MinConfidenceScore = 0.5
	}
	return opts
}

// AgentExecutionPlan defines how agents should be executed
type AgentExecutionPlan struct {
	PrimaryAgents  []Agent `json:"primaryAgents"`
	FallbackAgents []Agent `json:"fallbackAgents"`
	HybridMode     bool    `json:"hybridMode"`
	FailurePolicy  string  `json:"failurePolicy"`
	LocalOnlyMode  bool    `json:"localOnlyMode"`
}

// createAgentExecutionPlan creates an intelligent execution plan based on options
func (e *analysisEngine) createAgentExecutionPlan(opts AnalysisOptions) (*AgentExecutionPlan, error) {
	e.mutex.RLock()
	defer e.mutex.RUnlock()

	plan := &AgentExecutionPlan{
		HybridMode:    opts.HybridMode,
		FailurePolicy: opts.AgentFailurePolicy,
		LocalOnlyMode: opts.RequireLocalOnly,
	}

	// Get all available agents
	availableAgents := e.getAvailableAgents()
	if len(availableAgents) == 0 {
		return nil, errors.New("no agents available")
	}

	// Filter agents based on data sensitivity and requirements
	filteredAgents := e.filterAgentsBySensitivity(availableAgents, opts)

	// Select primary agents
	primaryAgents, err := e.selectPrimaryAgents(filteredAgents, opts.PreferredAgents)
	if err != nil {
		return nil, errors.Wrap(err, "failed to select primary agents")
	}
	plan.PrimaryAgents = primaryAgents

	// Select fallback agents if needed
	if opts.AgentFailurePolicy == "fallback" || opts.HybridMode {
		fallbackAgents := e.selectFallbackAgents(filteredAgents, opts.FallbackAgents, primaryAgents)
		plan.FallbackAgents = fallbackAgents
	}

	if len(plan.PrimaryAgents) == 0 && len(plan.FallbackAgents) == 0 {
		return nil, errors.New("no suitable agents available after filtering")
	}

	return plan, nil
}

// getAvailableAgents returns all registered agents that are healthy
func (e *analysisEngine) getAvailableAgents() []Agent {
	var available []Agent

	for _, agent := range e.agents {
		// Quick health check (with short timeout)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := agent.HealthCheck(ctx); err == nil {
			available = append(available, agent)
		}
		cancel()
	}

	return available
}

// filterAgentsBySensitivity filters agents based on data sensitivity requirements
func (e *analysisEngine) filterAgentsBySensitivity(agents []Agent, opts AnalysisOptions) []Agent {
	var filtered []Agent

	for _, agent := range agents {
		capabilities := agent.Capabilities()

		// Check if agent supports local-only requirement
		if opts.RequireLocalOnly {
			if !contains(capabilities, "local") && !contains(capabilities, "offline") {
				continue
			}
		}

		// Check if external APIs are allowed
		if !opts.AllowExternalAPIs {
			if contains(capabilities, "cloud-llm") || contains(capabilities, "hosted") {
				continue
			}
		}

		// Apply data sensitivity filtering
		switch opts.DataSensitivityLevel {
		case "restricted":
			// Only local agents allowed
			if !contains(capabilities, "local") {
				continue
			}
		case "confidential":
			// Local or privacy-focused agents only
			if !contains(capabilities, "local") && !contains(capabilities, "privacy-focused") {
				continue
			}
		case "internal":
			// Local, privacy-focused, or client-controlled agents
			if contains(capabilities, "third-party-data") {
				continue
			}
		case "public":
			// All agents allowed
		}

		filtered = append(filtered, agent)
	}

	return filtered
}

// selectPrimaryAgents selects the best primary agents based on preferences
func (e *analysisEngine) selectPrimaryAgents(agents []Agent, preferred []string) ([]Agent, error) {
	if len(agents) == 0 {
		return nil, errors.New("no agents available")
	}

	var selected []Agent

	// First, try to use preferred agents
	for _, preferredName := range preferred {
		for _, agent := range agents {
			if agent.Name() == preferredName {
				selected = append(selected, agent)
				break
			}
		}
	}

	// If no preferred agents found, use intelligent selection
	if len(selected) == 0 {
		// Prioritize local agent if available
		for _, agent := range agents {
			if agent.Name() == "local" {
				selected = append(selected, agent)
				break
			}
		}

		// If no local agent, select the first available
		if len(selected) == 0 {
			selected = append(selected, agents[0])
		}
	}

	return selected, nil
}

// selectFallbackAgents selects appropriate fallback agents
func (e *analysisEngine) selectFallbackAgents(agents []Agent, fallbackNames []string, primaryAgents []Agent) []Agent {
	var fallbacks []Agent

	// Create set of primary agent names for exclusion
	primaryNames := make(map[string]bool)
	for _, agent := range primaryAgents {
		primaryNames[agent.Name()] = true
	}

	// Try specified fallback agents first
	for _, fallbackName := range fallbackNames {
		if primaryNames[fallbackName] {
			continue // Skip if already primary
		}
		for _, agent := range agents {
			if agent.Name() == fallbackName {
				fallbacks = append(fallbacks, agent)
				break
			}
		}
	}

	// If no specific fallbacks specified, use intelligent selection
	if len(fallbacks) == 0 {
		for _, agent := range agents {
			if !primaryNames[agent.Name()] {
				fallbacks = append(fallbacks, agent)
				// Limit to 2 fallback agents to avoid excessive processing
				if len(fallbacks) >= 2 {
					break
				}
			}
		}
	}

	return fallbacks
}

// executeAnalysisPlan executes the analysis plan with fallback support
func (e *analysisEngine) executeAnalysisPlan(ctx context.Context, bundle *SupportBundle, analyzers []AnalyzerSpec, plan *AgentExecutionPlan, opts AnalysisOptions) (*AnalysisExecution, error) {
	execution := &AnalysisExecution{
		StartTime:  time.Now(),
		Results:    []EnhancedAnalyzerResult{},
		Insights:   []AnalysisInsight{},
		AgentsUsed: []string{},
		Metadata:   make(map[string]interface{}),
	}

	var primaryResults *AgentResult
	var primaryErr error

	// Execute primary agents
	if plan.HybridMode {
		// In hybrid mode, run all primary agents and combine results
		primaryResults, primaryErr = e.executeAgentsParallel(ctx, bundle, analyzers, plan.PrimaryAgents)
	} else {
		// Sequential execution with early success
		primaryResults, primaryErr = e.executeAgentsSequential(ctx, bundle, analyzers, plan.PrimaryAgents)
	}

	// Handle primary agent results
	if primaryErr == nil && primaryResults != nil {
		execution.Results = append(execution.Results, primaryResults.Results...)
		execution.Insights = append(execution.Insights, primaryResults.Insights...)
		for _, agent := range plan.PrimaryAgents {
			execution.AgentsUsed = append(execution.AgentsUsed, agent.Name())
		}
	}

	// Apply failure policy
	if primaryErr != nil || (primaryResults != nil && len(primaryResults.Results) == 0) {
		switch plan.FailurePolicy {
		case "fail-fast":
			if primaryErr != nil {
				return nil, errors.Wrap(primaryErr, "primary agents failed and fail-fast policy enabled")
			}

		case "fallback":
			if len(plan.FallbackAgents) > 0 {
				fallbackResults, fallbackErr := e.executeAgentsSequential(ctx, bundle, analyzers, plan.FallbackAgents)
				if fallbackErr == nil && fallbackResults != nil {
					execution.Results = append(execution.Results, fallbackResults.Results...)
					execution.Insights = append(execution.Insights, fallbackResults.Insights...)
					for _, agent := range plan.FallbackAgents {
						execution.AgentsUsed = append(execution.AgentsUsed, agent.Name())
					}
					execution.Metadata["fallbackUsed"] = true
				}
			}

		case "continue":
			// Continue with whatever results we have
			execution.Metadata["primaryFailed"] = true
		}
	}

	execution.ProcessingTime = time.Since(execution.StartTime)

	// Ensure we have at least some results
	if len(execution.Results) == 0 {
		return nil, errors.New("no agents produced results")
	}

	return execution, nil
}

// AnalysisExecution represents the execution state and results
type AnalysisExecution struct {
	StartTime      time.Time                `json:"startTime"`
	ProcessingTime time.Duration            `json:"processingTime"`
	Results        []EnhancedAnalyzerResult `json:"results"`
	Insights       []AnalysisInsight        `json:"insights"`
	AgentsUsed     []string                 `json:"agentsUsed"`
	Metadata       map[string]interface{}   `json:"metadata"`
}

// executeAgentsSequential executes agents one by one until success
func (e *analysisEngine) executeAgentsSequential(ctx context.Context, bundle *SupportBundle, analyzers []AnalyzerSpec, agents []Agent) (*AgentResult, error) {
	var lastErr error

	for _, agent := range agents {
		klog.V(2).Infof("Running analysis with agent: %s", agent.Name())

		result, err := agent.Analyze(ctx, bundle, analyzers)
		if err == nil && result != nil && len(result.Results) > 0 {
			return result, nil
		}

		lastErr = err
		if err != nil {
			klog.Errorf("Agent %s failed: %v", agent.Name(), err)
		}
	}

	return nil, errors.Wrap(lastErr, "all agents failed")
}

// executeAgentsParallel executes agents in parallel and combines results
func (e *analysisEngine) executeAgentsParallel(ctx context.Context, bundle *SupportBundle, analyzers []AnalyzerSpec, agents []Agent) (*AgentResult, error) {
	type agentResult struct {
		result *AgentResult
		err    error
		name   string
	}

	resultChan := make(chan agentResult, len(agents))

	// Launch goroutines for each agent
	for _, agent := range agents {
		go func(a Agent) {
			klog.V(2).Infof("Running parallel analysis with agent: %s", a.Name())
			result, err := a.Analyze(ctx, bundle, analyzers)
			resultChan <- agentResult{result: result, err: err, name: a.Name()}
		}(agent)
	}

	// Collect results
	combined := &AgentResult{
		Results:  []EnhancedAnalyzerResult{},
		Insights: []AnalysisInsight{},
		Metadata: make(map[string]interface{}),
	}

	successCount := 0
	for i := 0; i < len(agents); i++ {
		select {
		case res := <-resultChan:
			if res.err != nil {
				klog.Errorf("Agent %s failed in parallel execution: %v", res.name, res.err)
				continue
			}
			if res.result != nil {
				combined.Results = append(combined.Results, res.result.Results...)
				combined.Insights = append(combined.Insights, res.result.Insights...)
				combined.Metadata[res.name] = res.result.Metadata
				successCount++
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if successCount == 0 {
		return nil, errors.New("all parallel agents failed")
	}

	return combined, nil
}

// postProcessResults performs final processing and validation
func (e *analysisEngine) postProcessResults(execution *AnalysisExecution, opts AnalysisOptions, totalDuration time.Duration) (*EnhancedAnalysisResult, error) {
	// Filter results by confidence threshold
	filteredResults := e.filterResultsByConfidence(execution.Results, opts.MinConfidenceScore)

	// Generate remediation steps
	var remediation []RemediationStep
	if opts.IncludeRemediation {
		remediation = e.generateRemediation(filteredResults, execution.Insights)
	}

	// Generate summary
	summary := e.generateSummary(filteredResults, execution.Insights)

	// Create metadata
	metadata := AnalysisMetadata{
		Timestamp:      execution.StartTime,
		EngineVersion:  e.version,
		AgentsUsed:     execution.AgentsUsed,
		ProcessingTime: totalDuration,
		Configuration:  opts.CustomConfig,
	}

	// Add execution metadata
	for key, value := range execution.Metadata {
		if metadata.Configuration == nil {
			metadata.Configuration = make(map[string]string)
		}
		metadata.Configuration[key] = fmt.Sprintf("%v", value)
	}

	result := &EnhancedAnalysisResult{
		Results:     filteredResults,
		Remediation: remediation,
		Summary:     summary,
		Metadata:    metadata,
	}

	klog.V(1).Infof("Analysis completed in %v with %d results from %d agents",
		totalDuration, len(filteredResults), len(execution.AgentsUsed))

	return result, nil
}

// filterResultsByConfidence filters results based on minimum confidence score
func (e *analysisEngine) filterResultsByConfidence(results []EnhancedAnalyzerResult, minConfidence float64) []EnhancedAnalyzerResult {
	if minConfidence <= 0 {
		return results
	}

	var filtered []EnhancedAnalyzerResult
	for _, result := range results {
		// Always include failing results regardless of confidence
		if result.IsFail || result.Confidence >= minConfidence {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
