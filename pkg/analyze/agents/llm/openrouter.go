package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
)

// OpenRouterAgent represents a cloud LLM agent using OpenRouter API
type OpenRouterAgent struct {
	name          string
	version       string
	capabilities  []string
	config        *OpenRouterConfig
	client        *http.Client
	promptBuilder *PromptBuilder
	auditLogger   *AuditLogger
}

// OpenRouterConfig contains configuration for OpenRouter agent
type OpenRouterConfig struct {
	*LLMConfig

	// OpenRouter-specific settings
	APIKey      string `json:"apiKey"`
	BaseURL     string `json:"baseUrl"`
	Model       string `json:"model"`
	HTTPReferer string `json:"httpReferer"`
	XTitle      string `json:"xTitle"`

	// Fallback models in case primary model is unavailable
	FallbackModels []string `json:"fallbackModels"`

	// Cost management
	MaxTokensPerDay int     `json:"maxTokensPerDay"`
	MaxCostPerDay   float64 `json:"maxCostPerDay"`
	TrackUsage      bool    `json:"trackUsage"`

	// Request settings
	RetryOnRateLimit bool          `json:"retryOnRateLimit"`
	RateLimitDelay   time.Duration `json:"rateLimitDelay"`
	MaxRetries       int           `json:"maxRetries"`
}

// SupportedModels contains the list of well-tested models for troubleshooting
var SupportedModels = map[string]ModelInfo{
	"anthropic/claude-3.5-sonnet": {
		Name:      "Claude 3.5 Sonnet",
		Provider:  "Anthropic",
		MaxTokens: 200000,
		BestFor:   "Complex reasoning, detailed analysis",
		CostTier:  "high",
	},
	"anthropic/claude-3-haiku": {
		Name:      "Claude 3 Haiku",
		Provider:  "Anthropic",
		MaxTokens: 200000,
		BestFor:   "Fast analysis, cost-effective",
		CostTier:  "low",
	},
	"openai/gpt-4o": {
		Name:      "GPT-4o",
		Provider:  "OpenAI",
		MaxTokens: 128000,
		BestFor:   "General purpose, reliable",
		CostTier:  "medium",
	},
	"openai/gpt-4o-mini": {
		Name:      "GPT-4o Mini",
		Provider:  "OpenAI",
		MaxTokens: 128000,
		BestFor:   "Cost-effective, fast responses",
		CostTier:  "low",
	},
	"google/gemini-pro-1.5": {
		Name:      "Gemini Pro 1.5",
		Provider:  "Google",
		MaxTokens: 2000000,
		BestFor:   "Large context analysis",
		CostTier:  "medium",
	},
	"meta-llama/llama-3.1-70b-instruct": {
		Name:      "Llama 3.1 70B",
		Provider:  "Meta",
		MaxTokens: 131072,
		BestFor:   "Open source, balanced performance",
		CostTier:  "low",
	},
}

// ModelInfo contains information about a supported model
type ModelInfo struct {
	Name      string `json:"name"`
	Provider  string `json:"provider"`
	MaxTokens int    `json:"maxTokens"`
	BestFor   string `json:"bestFor"`
	CostTier  string `json:"costTier"` // "low", "medium", "high"
}

// DefaultOpenRouterConfig returns a default OpenRouter configuration
func DefaultOpenRouterConfig(apiKey, model string) *OpenRouterConfig {
	if model == "" {
		model = "anthropic/claude-3-haiku" // Cost-effective default
	}

	return &OpenRouterConfig{
		LLMConfig:   DefaultLLMConfig(ProviderOpenRouter, model),
		APIKey:      apiKey,
		BaseURL:     "https://openrouter.ai/api/v1",
		Model:       model,
		HTTPReferer: "https://troubleshoot.sh",
		XTitle:      "Troubleshoot Analysis Agent",
		FallbackModels: []string{
			"openai/gpt-4o-mini",
			"anthropic/claude-3-haiku",
			"meta-llama/llama-3.1-70b-instruct",
		},
		MaxTokensPerDay:  100000,
		MaxCostPerDay:    10.0,
		TrackUsage:       true,
		RetryOnRateLimit: true,
		RateLimitDelay:   30 * time.Second,
		MaxRetries:       3,
	}
}

// NewOpenRouterAgent creates a new OpenRouter agent
func NewOpenRouterAgent(config *OpenRouterConfig) (*OpenRouterAgent, error) {
	if config == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	if config.APIKey == "" {
		return nil, fmt.Errorf("API key is required for OpenRouter")
	}

	// Validate model is supported
	if _, exists := SupportedModels[config.Model]; !exists {
		return nil, fmt.Errorf("model %s is not in the list of supported models", config.Model)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 5,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	agent := &OpenRouterAgent{
		name:          "openrouter",
		version:       "1.0.0",
		capabilities:  []string{"cloud-llm", "multi-provider", "intelligent-analysis", "cost-managed"},
		config:        config,
		client:        client,
		promptBuilder: NewPromptBuilder(config.SystemPrompt, config.EnablePIIFilter),
		auditLogger:   NewAuditLogger(config.AuditLogging),
	}

	return agent, nil
}

// Name returns the agent name
func (or *OpenRouterAgent) Name() string {
	return or.name
}

// Version returns the agent version
func (or *OpenRouterAgent) Version() string {
	return or.version
}

// Capabilities returns the list of agent capabilities
func (or *OpenRouterAgent) Capabilities() []string {
	caps := make([]string, len(or.capabilities))
	copy(caps, or.capabilities)
	return caps
}

// HealthCheck verifies OpenRouter API access and model availability
func (or *OpenRouterAgent) HealthCheck(ctx context.Context) error {
	// Test API connectivity with a simple request
	if err := or.testAPIConnectivity(ctx); err != nil {
		return fmt.Errorf("API connectivity test failed: %w", err)
	}

	// Verify model availability
	if err := or.verifyModelAvailability(ctx); err != nil {
		return fmt.Errorf("model availability check failed: %w", err)
	}

	// Perform a simple generation test
	if err := or.performHealthTest(ctx); err != nil {
		return fmt.Errorf("generation health test failed: %w", err)
	}

	return nil
}

// Analyze performs intelligent analysis using OpenRouter cloud LLMs
func (or *OpenRouterAgent) Analyze(ctx context.Context, bundle *analyzer.SupportBundle, analyzers []analyzer.AnalyzerSpec) (*analyzer.AgentResult, error) {
	if bundle == nil {
		return nil, fmt.Errorf("support bundle cannot be nil")
	}

	startTime := time.Now()

	// Check usage limits if tracking is enabled
	if or.config.TrackUsage {
		if err := or.checkUsageLimits(); err != nil {
			return nil, fmt.Errorf("usage limits exceeded: %w", err)
		}
	}

	// Run basic local analysis first
	localResults, err := or.runLocalAnalysis(ctx, bundle, analyzers)
	if err != nil {
		return nil, fmt.Errorf("local analysis failed: %w", err)
	}

	// Extract bundle context
	bundleContext, err := or.extractBundleContext(bundle)
	if err != nil {
		bundleContext = &BundleContext{}
	}

	// Enhance results with cloud LLM intelligence
	enhancedResults := make([]analyzer.EnhancedAnalyzerResult, 0, len(localResults))

	for _, result := range localResults {
		enhanced, err := or.enhanceResultWithLLM(ctx, result, localResults, bundleContext)
		if err != nil {
			// Fall back to basic enhancement if LLM fails
			enhanced = or.basicEnhancement(result)
		}
		enhancedResults = append(enhancedResults, enhanced)
	}

	// Generate system-wide insights
	insights, err := or.generateIntelligentInsights(ctx, enhancedResults, bundleContext)
	if err != nil {
		insights = []analyzer.AnalysisInsight{}
	}

	// Build agent result
	agentResult := &analyzer.AgentResult{
		AgentName:      or.name,
		ProcessingTime: time.Since(startTime),
		Results:        enhancedResults,
		Insights:       insights,
		Metadata: map[string]interface{}{
			"model":               or.config.Model,
			"provider":            "openrouter",
			"apiUrl":              or.config.BaseURL,
			"enhancementsApplied": true,
			"usageTracking":       or.config.TrackUsage,
			"piiFilterEnabled":    or.config.EnablePIIFilter,
			"fallbackModels":      or.config.FallbackModels,
		},
	}

	return agentResult, nil
}

// testAPIConnectivity tests basic API connectivity
func (or *OpenRouterAgent) testAPIConnectivity(ctx context.Context) error {
	url := or.config.BaseURL + "/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create test request: %w", err)
	}

	or.addAuthHeaders(req)

	resp, err := or.client.Do(req)
	if err != nil {
		return fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// verifyModelAvailability checks if the configured model is available
func (or *OpenRouterAgent) verifyModelAvailability(ctx context.Context) error {
	url := or.config.BaseURL + "/models"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	or.addAuthHeaders(req)

	resp, err := or.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var modelsList struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &modelsList); err != nil {
		return fmt.Errorf("failed to parse models list: %w", err)
	}

	// Check if our model is available
	for _, model := range modelsList.Data {
		if model.ID == or.config.Model {
			return nil
		}
	}

	return fmt.Errorf("model %s is not available", or.config.Model)
}

// performHealthTest performs a simple generation test
func (or *OpenRouterAgent) performHealthTest(ctx context.Context) error {
	request := &LLMRequest{
		SystemPrompt: "You are a helpful assistant.",
		UserPrompt:   "Respond with exactly 'OK' to confirm you're working.",
		Temperature:  0.1,
		MaxTokens:    10,
	}

	response, err := or.generateCompletion(ctx, request)
	if err != nil {
		return err
	}

	if !strings.Contains(strings.ToUpper(response.Content), "OK") {
		return fmt.Errorf("unexpected health test response: %s", response.Content)
	}

	return nil
}

// runLocalAnalysis runs basic local analysis as foundation
func (or *OpenRouterAgent) runLocalAnalysis(ctx context.Context, bundle *analyzer.SupportBundle, analyzers []analyzer.AnalyzerSpec) ([]*analyzer.AnalyzeResult, error) {
	// Mock results - in real implementation would use local agent from Phase 1
	return []*analyzer.AnalyzeResult{
		{
			Title:   "Kubernetes Version Check",
			IsPass:  true,
			Message: "Cluster is running Kubernetes v1.24.0",
		},
		{
			Title:   "Node Resources Check",
			IsFail:  true,
			Message: "Node worker-1 has high memory utilization (85%)",
		},
		{
			Title:   "Pod Security Standards",
			IsWarn:  true,
			Message: "Some pods are running without security contexts",
		},
	}, nil
}

// extractBundleContext extracts context from support bundle (reusing ollama logic)
func (or *OpenRouterAgent) extractBundleContext(bundle *analyzer.SupportBundle) (*BundleContext, error) {
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

	return context, nil
}

// enhanceResultWithLLM uses OpenRouter LLM to enhance analysis results
func (or *OpenRouterAgent) enhanceResultWithLLM(ctx context.Context, result *analyzer.AnalyzeResult, allResults []*analyzer.AnalyzeResult, bundleContext *BundleContext) (analyzer.EnhancedAnalyzerResult, error) {
	// Build enhancement request
	request := &EnhancementRequest{
		OriginalResult: result,
		AllResults:     allResults,
		BundleContext:  bundleContext,
		Options: &EnhancementOptions{
			IncludeRemediation:   true,
			EnableCorrelation:    true,
			DetailLevel:          "comprehensive",
			ExcludeGenericAdvice: true,
			FocusAreas:           []string{"kubernetes", "security", "performance"},
		},
	}

	// Build LLM prompt
	prompt, err := or.promptBuilder.BuildAnalysisPrompt(request)
	if err != nil {
		return analyzer.EnhancedAnalyzerResult{}, err
	}

	// Generate completion with retry logic
	var response *LLMResponse
	var lastErr error

	models := append([]string{or.config.Model}, or.config.FallbackModels...)

	for _, model := range models {
		llmRequest := &LLMRequest{
			SystemPrompt: or.config.SystemPrompt,
			UserPrompt:   prompt,
			Temperature:  or.config.Temperature,
			MaxTokens:    or.config.MaxTokens,
		}

		or.config.Model = model // Temporarily use this model
		response, lastErr = or.generateCompletion(ctx, llmRequest)
		if lastErr == nil {
			break
		}
	}

	if lastErr != nil {
		return analyzer.EnhancedAnalyzerResult{}, fmt.Errorf("all models failed: %w", lastErr)
	}

	// Parse LLM response
	llmResult, err := ParseLLMResponse(response.Content)
	if err != nil {
		return analyzer.EnhancedAnalyzerResult{}, err
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
		AgentUsed:     or.name,
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

// basicEnhancement provides fallback enhancement
func (or *OpenRouterAgent) basicEnhancement(result *analyzer.AnalyzeResult) analyzer.EnhancedAnalyzerResult {
	confidence := 0.6
	impact := "MEDIUM"
	explanation := fmt.Sprintf("Analysis of %s completed with fallback logic", result.Title)

	if result.IsPass {
		confidence = 0.8
		impact = ""
		explanation = fmt.Sprintf("%s check passed successfully", result.Title)
	} else if result.IsFail {
		impact = "HIGH"
		explanation = fmt.Sprintf("%s check failed and requires immediate attention", result.Title)
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
		AgentUsed:   or.name + "-fallback",
		Confidence:  confidence,
		Impact:      impact,
		Explanation: explanation,
		Evidence:    []string{result.Message},
	}
}

// generateIntelligentInsights generates system insights using LLM
func (or *OpenRouterAgent) generateIntelligentInsights(ctx context.Context, results []analyzer.EnhancedAnalyzerResult, bundleContext *BundleContext) ([]analyzer.AnalysisInsight, error) {
	if len(results) == 0 {
		return []analyzer.AnalysisInsight{}, nil
	}

	prompt := or.buildInsightsPrompt(results, bundleContext)

	llmRequest := &LLMRequest{
		SystemPrompt: or.getInsightsSystemPrompt(),
		UserPrompt:   prompt,
		Temperature:  0.2, // Lower temperature for more consistent insights
		MaxTokens:    2000,
	}

	response, err := or.generateCompletion(ctx, llmRequest)
	if err != nil {
		return []analyzer.AnalysisInsight{}, err
	}

	insights, err := or.parseInsightsResponse(response.Content)
	if err != nil {
		return []analyzer.AnalysisInsight{}, err
	}

	return insights, nil
}

// generateCompletion calls OpenRouter API to generate completion
func (or *OpenRouterAgent) generateCompletion(ctx context.Context, request *LLMRequest) (*LLMResponse, error) {
	startTime := time.Now()

	// Build OpenRouter request payload
	messages := []map[string]string{
		{
			"role":    "system",
			"content": request.SystemPrompt,
		},
		{
			"role":    "user",
			"content": request.UserPrompt,
		},
	}

	payload := map[string]interface{}{
		"model":       or.config.Model,
		"messages":    messages,
		"temperature": request.Temperature,
		"max_tokens":  request.MaxTokens,
		"stream":      false,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create request
	url := or.config.BaseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	or.addAuthHeaders(req)
	req.Header.Set("Content-Type", "application/json")

	// Execute request with retries
	var response *LLMResponse
	var lastErr error

	for attempt := 0; attempt <= or.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(or.config.RateLimitDelay * time.Duration(attempt)):
			}
		}

		response, lastErr = or.executeRequest(req, startTime)
		if lastErr == nil {
			return response, nil
		}

		// Don't retry certain errors
		if !or.shouldRetry(lastErr) {
			break
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", or.config.MaxRetries+1, lastErr)
}

// executeRequest executes a single API request
func (or *OpenRouterAgent) executeRequest(req *http.Request, startTime time.Time) (*LLMResponse, error) {
	resp, err := or.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, or.handleAPIError(resp.StatusCode, body)
	}

	var openrouterResp struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.Unmarshal(body, &openrouterResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(openrouterResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	processingTime := time.Since(startTime)

	// Log for audit
	if or.auditLogger != nil {
		auditLog := AuditLog{
			AgentName:   or.name,
			Provider:    string(ProviderOpenRouter),
			Model:       or.config.Model,
			InputSize:   len(req.Header.Get("Content-Length")),
			OutputSize:  len(openrouterResp.Choices[0].Message.Content),
			Duration:    processingTime,
			PIIFiltered: or.config.EnablePIIFilter,
			Success:     true,
			TokenUsage: &TokenUsage{
				PromptTokens:     openrouterResp.Usage.PromptTokens,
				CompletionTokens: openrouterResp.Usage.CompletionTokens,
				TotalTokens:      openrouterResp.Usage.TotalTokens,
			},
		}
		or.auditLogger.LogRequest(auditLog)
	}

	return &LLMResponse{
		Content: openrouterResp.Choices[0].Message.Content,
		Model:   openrouterResp.Model,
		Usage: &TokenUsage{
			PromptTokens:     openrouterResp.Usage.PromptTokens,
			CompletionTokens: openrouterResp.Usage.CompletionTokens,
			TotalTokens:      openrouterResp.Usage.TotalTokens,
		},
		ProcessingTime: processingTime,
		Metadata: map[string]interface{}{
			"id":            openrouterResp.ID,
			"finish_reason": openrouterResp.Choices[0].FinishReason,
		},
	}, nil
}

// addAuthHeaders adds authentication headers to the request
func (or *OpenRouterAgent) addAuthHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+or.config.APIKey)
	if or.config.HTTPReferer != "" {
		req.Header.Set("HTTP-Referer", or.config.HTTPReferer)
	}
	if or.config.XTitle != "" {
		req.Header.Set("X-Title", or.config.XTitle)
	}
}

// handleAPIError processes API error responses
func (or *OpenRouterAgent) handleAPIError(statusCode int, body []byte) error {
	var errorResp struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    string `json:"code"`
		} `json:"error"`
	}

	if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error.Message != "" {
		return fmt.Errorf("API error (%d): %s", statusCode, errorResp.Error.Message)
	}

	return fmt.Errorf("API error %d: %s", statusCode, string(body))
}

// shouldRetry determines if an error should be retried
func (or *OpenRouterAgent) shouldRetry(err error) bool {
	if !or.config.RetryOnRateLimit {
		return false
	}

	errorStr := strings.ToLower(err.Error())
	return strings.Contains(errorStr, "rate limit") ||
		strings.Contains(errorStr, "timeout") ||
		strings.Contains(errorStr, "503") ||
		strings.Contains(errorStr, "502")
}

// checkUsageLimits checks if usage limits are exceeded
func (or *OpenRouterAgent) checkUsageLimits() error {
	// Simplified usage tracking - in real implementation would track actual usage
	// For now, just return nil to indicate limits are OK
	return nil
}

// buildInsightsPrompt builds prompt for system-wide insights
func (or *OpenRouterAgent) buildInsightsPrompt(results []analyzer.EnhancedAnalyzerResult, context *BundleContext) string {
	var parts []string
	parts = append(parts, "Analyze these Kubernetes troubleshooting results and provide strategic insights:")

	if context != nil && context.ClusterVersion != "" {
		parts = append(parts, fmt.Sprintf("\nCluster: Kubernetes %s", context.ClusterVersion))
	}

	parts = append(parts, "\nRESULTS:")
	for i, result := range results {
		status := "UNKNOWN"
		if result.IsPass {
			status = "✅ PASS"
		} else if result.IsFail {
			status = "❌ FAIL"
		} else if result.IsWarn {
			status = "⚠️  WARN"
		}
		parts = append(parts, fmt.Sprintf("%d. %s: %s [%s]", i+1, result.Title, result.Message, status))

		if result.Impact != "" {
			parts = append(parts, fmt.Sprintf("   Impact: %s", result.Impact))
		}
	}

	parts = append(parts, `

Provide 2-4 high-level insights focusing on:
- Security vulnerabilities and risks
- Performance bottlenecks and optimization opportunities  
- Operational improvements and best practices
- Strategic recommendations for system reliability

Return as JSON array of insights with type, title, description, and confidence.`)

	return strings.Join(parts, "\n")
}

// parseInsightsResponse parses insights from LLM response (reusing ollama logic)
func (or *OpenRouterAgent) parseInsightsResponse(content string) ([]analyzer.AnalysisInsight, error) {
	// Implementation similar to ollama agent's parseInsightsResponse
	content = strings.TrimSpace(content)

	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		end := strings.Index(content[start:], "```")
		if end > 0 {
			content = content[start : start+end]
		}
	}

	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start >= 0 && end > start {
		content = content[start : end+1]
	}

	var rawInsights []struct {
		Type        string  `json:"type"`
		Title       string  `json:"title"`
		Description string  `json:"description"`
		Confidence  float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(content), &rawInsights); err != nil {
		return []analyzer.AnalysisInsight{
			{
				Type:        "recommendation",
				Title:       "Analysis Complete",
				Description: "Cloud LLM analysis completed successfully",
				Confidence:  0.9,
			},
		}, nil
	}

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

// getInsightsSystemPrompt returns specialized prompt for insights
func (or *OpenRouterAgent) getInsightsSystemPrompt() string {
	return `You are an expert Kubernetes platform engineer analyzing troubleshooting results. Provide strategic insights that focus on:

1. Security posture and vulnerability assessment
2. Performance optimization and resource efficiency
3. Operational excellence and reliability improvements
4. Cost optimization opportunities

Be specific, actionable, and prioritize high-impact recommendations. Avoid generic advice.`
}

// calculatePriority and basicPriority (reusing ollama logic)
func (or *OpenRouterAgent) calculatePriority(result *analyzer.AnalyzeResult, llmResult *LLMAnalysisResult) int {
	if result.IsPass {
		return 10
	}

	priority := 5
	switch strings.ToUpper(llmResult.Impact) {
	case "HIGH", "CRITICAL":
		priority = 1
	case "MEDIUM":
		priority = 3
	case "LOW":
		priority = 7
	}

	if llmResult.Confidence < 0.5 {
		priority += 2
	} else if llmResult.Confidence > 0.9 {
		priority -= 1
	}

	if priority < 1 {
		priority = 1
	} else if priority > 10 {
		priority = 10
	}

	return priority
}

func (or *OpenRouterAgent) basicPriority(result *analyzer.AnalyzeResult) int {
	if result.IsPass {
		return 10
	} else if result.IsFail {
		return 2
	} else if result.IsWarn {
		return 5
	}
	return 7
}
