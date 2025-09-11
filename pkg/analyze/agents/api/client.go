package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// HTTPClient provides a robust HTTP client with advanced features
type HTTPClient struct {
	client          *http.Client
	config          *ClientConfig
	credentialStore CredentialStore
	rateLimiter     RateLimiter
	retryPolicy     RetryPolicy
}

// ClientConfig contains configuration for the HTTP client
type ClientConfig struct {
	// Timeout settings
	ConnectTimeout  time.Duration `json:"connectTimeout"`
	RequestTimeout  time.Duration `json:"requestTimeout"`
	IdleConnTimeout time.Duration `json:"idleConnTimeout"`

	// Connection pooling
	MaxIdleConns        int `json:"maxIdleConns"`
	MaxIdleConnsPerHost int `json:"maxIdleConnsPerHost"`
	MaxConnsPerHost     int `json:"maxConnsPerHost"`

	// Security settings
	InsecureSkipVerify bool     `json:"insecureSkipVerify"`
	TrustedCACerts     []string `json:"trustedCACerts"`
	ClientCertFile     string   `json:"clientCertFile"`
	ClientKeyFile      string   `json:"clientKeyFile"`

	// Proxy settings
	ProxyURL     string   `json:"proxyUrl"`
	NoProxyHosts []string `json:"noProxyHosts"`

	// User agent and headers
	UserAgent      string            `json:"userAgent"`
	DefaultHeaders map[string]string `json:"defaultHeaders"`

	// Compression
	EnableCompression bool `json:"enableCompression"`

	// Logging and debugging
	EnableDebugLogging bool `json:"enableDebugLogging"`
	LogRequestHeaders  bool `json:"logRequestHeaders"`
	LogResponseHeaders bool `json:"logResponseHeaders"`
}

// DefaultClientConfig returns a default client configuration
func DefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		ConnectTimeout:      10 * time.Second,
		RequestTimeout:      30 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 5,
		MaxConnsPerHost:     10,
		InsecureSkipVerify:  false,
		UserAgent:           "troubleshoot-agent/1.0",
		DefaultHeaders:      make(map[string]string),
		EnableCompression:   true,
		EnableDebugLogging:  false,
		LogRequestHeaders:   false,
		LogResponseHeaders:  false,
	}
}

// NewHTTPClient creates a new HTTP client with the provided configuration
func NewHTTPClient(config *ClientConfig, credStore CredentialStore, rateLimiter RateLimiter, retryPolicy RetryPolicy) (*HTTPClient, error) {
	if config == nil {
		config = DefaultClientConfig()
	}

	// Create custom transport
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.ConnectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     config.MaxConnsPerHost,
		IdleConnTimeout:     config.IdleConnTimeout,
		DisableCompression:  !config.EnableCompression,
	}

	// Configure TLS
	if config.InsecureSkipVerify || len(config.TrustedCACerts) > 0 || config.ClientCertFile != "" {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: config.InsecureSkipVerify,
		}
		transport.TLSClientConfig = tlsConfig
	}

	// Configure proxy
	if config.ProxyURL != "" {
		proxyURL, err := url.Parse(config.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	// Create HTTP client
	client := &http.Client{
		Transport: transport,
		Timeout:   config.RequestTimeout,
	}

	return &HTTPClient{
		client:          client,
		config:          config,
		credentialStore: credStore,
		rateLimiter:     rateLimiter,
		retryPolicy:     retryPolicy,
	}, nil
}

// Request represents an HTTP request with enhanced features
type Request struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	Headers     map[string]string `json:"headers"`
	Body        interface{}       `json:"body"`
	QueryParams map[string]string `json:"queryParams"`

	// Request-specific settings
	Timeout       time.Duration `json:"timeout"`
	RetryPolicy   *RetryPolicy  `json:"retryPolicy"`
	SkipRateLimit bool          `json:"skipRateLimit"`
	RequireAuth   bool          `json:"requireAuth"`
	AuthType      string        `json:"authType"`

	// Metadata
	RequestID     string                 `json:"requestId"`
	OperationName string                 `json:"operationName"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// Response represents an HTTP response with additional metadata
type Response struct {
	StatusCode    int               `json:"statusCode"`
	Status        string            `json:"status"`
	Headers       map[string]string `json:"headers"`
	Body          []byte            `json:"body"`
	ContentLength int64             `json:"contentLength"`

	// Timing information
	RequestDuration  time.Duration `json:"requestDuration"`
	DNSLookupTime    time.Duration `json:"dnsLookupTime"`
	ConnectionTime   time.Duration `json:"connectionTime"`
	TLSHandshakeTime time.Duration `json:"tlsHandshakeTime"`

	// Metadata
	RequestID    string                 `json:"requestId"`
	AttemptCount int                    `json:"attemptCount"`
	FromCache    bool                   `json:"fromCache"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Do executes an HTTP request with all the configured enhancements
func (c *HTTPClient) Do(ctx context.Context, req *Request) (*Response, error) {
	// Apply rate limiting if not skipped
	if !req.SkipRateLimit && c.rateLimiter != nil {
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit error: %w", err)
		}
	}

	// Execute with retry policy
	return c.executeWithRetries(ctx, req)
}

// executeWithRetries executes the request with the configured retry policy
func (c *HTTPClient) executeWithRetries(ctx context.Context, req *Request) (*Response, error) {
	var lastResponse *Response
	var lastErr error

	retryPolicy := req.RetryPolicy
	if retryPolicy == nil {
		retryPolicy = &c.retryPolicy
	}

	maxAttempts := retryPolicy.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Add delay before retry (except first attempt)
		if attempt > 1 {
			delay := retryPolicy.CalculateDelay(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				// Continue with retry
			}
		}

		response, err := c.executeSingleRequest(ctx, req, attempt)
		if err == nil && !retryPolicy.ShouldRetry(response.StatusCode, nil) {
			response.AttemptCount = attempt
			return response, nil
		}

		lastResponse = response
		lastErr = err

		// Don't retry if context is cancelled
		if ctx.Err() != nil {
			break
		}

		// Don't retry if the error/response is not retryable
		if !retryPolicy.ShouldRetry(response.StatusCode, err) {
			break
		}
	}

	if lastResponse != nil {
		lastResponse.AttemptCount = maxAttempts
		return lastResponse, lastErr
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", maxAttempts, lastErr)
}

// executeSingleRequest executes a single HTTP request
func (c *HTTPClient) executeSingleRequest(ctx context.Context, req *Request, attempt int) (*Response, error) {
	startTime := time.Now()

	// Build HTTP request
	httpReq, err := c.buildHTTPRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to build HTTP request: %w", err)
	}

	// Add authentication if required
	if req.RequireAuth && c.credentialStore != nil {
		if err := c.addAuthentication(httpReq, req.AuthType); err != nil {
			return nil, fmt.Errorf("authentication failed: %w", err)
		}
	}

	// Execute the request
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// Read response body
	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Build response headers map
	headers := make(map[string]string)
	for key, values := range httpResp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}

	response := &Response{
		StatusCode:      httpResp.StatusCode,
		Status:          httpResp.Status,
		Headers:         headers,
		Body:            body,
		ContentLength:   httpResp.ContentLength,
		RequestDuration: time.Since(startTime),
		RequestID:       req.RequestID,
		AttemptCount:    attempt,
		Metadata:        make(map[string]interface{}),
	}

	// Add debug logging if enabled
	if c.config.EnableDebugLogging {
		c.logRequest(httpReq, req)
		c.logResponse(httpResp, response)
	}

	return response, nil
}

// buildHTTPRequest builds an http.Request from the enhanced Request
func (c *HTTPClient) buildHTTPRequest(ctx context.Context, req *Request) (*http.Request, error) {
	// Build URL with query parameters
	reqURL := req.URL
	if len(req.QueryParams) > 0 {
		u, err := url.Parse(reqURL)
		if err != nil {
			return nil, fmt.Errorf("invalid URL: %w", err)
		}

		q := u.Query()
		for key, value := range req.QueryParams {
			q.Set(key, value)
		}
		u.RawQuery = q.Encode()
		reqURL = u.String()
	}

	// Serialize body if provided
	var bodyReader io.Reader
	if req.Body != nil {
		jsonBody, err := json.Marshal(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, req.Method, reqURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Add default headers
	for key, value := range c.config.DefaultHeaders {
		httpReq.Header.Set(key, value)
	}

	// Add request-specific headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set User-Agent if configured
	if c.config.UserAgent != "" {
		httpReq.Header.Set("User-Agent", c.config.UserAgent)
	}

	// Set Content-Type for JSON body
	if req.Body != nil && httpReq.Header.Get("Content-Type") == "" {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	return httpReq, nil
}

// addAuthentication adds authentication headers to the request
func (c *HTTPClient) addAuthentication(req *http.Request, authType string) error {
	if c.credentialStore == nil {
		return fmt.Errorf("credential store not configured")
	}

	creds, err := c.credentialStore.GetCredentials(authType)
	if err != nil {
		return fmt.Errorf("failed to get credentials: %w", err)
	}

	switch authType {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+creds.Token)
	case "api-key":
		req.Header.Set("X-API-Key", creds.APIKey)
	case "basic":
		req.SetBasicAuth(creds.Username, creds.Password)
	default:
		return fmt.Errorf("unsupported auth type: %s", authType)
	}

	return nil
}

// logRequest logs the HTTP request for debugging
func (c *HTTPClient) logRequest(httpReq *http.Request, req *Request) {
	fmt.Printf("[DEBUG] HTTP Request: %s %s\n", httpReq.Method, httpReq.URL.String())

	if c.config.LogRequestHeaders {
		fmt.Printf("[DEBUG] Request Headers:\n")
		for key, values := range httpReq.Header {
			for _, value := range values {
				if strings.ToLower(key) == "authorization" {
					value = "[REDACTED]"
				}
				fmt.Printf("  %s: %s\n", key, value)
			}
		}
	}
}

// logResponse logs the HTTP response for debugging
func (c *HTTPClient) logResponse(httpResp *http.Response, resp *Response) {
	fmt.Printf("[DEBUG] HTTP Response: %d %s (Duration: %v)\n",
		httpResp.StatusCode, httpResp.Status, resp.RequestDuration)

	if c.config.LogResponseHeaders {
		fmt.Printf("[DEBUG] Response Headers:\n")
		for key, value := range resp.Headers {
			fmt.Printf("  %s: %s\n", key, value)
		}
	}
}

// CredentialStore interface for managing authentication credentials
type CredentialStore interface {
	GetCredentials(authType string) (*Credentials, error)
	SetCredentials(authType string, creds *Credentials) error
	RefreshCredentials(authType string) (*Credentials, error)
}

// Credentials represents authentication credentials
type Credentials struct {
	Token     string            `json:"token"`
	APIKey    string            `json:"apiKey"`
	Username  string            `json:"username"`
	Password  string            `json:"password"`
	ExpiresAt time.Time         `json:"expiresAt"`
	Metadata  map[string]string `json:"metadata"`
}

// RateLimiter interface for rate limiting requests
type RateLimiter interface {
	Wait(ctx context.Context) error
	Allow() bool
}

// RetryPolicy defines when and how to retry failed requests
type RetryPolicy struct {
	MaxAttempts          int           `json:"maxAttempts"`
	BaseDelay            time.Duration `json:"baseDelay"`
	MaxDelay             time.Duration `json:"maxDelay"`
	BackoffMultiplier    float64       `json:"backoffMultiplier"`
	RetryableStatusCodes []int         `json:"retryableStatusCodes"`
	RetryableErrors      []string      `json:"retryableErrors"`
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxAttempts:          3,
		BaseDelay:            1 * time.Second,
		MaxDelay:             30 * time.Second,
		BackoffMultiplier:    2.0,
		RetryableStatusCodes: []int{429, 502, 503, 504},
		RetryableErrors:      []string{"timeout", "connection", "dns"},
	}
}

// ShouldRetry determines if a request should be retried based on status code and error
func (rp *RetryPolicy) ShouldRetry(statusCode int, err error) bool {
	// Check if status code is retryable
	for _, code := range rp.RetryableStatusCodes {
		if statusCode == code {
			return true
		}
	}

	// Check if error is retryable
	if err != nil {
		errorStr := strings.ToLower(err.Error())
		for _, retryableError := range rp.RetryableErrors {
			if strings.Contains(errorStr, retryableError) {
				return true
			}
		}
	}

	return false
}

// CalculateDelay calculates the delay for a retry attempt
func (rp *RetryPolicy) CalculateDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return 0
	}

	delay := time.Duration(float64(rp.BaseDelay) *
		(rp.BackoffMultiplier * float64(attempt-1)))

	if delay > rp.MaxDelay {
		delay = rp.MaxDelay
	}

	return delay
}
