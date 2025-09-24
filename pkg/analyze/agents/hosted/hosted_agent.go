package hosted

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"k8s.io/klog/v2"
)

// HostedAgent implements the Agent interface for remote analysis services
type HostedAgent struct {
	name         string
	endpoint     string
	apiKey       string
	client       *http.Client
	capabilities []string
	enabled      bool
	version      string
	rateLimiter  *RateLimiter
	retryConfig  *RetryConfig
}

// HostedAgentOptions configures the hosted agent
type HostedAgentOptions struct {
	Endpoint           string
	APIKey             string
	Timeout            time.Duration
	MaxRetries         int
	RateLimit          int // requests per minute
	InsecureSkipVerify bool
	CustomHeaders      map[string]string
}

// RateLimiter manages API rate limiting
type RateLimiter struct {
	tokens    chan struct{}
	interval  time.Duration
	lastReset time.Time
	stopCh    chan struct{}
	stopped   bool
}

// RetryConfig defines retry behavior
type RetryConfig struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
	Multiplier float64
}

// HostedAnalysisRequest represents the request payload for hosted analysis
type HostedAnalysisRequest struct {
	BundleData []byte                  `json:"bundleData"`
	Analyzers  []analyzer.AnalyzerSpec `json:"analyzers"`
	Options    HostedAnalysisOptions   `json:"options"`
	Metadata   RequestMetadata         `json:"metadata"`
}

// HostedAnalysisOptions configures the analysis request
type HostedAnalysisOptions struct {
	IncludeRemediation bool     `json:"includeRemediation"`
	AnalysisTypes      []string `json:"analysisTypes,omitempty"`
	Priority           string   `json:"priority,omitempty"`
	Timeout            int      `json:"timeout,omitempty"`
}

// RequestMetadata provides context about the request
type RequestMetadata struct {
	RequestID     string            `json:"requestId"`
	ClientVersion string            `json:"clientVersion"`
	Timestamp     time.Time         `json:"timestamp"`
	Labels        map[string]string `json:"labels,omitempty"`
}

// HostedAnalysisResponse represents the response from hosted analysis
type HostedAnalysisResponse struct {
	Results   []*analyzer.AnalyzerResult `json:"results"`
	Metadata  HostedResponseMetadata     `json:"metadata"`
	Errors    []string                   `json:"errors,omitempty"`
	Status    string                     `json:"status"`
	RequestID string                     `json:"requestId"`
}

// HostedResponseMetadata provides analysis metadata from the service
type HostedResponseMetadata struct {
	Duration       time.Duration `json:"duration"`
	AnalyzerCount  int           `json:"analyzerCount"`
	ServiceVersion string        `json:"serviceVersion"`
	ModelVersion   string        `json:"modelVersion,omitempty"`
	Confidence     float64       `json:"confidence,omitempty"`
}

// NewHostedAgent creates a new hosted analysis agent
func NewHostedAgent(opts *HostedAgentOptions) (*HostedAgent, error) {
	if opts == nil {
		return nil, errors.New("options cannot be nil")
	}

	if opts.Endpoint == "" {
		return nil, errors.New("endpoint is required")
	}

	if opts.APIKey == "" {
		return nil, errors.New("API key is required")
	}

	// Validate endpoint URL
	_, err := url.Parse(opts.Endpoint)
	if err != nil {
		return nil, errors.Wrap(err, "invalid endpoint URL")
	}

	// Set default timeout
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Minute
	}

	// Set default rate limit
	if opts.RateLimit == 0 {
		opts.RateLimit = 60 // 60 requests per minute
	}

	// Set default retry config
	if opts.MaxRetries == 0 {
		opts.MaxRetries = 3
	}

	// Create HTTP client with timeout and TLS config
	client := &http.Client{
		Timeout: opts.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: opts.InsecureSkipVerify,
			},
		},
	}

	agent := &HostedAgent{
		name:     "hosted",
		endpoint: strings.TrimSuffix(opts.Endpoint, "/"),
		apiKey:   opts.APIKey,
		client:   client,
		capabilities: []string{
			"advanced-analysis",
			"ml-powered",
			"correlation-detection",
			"trend-analysis",
			"intelligent-remediation",
			"multi-cluster-comparison",
		},
		enabled:     true,
		version:     "1.0.0",
		rateLimiter: NewRateLimiter(opts.RateLimit),
		retryConfig: &RetryConfig{
			MaxRetries: opts.MaxRetries,
			BaseDelay:  100 * time.Millisecond,
			MaxDelay:   30 * time.Second,
			Multiplier: 2.0,
		},
	}

	return agent, nil
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	tokens := make(chan struct{}, requestsPerMinute)
	interval := time.Minute / time.Duration(requestsPerMinute)

	// Fill the initial bucket
	for i := 0; i < requestsPerMinute; i++ {
		tokens <- struct{}{}
	}

	rl := &RateLimiter{
		tokens:    tokens,
		interval:  interval,
		lastReset: time.Now(),
		stopCh:    make(chan struct{}),
		stopped:   false,
	}

	// Start token replenishment goroutine
	go rl.replenishTokens()

	return rl
}

// replenishTokens refills the rate limiter token bucket
func (rl *RateLimiter) replenishTokens() {
	ticker := time.NewTicker(rl.interval)
	defer ticker.Stop()

	for {
		select {
		case <-rl.stopCh:
			// Stop signal received, exit goroutine
			return
		case <-ticker.C:
			select {
			case rl.tokens <- struct{}{}:
				// Token added successfully
			default:
				// Bucket is full, skip
			}
		}
	}
}

// waitForToken blocks until a token is available
func (rl *RateLimiter) waitForToken(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stop cleanly shuts down the rate limiter and stops the replenishment goroutine
func (rl *RateLimiter) Stop() {
	if !rl.stopped {
		rl.stopped = true
		close(rl.stopCh)
	}
}

// Name returns the agent name
func (a *HostedAgent) Name() string {
	return a.name
}

// IsAvailable checks if the hosted service is available
func (a *HostedAgent) IsAvailable() bool {
	if !a.enabled {
		return false
	}

	// Quick health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return a.HealthCheck(ctx) == nil
}

// Capabilities returns the agent's capabilities
func (a *HostedAgent) Capabilities() []string {
	return append([]string{}, a.capabilities...)
}

// HealthCheck verifies the hosted service is accessible and functioning
func (a *HostedAgent) HealthCheck(ctx context.Context) error {
	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "HostedAgent.HealthCheck")
	defer span.End()

	if !a.enabled {
		return errors.New("hosted agent is disabled")
	}

	healthURL := fmt.Sprintf("%s/health", a.endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		span.SetStatus(codes.Error, "failed to create health check request")
		return errors.Wrap(err, "failed to create health check request")
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))
	req.Header.Set("User-Agent", "troubleshoot-hosted-agent/1.0")

	resp, err := a.client.Do(req)
	if err != nil {
		span.SetStatus(codes.Error, "health check request failed")
		return errors.Wrap(err, "health check request failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		span.SetStatus(codes.Error, fmt.Sprintf("health check failed with status %d", resp.StatusCode))
		return errors.Errorf("health check failed with status %d", resp.StatusCode)
	}

	span.SetAttributes(attribute.String("health_status", "ok"))
	return nil
}

// Analyze performs analysis using the hosted service
func (a *HostedAgent) Analyze(ctx context.Context, data []byte, analyzers []analyzer.AnalyzerSpec) (*analyzer.AgentResult, error) {
	startTime := time.Now()

	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "HostedAgent.Analyze")
	defer span.End()

	if !a.enabled {
		return nil, errors.New("hosted agent is not enabled")
	}

	// Wait for rate limit token
	if err := a.rateLimiter.waitForToken(ctx); err != nil {
		return nil, errors.Wrap(err, "rate limit exceeded")
	}

	// Prepare the analysis request
	request := HostedAnalysisRequest{
		BundleData: data,
		Analyzers:  analyzers,
		Options: HostedAnalysisOptions{
			IncludeRemediation: true,
			Priority:           "standard",
			Timeout:            300, // 5 minutes
		},
		Metadata: RequestMetadata{
			RequestID:     fmt.Sprintf("req-%d", time.Now().UnixNano()),
			ClientVersion: a.version,
			Timestamp:     time.Now(),
		},
	}

	// Execute the request with retry logic
	response, err := a.executeWithRetry(ctx, request)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Convert hosted response to agent result
	result := &analyzer.AgentResult{
		Results: response.Results,
		Metadata: analyzer.AgentResultMetadata{
			Duration:      time.Since(startTime),
			AnalyzerCount: len(analyzers),
			Version:       response.Metadata.ServiceVersion,
		},
		Errors: response.Errors,
	}

	// Enhance results with hosted service metadata
	for _, r := range result.Results {
		r.AgentName = a.name
		if response.Metadata.Confidence > 0 {
			r.Confidence = response.Metadata.Confidence
		}
	}

	span.SetAttributes(
		attribute.Int("total_analyzers", len(analyzers)),
		attribute.Int("successful_results", len(result.Results)),
		attribute.Int("errors", len(result.Errors)),
		attribute.String("request_id", request.Metadata.RequestID),
		attribute.String("service_version", response.Metadata.ServiceVersion),
	)

	return result, nil
}

// executeWithRetry executes the analysis request with retry logic
func (a *HostedAgent) executeWithRetry(ctx context.Context, request HostedAnalysisRequest) (*HostedAnalysisResponse, error) {
	var lastErr error

	for attempt := 0; attempt <= a.retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay
			delay := time.Duration(float64(a.retryConfig.BaseDelay) *
				float64(attempt) * a.retryConfig.Multiplier)
			if delay > a.retryConfig.MaxDelay {
				delay = a.retryConfig.MaxDelay
			}

			klog.V(2).Infof("Retrying hosted analysis request (attempt %d/%d) after %v",
				attempt, a.retryConfig.MaxRetries, delay)

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}

		response, err := a.executeRequest(ctx, request)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Don't retry certain errors
		if isNonRetryableError(err) {
			break
		}
	}

	return nil, errors.Wrapf(lastErr, "hosted analysis failed after %d attempts", a.retryConfig.MaxRetries+1)
}

// executeRequest executes a single analysis request
func (a *HostedAgent) executeRequest(ctx context.Context, request HostedAnalysisRequest) (*HostedAnalysisResponse, error) {
	// Marshal the request
	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal request")
	}

	// Create HTTP request
	analyzeURL := fmt.Sprintf("%s/analyze", a.endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", analyzeURL, bytes.NewReader(requestBody))
	if err != nil {
		return nil, errors.Wrap(err, "failed to create HTTP request")
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.apiKey))
	req.Header.Set("User-Agent", "troubleshoot-hosted-agent/1.0")
	req.Header.Set("X-Request-ID", request.Metadata.RequestID)

	// Execute request
	resp, err := a.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "HTTP request failed")
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}

	// Check HTTP status
	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("analysis request failed with status %d: %s",
			resp.StatusCode, string(body))
	}

	// Parse response
	var response HostedAnalysisResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, errors.Wrap(err, "failed to parse response")
	}

	// Validate response
	if response.Status != "success" && response.Status != "completed" {
		return nil, errors.Errorf("analysis failed with status: %s", response.Status)
	}

	return &response, nil
}

// isNonRetryableError determines if an error should not be retried
func isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "400") || // Bad Request
		strings.Contains(errStr, "401") || // Unauthorized
		strings.Contains(errStr, "403") || // Forbidden
		strings.Contains(errStr, "422") // Unprocessable Entity
}

// SetEnabled enables or disables the hosted agent
func (a *HostedAgent) SetEnabled(enabled bool) {
	a.enabled = enabled
}

// Stop cleanly shuts down the hosted agent and stops background goroutines
func (a *HostedAgent) Stop() {
	if a.rateLimiter != nil {
		a.rateLimiter.Stop()
	}
}

// UpdateCredentials updates the API key for authentication
func (a *HostedAgent) UpdateCredentials(apiKey string) error {
	if apiKey == "" {
		return errors.New("API key cannot be empty")
	}
	a.apiKey = apiKey
	return nil
}

// GetEndpoint returns the current endpoint URL
func (a *HostedAgent) GetEndpoint() string {
	return a.endpoint
}

// GetStats returns usage statistics for the hosted agent
func (a *HostedAgent) GetStats() HostedAgentStats {
	return HostedAgentStats{
		Enabled:      a.enabled,
		Endpoint:     a.endpoint,
		Version:      a.version,
		Capabilities: len(a.capabilities),
		// Additional stats would be tracked with counters
	}
}

// HostedAgentStats provides usage statistics
type HostedAgentStats struct {
	Enabled          bool    `json:"enabled"`
	Endpoint         string  `json:"endpoint"`
	Version          string  `json:"version"`
	Capabilities     int     `json:"capabilities"`
	RequestsThisHour int64   `json:"requestsThisHour,omitempty"`
	SuccessRate      float64 `json:"successRate,omitempty"`
	AverageLatency   string  `json:"averageLatency,omitempty"`
}
