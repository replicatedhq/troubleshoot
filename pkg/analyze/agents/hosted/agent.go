package hosted

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"golang.org/x/time/rate"
)

// HostedAgent represents a cloud-hosted analysis agent with REST API integration
type HostedAgent struct {
	name         string
	version      string
	capabilities []string
	config       *HostedAgentConfig
	client       *http.Client
	limiter      *rate.Limiter
	mu           sync.RWMutex
	authToken    string
	authExpiry   time.Time
}

// HostedAgentConfig contains configuration for the hosted agent
type HostedAgentConfig struct {
	// API Configuration
	BaseURL    string            `json:"baseUrl"`
	APIVersion string            `json:"apiVersion"`
	Endpoints  map[string]string `json:"endpoints"`

	// Authentication
	AuthType     string `json:"authType"` // "bearer", "api-key", "oauth2"
	APIKey       string `json:"apiKey"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	TokenURL     string `json:"tokenUrl"`

	// Rate Limiting
	RequestsPerSecond float64 `json:"requestsPerSecond"`
	BurstSize         int     `json:"burstSize"`

	// Request Configuration
	MaxRetries     int           `json:"maxRetries"`
	RetryDelay     time.Duration `json:"retryDelay"`
	RequestTimeout time.Duration `json:"requestTimeout"`

	// Data Configuration
	EnableCompression bool              `json:"enableCompression"`
	MaxBundleSize     int64             `json:"maxBundleSize"`
	CustomHeaders     map[string]string `json:"customHeaders"`
}

// DefaultHostedAgentConfig returns a default configuration for hosted agents
func DefaultHostedAgentConfig() *HostedAgentConfig {
	return &HostedAgentConfig{
		BaseURL:           "https://api.troubleshoot.sh",
		APIVersion:        "v1",
		AuthType:          "api-key",
		APIKey:            "",
		RequestsPerSecond: 10.0,
		BurstSize:         5,
		MaxRetries:        3,
		RetryDelay:        1 * time.Second,
		RequestTimeout:    30 * time.Second,
		EnableCompression: true,
		MaxBundleSize:     100 * 1024 * 1024, // 100MB
		Endpoints: map[string]string{
			"analyze":      "/analyze",
			"health":       "/health",
			"auth":         "/auth/token",
			"capabilities": "/capabilities",
		},
		CustomHeaders: make(map[string]string),
	}
}

// NewHostedAgent creates a new hosted agent with the provided configuration
func NewHostedAgent(config *HostedAgentConfig) (*HostedAgent, error) {
	if config == nil {
		config = DefaultHostedAgentConfig()
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: config.RequestTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Create rate limiter
	limiter := rate.NewLimiter(rate.Limit(config.RequestsPerSecond), config.BurstSize)

	agent := &HostedAgent{
		name:         "hosted",
		version:      "1.0.0",
		capabilities: []string{"cloud-analysis", "advanced-ml", "correlation", "remediation"},
		config:       config,
		client:       client,
		limiter:      limiter,
	}

	return agent, nil
}

// Name returns the agent name
func (h *HostedAgent) Name() string {
	return h.name
}

// Version returns the agent version
func (h *HostedAgent) Version() string {
	return h.version
}

// Capabilities returns the list of agent capabilities
func (h *HostedAgent) Capabilities() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()

	caps := make([]string, len(h.capabilities))
	copy(caps, h.capabilities)
	return caps
}

// HealthCheck performs a health check against the hosted service
func (h *HostedAgent) HealthCheck(ctx context.Context) error {
	// Apply rate limiting
	if err := h.limiter.Wait(ctx); err != nil {
		return fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Build health check URL
	url := h.config.BaseURL + "/" + h.config.APIVersion + h.config.Endpoints["health"]

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Add authentication
	if err := h.addAuthentication(req); err != nil {
		return fmt.Errorf("failed to add authentication: %w", err)
	}

	// Add custom headers
	h.addCustomHeaders(req)

	// Execute request
	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Analyze performs analysis using the hosted service
func (h *HostedAgent) Analyze(ctx context.Context, bundle *analyzer.SupportBundle, analyzers []analyzer.AnalyzerSpec) (*analyzer.AgentResult, error) {
	if bundle == nil {
		return nil, fmt.Errorf("support bundle cannot be nil")
	}

	startTime := time.Now()

	// Apply rate limiting
	if err := h.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit exceeded: %w", err)
	}

	// Prepare request payload
	payload, err := h.prepareAnalysisPayload(bundle, analyzers)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare analysis payload: %w", err)
	}

	// Execute analysis request with retries
	result, err := h.executeAnalysisWithRetries(ctx, payload)
	if err != nil {
		return nil, fmt.Errorf("analysis request failed: %w", err)
	}

	// Calculate processing time
	processingTime := time.Since(startTime)

	// Build agent result
	agentResult := &analyzer.AgentResult{
		AgentName:      h.name,
		ProcessingTime: processingTime,
		Results:        result.Results,
		Insights:       result.Insights,
		Metadata: map[string]interface{}{
			"apiVersion":   h.config.APIVersion,
			"endpoint":     h.config.BaseURL,
			"requestTime":  startTime.Format(time.RFC3339),
			"responseTime": time.Now().Format(time.RFC3339),
			"rateLimited":  false, // Would be true if we hit limits
			"retryCount":   0,     // Would track actual retries
			"payloadSize":  len(payload),
		},
	}

	return agentResult, nil
}

// AnalysisRequest represents the request payload for hosted analysis
type AnalysisRequest struct {
	BundleData []byte                  `json:"bundleData"`
	Analyzers  []analyzer.AnalyzerSpec `json:"analyzers"`
	Options    AnalysisOptions         `json:"options"`
	Metadata   map[string]interface{}  `json:"metadata"`
}

// AnalysisResponse represents the response from hosted analysis
type AnalysisResponse struct {
	Results  []analyzer.EnhancedAnalyzerResult `json:"results"`
	Insights []analyzer.AnalysisInsight        `json:"insights"`
	Summary  analyzer.AnalysisSummary          `json:"summary"`
	Metadata map[string]interface{}            `json:"metadata"`
	Error    string                            `json:"error,omitempty"`
}

// AnalysisOptions contains options for the hosted analysis
type AnalysisOptions struct {
	EnableCompression  bool     `json:"enableCompression"`
	IncludeRemediation bool     `json:"includeRemediation"`
	CorrelationLevel   string   `json:"correlationLevel"` // "basic", "advanced", "deep"
	ModelVersion       string   `json:"modelVersion"`
	Features           []string `json:"features"`
}

// prepareAnalysisPayload creates the request payload for hosted analysis
func (h *HostedAgent) prepareAnalysisPayload(bundle *analyzer.SupportBundle, analyzers []analyzer.AnalyzerSpec) ([]byte, error) {
	// Extract bundle data (simplified - in real implementation would compress and serialize)
	bundleData := []byte(fmt.Sprintf("bundle-path:%s", bundle.Path))

	// Create analysis request
	request := AnalysisRequest{
		BundleData: bundleData,
		Analyzers:  analyzers,
		Options: AnalysisOptions{
			EnableCompression:  h.config.EnableCompression,
			IncludeRemediation: true,
			CorrelationLevel:   "advanced",
			ModelVersion:       "latest",
			Features:           []string{"ml-correlation", "intelligent-remediation", "context-analysis"},
		},
		Metadata: map[string]interface{}{
			"clientVersion": h.version,
			"timestamp":     time.Now().Format(time.RFC3339),
			"bundleSize":    len(bundleData),
		},
	}

	// Serialize request
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	// Check size limits
	if int64(len(payload)) > h.config.MaxBundleSize {
		return nil, fmt.Errorf("payload size %d exceeds maximum %d bytes", len(payload), h.config.MaxBundleSize)
	}

	return payload, nil
}

// executeAnalysisWithRetries executes the analysis request with retry logic
func (h *HostedAgent) executeAnalysisWithRetries(ctx context.Context, payload []byte) (*AnalysisResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= h.config.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(h.config.RetryDelay * time.Duration(attempt)):
				// Continue with retry
			}
		}

		result, err := h.executeAnalysisRequest(ctx, payload)
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Don't retry on certain errors
		if isNonRetryableError(err) {
			break
		}
	}

	return nil, fmt.Errorf("analysis failed after %d attempts: %w", h.config.MaxRetries+1, lastErr)
}

// executeAnalysisRequest executes a single analysis request
func (h *HostedAgent) executeAnalysisRequest(ctx context.Context, payload []byte) (*AnalysisResponse, error) {
	// Build analysis URL
	url := h.config.BaseURL + "/" + h.config.APIVersion + h.config.Endpoints["analyze"]

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("failed to create analysis request: %w", err)
	}

	// Set content type
	req.Header.Set("Content-Type", "application/json")

	// Add authentication
	if err := h.addAuthentication(req); err != nil {
		return nil, fmt.Errorf("failed to add authentication: %w", err)
	}

	// Add custom headers
	h.addCustomHeaders(req)

	// Execute request
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("analysis request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("analysis request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var result AnalysisResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse analysis response: %w", err)
	}

	// Check for service-level errors
	if result.Error != "" {
		return nil, fmt.Errorf("analysis service error: %s", result.Error)
	}

	return &result, nil
}

// addAuthentication adds appropriate authentication to the request
func (h *HostedAgent) addAuthentication(req *http.Request) error {
	switch h.config.AuthType {
	case "bearer":
		token, err := h.getBearerToken()
		if err != nil {
			return fmt.Errorf("failed to get bearer token: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)

	case "api-key":
		if h.config.APIKey == "" {
			return fmt.Errorf("API key not configured")
		}
		req.Header.Set("X-API-Key", h.config.APIKey)

	default:
		return fmt.Errorf("unsupported auth type: %s", h.config.AuthType)
	}

	return nil
}

// getBearerToken obtains or refreshes the bearer token
func (h *HostedAgent) getBearerToken() (string, error) {
	h.mu.RLock()
	if h.authToken != "" && time.Now().Before(h.authExpiry) {
		token := h.authToken
		h.mu.RUnlock()
		return token, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// Double-check after acquiring write lock
	if h.authToken != "" && time.Now().Before(h.authExpiry) {
		return h.authToken, nil
	}

	// Need to obtain new token
	token, expiry, err := h.fetchBearerToken()
	if err != nil {
		return "", err
	}

	h.authToken = token
	h.authExpiry = expiry

	return token, nil
}

// fetchBearerToken fetches a new bearer token from the auth endpoint
func (h *HostedAgent) fetchBearerToken() (string, time.Time, error) {
	// Build auth URL
	url := h.config.BaseURL + "/" + h.config.APIVersion + h.config.Endpoints["auth"]

	// Create auth request payload
	authPayload := map[string]string{
		"client_id":     h.config.ClientID,
		"client_secret": h.config.ClientSecret,
		"grant_type":    "client_credentials",
	}

	payload, err := json.Marshal(authPayload)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create auth payload: %w", err)
	}

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewReader(payload))
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to create auth request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := h.client.Do(req)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", time.Time{}, fmt.Errorf("auth failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse auth response
	var authResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		TokenType   string `json:"token_type"`
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to read auth response: %w", err)
	}

	if err := json.Unmarshal(body, &authResp); err != nil {
		return "", time.Time{}, fmt.Errorf("failed to parse auth response: %w", err)
	}

	// Calculate expiry time (with 5 minute buffer)
	expiry := time.Now().Add(time.Duration(authResp.ExpiresIn)*time.Second - 5*time.Minute)

	return authResp.AccessToken, expiry, nil
}

// addCustomHeaders adds any custom headers to the request
func (h *HostedAgent) addCustomHeaders(req *http.Request) {
	for key, value := range h.config.CustomHeaders {
		req.Header.Set(key, value)
	}
}

// validateConfig validates the hosted agent configuration
func validateConfig(config *HostedAgentConfig) error {
	if config.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}

	if config.RequestsPerSecond <= 0 {
		return fmt.Errorf("requests per second must be positive")
	}

	if config.BurstSize <= 0 {
		return fmt.Errorf("burst size must be positive")
	}

	if config.MaxRetries < 0 {
		return fmt.Errorf("max retries cannot be negative")
	}

	if config.RequestTimeout <= 0 {
		return fmt.Errorf("request timeout must be positive")
	}

	switch config.AuthType {
	case "bearer":
		if config.ClientID == "" || config.ClientSecret == "" {
			return fmt.Errorf("client ID and secret required for bearer auth")
		}
	case "api-key":
		// API key can be empty for testing purposes - it will be validated when used
	default:
		return fmt.Errorf("unsupported auth type: %s", config.AuthType)
	}

	return nil
}

// isNonRetryableError determines if an error should not be retried
func isNonRetryableError(err error) bool {
	// Add logic to identify non-retryable errors (4xx status codes, auth errors, etc.)
	return false // For now, retry all errors
}
