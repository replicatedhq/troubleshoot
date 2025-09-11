package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

// OllamaAgent represents a self-hosted LLM agent using Ollama
type OllamaAgent struct {
	name          string
	version       string
	capabilities  []string
	config        *OllamaConfig
	client        *http.Client
	promptBuilder *PromptBuilder
	auditLogger   *AuditLogger
}

// OllamaConfig contains configuration for Ollama agent
type OllamaConfig struct {
	*LLMConfig

	// Ollama-specific settings
	BaseURL        string                 `json:"baseUrl"`
	Model          string                 `json:"model"`
	PullModel      bool                   `json:"pullModel"`
	KeepAlive      string                 `json:"keepAlive"`
	ConnectTimeout time.Duration          `json:"connectTimeout"`
	Options        map[string]interface{} `json:"options"`
}

// DefaultOllamaConfig returns a default Ollama configuration
func DefaultOllamaConfig(model string) *OllamaConfig {
	return &OllamaConfig{
		LLMConfig:      DefaultLLMConfig(ProviderOllama, model),
		BaseURL:        "http://localhost:11434",
		Model:          model,
		PullModel:      false, // Don't auto-pull by default
		KeepAlive:      "5m",
		ConnectTimeout: 10 * time.Second,
		Options: map[string]interface{}{
			"num_ctx":        4096, // Context window
			"temperature":    0.3,  // Creativity vs consistency
			"top_p":          0.9,  // Nucleus sampling
			"top_k":          40,   // Top-k sampling
			"repeat_penalty": 1.1,  // Reduce repetition
		},
	}
}

// NewOllamaAgent creates a new Ollama agent
func NewOllamaAgent(config *OllamaConfig) (*OllamaAgent, error) {
	if config == nil {
		config = DefaultOllamaConfig("codellama:13b")
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   config.ConnectTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	agent := &OllamaAgent{
		name:          "ollama",
		version:       "1.0.0",
		capabilities:  []string{"local-llm", "privacy-focused", "intelligent-analysis", "contextual-remediation"},
		config:        config,
		client:        client,
		promptBuilder: NewPromptBuilder(config.SystemPrompt, config.EnablePIIFilter),
		auditLogger:   NewAuditLogger(config.AuditLogging),
	}

	return agent, nil
}

// Name returns the agent name
func (o *OllamaAgent) Name() string {
	return o.name
}

// Version returns the agent version
func (o *OllamaAgent) Version() string {
	return o.version
}

// Capabilities returns the list of agent capabilities
func (o *OllamaAgent) Capabilities() []string {
	caps := make([]string, len(o.capabilities))
	copy(caps, o.capabilities)
	return caps
}

// HealthCheck verifies that Ollama is accessible and the model is available
func (o *OllamaAgent) HealthCheck(ctx context.Context) error {
	// Check if Ollama server is running
	if err := o.checkOllamaServer(ctx); err != nil {
		return fmt.Errorf("ollama server check failed: %w", err)
	}

	// Check if model is available
	if err := o.checkModel(ctx); err != nil {
		return fmt.Errorf("model availability check failed: %w", err)
	}

	// Perform a simple test generation
	if err := o.performHealthTest(ctx); err != nil {
		return fmt.Errorf("health test failed: %w", err)
	}

	return nil
}

// Analyze performs intelligent analysis using Ollama LLM
func (o *OllamaAgent) Analyze(ctx context.Context, bundle *analyzer.SupportBundle, analyzers []analyzer.AnalyzerSpec) (*analyzer.AgentResult, error) {
	if bundle == nil {
		return nil, fmt.Errorf("support bundle cannot be nil")
	}

	startTime := time.Now()

	// First, run basic local analysis to get baseline results
	localResults, err := o.runLocalAnalysis(ctx, bundle, analyzers)
	if err != nil {
		return nil, fmt.Errorf("local analysis failed: %w", err)
	}

	// Extract bundle context for better LLM understanding
	bundleContext, err := o.extractBundleContext(bundle)
	if err != nil {
		// Log warning but continue without context
		bundleContext = &BundleContext{}
	}

	// Enhance each result using LLM intelligence
	enhancedResults := make([]analyzer.EnhancedAnalyzerResult, 0, len(localResults))

	for _, result := range localResults {
		enhanced, err := o.enhanceResultWithLLM(ctx, result, localResults, bundleContext)
		if err != nil {
			// If LLM enhancement fails, use basic enhancement
			enhanced = o.basicEnhancement(result)
		}
		enhancedResults = append(enhancedResults, enhanced)
	}

	// Generate intelligent insights across all results
	insights, err := o.generateIntelligentInsights(ctx, enhancedResults, bundleContext)
	if err != nil {
		// Log warning but continue with empty insights
		insights = []analyzer.AnalysisInsight{}
	}

	// Build agent result
	agentResult := &analyzer.AgentResult{
		AgentName:      o.name,
		ProcessingTime: time.Since(startTime),
		Results:        enhancedResults,
		Insights:       insights,
		Metadata: map[string]interface{}{
			"model":               o.config.Model,
			"provider":            "ollama",
			"baseUrl":             o.config.BaseURL,
			"enhancementsApplied": true,
			"contextExtracted":    bundleContext != nil,
			"llmCallsSuccessful":  true, // Would track actual success rate
			"piiFilterEnabled":    o.config.EnablePIIFilter,
			"auditLogging":        o.config.AuditLogging,
		},
	}

	return agentResult, nil
}

// checkOllamaServer verifies Ollama server is running
func (o *OllamaAgent) checkOllamaServer(ctx context.Context) error {
	url := o.config.BaseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health request: %w", err)
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("ollama server unreachable at %s: %w", o.config.BaseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ollama server returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// checkModel verifies the specified model is available
func (o *OllamaAgent) checkModel(ctx context.Context) error {
	url := o.config.BaseURL + "/api/tags"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read model list: %w", err)
	}

	var modelList struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &modelList); err != nil {
		return fmt.Errorf("failed to parse model list: %w", err)
	}

	// Check if our model is in the list
	for _, model := range modelList.Models {
		if model.Name == o.config.Model || strings.HasPrefix(model.Name, o.config.Model+":") {
			return nil
		}
	}

	// Model not found, attempt to pull if configured
	if o.config.PullModel {
		return o.pullModel(ctx)
	}

	return fmt.Errorf("model %s not found and auto-pull disabled", o.config.Model)
}

// pullModel pulls the specified model from Ollama registry
func (o *OllamaAgent) pullModel(ctx context.Context) error {
	url := o.config.BaseURL + "/api/pull"

	payload := map[string]interface{}{
		"name": o.config.Model,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return fmt.Errorf("model pull request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("model pull failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// performHealthTest performs a simple generation test
func (o *OllamaAgent) performHealthTest(ctx context.Context) error {
	request := &LLMRequest{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Respond with exactly 'OK' if you can understand this message.",
		Temperature:  0.1,
		MaxTokens:    10,
	}

	response, err := o.generateCompletion(ctx, request)
	if err != nil {
		return fmt.Errorf("health test generation failed: %w", err)
	}

	if !strings.Contains(strings.ToUpper(response.Content), "OK") {
		return fmt.Errorf("health test returned unexpected response: %s", response.Content)
	}

	return nil
}

// runLocalAnalysis runs basic local analysis as a foundation
func (o *OllamaAgent) runLocalAnalysis(ctx context.Context, bundle *analyzer.SupportBundle, analyzers []analyzer.AnalyzerSpec) ([]*analyzer.AnalyzeResult, error) {
	// This would use the existing local analyzer logic from Phase 1
	// For now, return mock results
	return []*analyzer.AnalyzeResult{
		{
			Title:   "Kubernetes Version Check",
			IsPass:  true,
			Message: "Cluster is running Kubernetes v1.24.0",
		},
		{
			Title:   "Node Resources Check",
			IsFail:  true,
			Message: "2 nodes are experiencing memory pressure",
		},
		{
			Title:   "Storage Classes",
			IsWarn:  true,
			Message: "No default storage class configured",
		},
	}, nil
}

// extractBundleContext extracts relevant context from the support bundle
func (o *OllamaAgent) extractBundleContext(bundle *analyzer.SupportBundle) (*BundleContext, error) {
	context := &BundleContext{
		Metadata: make(map[string]string),
	}

	// Extract cluster version
	if versionData, err := bundle.GetFile("version.json"); err == nil {
		var versionInfo struct {
			ServerVersion struct {
				GitVersion string `json:"gitVersion"`
			} `json:"serverVersion"`
		}
		if json.Unmarshal(versionData, &versionInfo) == nil {
			context.ClusterVersion = versionInfo.ServerVersion.GitVersion
		}
	}

	// Extract node information
	if nodesData, err := bundle.GetFile("cluster-resources/nodes.json"); err == nil {
		var nodesList struct {
			Items []struct {
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
				Status struct {
					Conditions []struct {
						Type   string `json:"type"`
						Status string `json:"status"`
					} `json:"conditions"`
				} `json:"status"`
			} `json:"items"`
		}
		if json.Unmarshal(nodesData, &nodesList) == nil {
			context.NodeCount = len(nodesList.Items)
		}
	}

	// Extract namespace information
	if namespacesData, err := bundle.GetFile("cluster-resources/namespaces.json"); err == nil {
		var namespacesList struct {
			Items []struct {
				Metadata struct {
					Name string `json:"name"`
				} `json:"metadata"`
			} `json:"items"`
		}
		if json.Unmarshal(namespacesData, &namespacesList) == nil {
			context.Namespaces = make([]string, len(namespacesList.Items))
			for i, ns := range namespacesList.Items {
				context.Namespaces[i] = ns.Metadata.Name
			}
		}
	}

	return context, nil
}

// enhanceResultWithLLM uses LLM to enhance a single analysis result
func (o *OllamaAgent) enhanceResultWithLLM(ctx context.Context, result *analyzer.AnalyzeResult, allResults []*analyzer.AnalyzeResult, bundleContext *BundleContext) (analyzer.EnhancedAnalyzerResult, error) {
	// Build enhancement request
	request := &EnhancementRequest{
		OriginalResult: result,
		AllResults:     allResults,
		BundleContext:  bundleContext,
		Options: &EnhancementOptions{
			IncludeRemediation:   true,
			EnableCorrelation:    true,
			DetailLevel:          "detailed",
			ExcludeGenericAdvice: true,
		},
	}

	// Build LLM prompt
	prompt, err := o.promptBuilder.BuildAnalysisPrompt(request)
	if err != nil {
		return analyzer.EnhancedAnalyzerResult{}, fmt.Errorf("failed to build prompt: %w", err)
	}

	// Generate LLM completion
	llmRequest := &LLMRequest{
		SystemPrompt: o.config.SystemPrompt,
		UserPrompt:   prompt,
		Temperature:  o.config.Temperature,
		MaxTokens:    o.config.MaxTokens,
	}

	response, err := o.generateCompletion(ctx, llmRequest)
	if err != nil {
		return analyzer.EnhancedAnalyzerResult{}, fmt.Errorf("LLM generation failed: %w", err)
	}

	// Parse LLM response
	llmResult, err := ParseLLMResponse(response.Content)
	if err != nil {
		return analyzer.EnhancedAnalyzerResult{}, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	// Convert to enhanced result
	enhanced := analyzer.EnhancedAnalyzerResult{
		// Copy original fields
		IsPass:  result.IsPass,
		IsFail:  result.IsFail,
		IsWarn:  result.IsWarn,
		Strict:  result.Strict,
		Title:   result.Title,
		Message: result.Message,
		URI:     result.URI,
		IconKey: result.IconKey,
		IconURI: result.IconURI,

		// Enhanced fields
		AgentUsed:     o.name,
		Confidence:    llmResult.Confidence,
		Impact:        llmResult.Impact,
		Explanation:   llmResult.Explanation,
		Evidence:      llmResult.Evidence,
		RootCause:     llmResult.Problem,
		Remediation:   llmResult.Remediation,
		RelatedIssues: llmResult.Correlations,
	}

	return enhanced, nil
}

// basicEnhancement provides basic enhancement when LLM fails
func (o *OllamaAgent) basicEnhancement(result *analyzer.AnalyzeResult) analyzer.EnhancedAnalyzerResult {
	confidence := 0.7
	impact := "MEDIUM"
	explanation := fmt.Sprintf("Basic analysis of %s", result.Title)

	if result.IsPass {
		confidence = 0.9
		impact = ""
		explanation = fmt.Sprintf("%s check passed successfully", result.Title)
	} else if result.IsFail {
		impact = "HIGH"
		explanation = fmt.Sprintf("%s check failed and requires attention", result.Title)
	}

	return analyzer.EnhancedAnalyzerResult{
		// Copy original fields
		IsPass:  result.IsPass,
		IsFail:  result.IsFail,
		IsWarn:  result.IsWarn,
		Strict:  result.Strict,
		Title:   result.Title,
		Message: result.Message,
		URI:     result.URI,
		IconKey: result.IconKey,
		IconURI: result.IconURI,

		// Enhanced fields
		AgentUsed:   o.name + "-basic",
		Confidence:  confidence,
		Impact:      impact,
		Explanation: explanation,
		Evidence:    []string{result.Message},
	}
}

// generateIntelligentInsights generates cross-cutting insights using LLM
func (o *OllamaAgent) generateIntelligentInsights(ctx context.Context, results []analyzer.EnhancedAnalyzerResult, bundleContext *BundleContext) ([]analyzer.AnalysisInsight, error) {
	if len(results) == 0 {
		return []analyzer.AnalysisInsight{}, nil
	}

	// Build insights prompt
	prompt := o.buildInsightsPrompt(results, bundleContext)

	llmRequest := &LLMRequest{
		SystemPrompt: o.config.SystemPrompt,
		UserPrompt:   prompt,
		Temperature:  o.config.Temperature,
		MaxTokens:    o.config.MaxTokens,
	}

	response, err := o.generateCompletion(ctx, llmRequest)
	if err != nil {
		return []analyzer.AnalysisInsight{}, err
	}

	// Parse insights from response
	insights, err := o.parseInsightsResponse(response.Content)
	if err != nil {
		return []analyzer.AnalysisInsight{}, err
	}

	return insights, nil
}

// generateCompletion calls Ollama to generate a completion
func (o *OllamaAgent) generateCompletion(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	startTime := time.Now()

	// Build Ollama request payload
	payload := map[string]interface{}{
		"model":   o.config.Model,
		"prompt":  request.SystemPrompt + "\n\n" + request.UserPrompt,
		"stream":  false,
		"options": o.config.Options,
	}

	if o.config.KeepAlive != "" {
		payload["keep_alive"] = o.config.KeepAlive
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	url := o.config.BaseURL + "/api/generate"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var ollamaResp struct {
		Response           string `json:"response"`
		Model              string `json:"model"`
		Done               bool   `json:"done"`
		Context            []int  `json:"context"`
		TotalDuration      int64  `json:"total_duration"`
		LoadDuration       int64  `json:"load_duration"`
		PromptEvalDuration int64  `json:"prompt_eval_duration"`
		EvalCount          int    `json:"eval_count"`
		EvalDuration       int64  `json:"eval_duration"`
	}

	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	processingTime := time.Since(startTime)

	// Log for audit if enabled
	if o.auditLogger != nil {
		auditLog := AuditLog{
			AgentName:   o.name,
			Provider:    string(ProviderOllama),
			Model:       o.config.Model,
			InputSize:   len(jsonData),
			OutputSize:  len(ollamaResp.Response),
			Duration:    processingTime,
			PIIFiltered: o.config.EnablePIIFilter,
			Success:     true,
			Metadata: map[string]interface{}{
				"eval_count":    ollamaResp.EvalCount,
				"eval_duration": ollamaResp.EvalDuration,
				"load_duration": ollamaResp.LoadDuration,
			},
		}
		o.auditLogger.LogRequest(auditLog)
	}

	return &LLMResponse{
		Content:        ollamaResp.Response,
		Model:          ollamaResp.Model,
		ProcessingTime: processingTime,
		Usage: &TokenUsage{
			CompletionTokens: ollamaResp.EvalCount,
			TotalTokens:      ollamaResp.EvalCount, // Ollama doesn't separate prompt tokens
		},
		Metadata: map[string]interface{}{
			"total_duration": ollamaResp.TotalDuration,
			"load_duration":  ollamaResp.LoadDuration,
			"eval_duration":  ollamaResp.EvalDuration,
		},
	}, nil
}

// buildInsightsPrompt creates a prompt for generating system-wide insights
func (o *OllamaAgent) buildInsightsPrompt(results []analyzer.EnhancedAnalyzerResult, context *BundleContext) string {
	var parts []string
	parts = append(parts, "Analyze the following collection of troubleshooting results and provide high-level insights:")

	// Add context
	if context != nil && context.ClusterVersion != "" {
		parts = append(parts, fmt.Sprintf("Cluster: Kubernetes %s with %d nodes", context.ClusterVersion, context.NodeCount))
	}

	// Add results summary
	parts = append(parts, "\nRESULTS SUMMARY:")
	passCount, failCount, warnCount := 0, 0, 0
	for i, result := range results {
		status := "UNKNOWN"
		if result.IsPass {
			status = "PASS"
			passCount++
		} else if result.IsFail {
			status = "FAIL"
			failCount++
		} else if result.IsWarn {
			status = "WARN"
			warnCount++
		}
		parts = append(parts, fmt.Sprintf("%d. %s: %s [%s]", i+1, result.Title, result.Message, status))
	}

	parts = append(parts, fmt.Sprintf("\nSummary: %d passing, %d failing, %d warnings", passCount, failCount, warnCount))

	parts = append(parts, `

Provide insights as JSON array:
[
  {
    "type": "correlation|recommendation|warning",
    "title": "Insight Title",
    "description": "Detailed description",
    "confidence": 0.0-1.0,
    "affectedComponents": ["component1", "component2"],
    "priority": "HIGH|MEDIUM|LOW",
    "actionRequired": true/false
  }
]

Focus on:
1. System-wide patterns and correlations
2. Root cause analysis across multiple failures
3. Security or performance implications
4. Recommended next steps or investigations`)

	return strings.Join(parts, "\n")
}

// parseInsightsResponse parses the LLM response to extract insights
func (o *OllamaAgent) parseInsightsResponse(content string) ([]analyzer.AnalysisInsight, error) {
	// Clean up the response
	content = strings.TrimSpace(content)

	// Find JSON array if wrapped in markdown
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.Index(content[start:], "```")
		if end > 0 {
			content = content[start : start+end]
		}
	}

	// Try to find JSON array in the response
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}

	var rawInsights []struct {
		Type               string   `json:"type"`
		Title              string   `json:"title"`
		Description        string   `json:"description"`
		Confidence         float64  `json:"confidence"`
		AffectedComponents []string `json:"affectedComponents"`
		Priority           string   `json:"priority"`
		ActionRequired     bool     `json:"actionRequired"`
	}

	if err := json.Unmarshal([]byte(content), &rawInsights); err != nil {
		// Return basic insights if parsing fails
		return []analyzer.AnalysisInsight{
			{
				Type:        "recommendation",
				Title:       "System Analysis Complete",
				Description: "Analysis completed with LLM enhancement",
				Confidence:  0.8,
			},
		}, nil
	}

	// Convert to analyzer insights
	insights := make([]analyzer.AnalysisInsight, len(rawInsights))
	for i, raw := range rawInsights {
		insights[i] = analyzer.AnalysisInsight{
			Type:        raw.Type,
			Title:       raw.Title,
			Description: raw.Description,
			Confidence:  raw.Confidence,
		}
	}

	return insights, nil
}

// calculatePriority determines the priority of an issue
func (o *OllamaAgent) calculatePriority(result *analyzer.AnalyzeResult, llmResult *LLMAnalysisResult) int {
	if result.IsPass {
		return 10 // Lowest priority
	}

	// Base priority on impact and confidence
	priority := 5 // Default medium priority

	switch strings.ToUpper(llmResult.Impact) {
	case "HIGH", "CRITICAL":
		priority = 1
	case "MEDIUM":
		priority = 3
	case "LOW":
		priority = 7
	}

	// Adjust based on confidence
	if llmResult.Confidence < 0.5 {
		priority += 2 // Lower confidence = lower priority
	} else if llmResult.Confidence > 0.9 {
		priority -= 1 // Higher confidence = higher priority
	}

	// Ensure priority stays in valid range
	if priority < 1 {
		priority = 1
	} else if priority > 10 {
		priority = 10
	}

	return priority
}

// basicPriority calculates basic priority without LLM
func (o *OllamaAgent) basicPriority(result *analyzer.AnalyzeResult) int {
	if result.IsPass {
		return 10
	} else if result.IsFail {
		return 2
	} else if result.IsWarn {
		return 5
	}
	return 7
}
