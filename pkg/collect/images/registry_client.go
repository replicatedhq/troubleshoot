package images

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

// DefaultRegistryClient implements the RegistryClient interface
type DefaultRegistryClient struct {
	httpClient  *http.Client
	credentials RegistryCredentials
	registry    string
	userAgent   string
}

// NewRegistryClient creates a new registry client for the specified registry
func NewRegistryClient(registry string, credentials RegistryCredentials, options CollectionOptions) (*DefaultRegistryClient, error) {
	if registry == "" {
		registry = DefaultRegistry
	}

	// Normalize registry URL
	if !strings.HasPrefix(registry, "http://") && !strings.HasPrefix(registry, "https://") {
		// Default to HTTPS for security
		registry = "https://" + registry
	}

	// Configure HTTP client
	tlsConfig := &tls.Config{
		InsecureSkipVerify: options.SkipTLSVerify,
	}

	// Add custom CA cert if provided
	if credentials.CACert != "" {
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM([]byte(credentials.CACert)); !ok {
			return nil, errors.New("failed to parse CA certificate")
		}
		tlsConfig.RootCAs = caCertPool
		klog.V(2).Info("Custom CA certificate loaded successfully")
	}

	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   options.Timeout,
	}

	client := &DefaultRegistryClient{
		httpClient:  httpClient,
		credentials: credentials,
		registry:    registry,
		userAgent:   "troubleshoot-image-collector/1.0",
	}

	return client, nil
}

// GetManifest retrieves the image manifest
func (c *DefaultRegistryClient) GetManifest(ctx context.Context, imageRef ImageReference) (Manifest, error) {
	klog.V(3).Infof("Getting manifest for image: %s", imageRef.String())

	// Build manifest URL
	manifestURL := fmt.Sprintf("%s/v2/%s/manifests/%s",
		c.registry, imageRef.Repository, c.getReference(imageRef))

	req, err := http.NewRequestWithContext(ctx, "GET", manifestURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create manifest request")
	}

	// Set required headers
	req.Header.Set("Accept", strings.Join([]string{
		DockerManifestSchema2,
		DockerManifestListSchema2,
		OCIManifestSchema1,
		OCIImageIndex,
		DockerManifestSchema1, // Fallback for older registries
	}, ","))
	req.Header.Set("User-Agent", c.userAgent)

	// Add authentication
	if err := c.addAuth(req, imageRef.Repository); err != nil {
		return nil, errors.Wrap(err, "failed to add authentication")
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get manifest")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	// Read manifest content
	manifestBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read manifest response")
	}

	// Parse manifest based on media type
	contentType := resp.Header.Get("Content-Type")
	manifest, err := c.parseManifest(manifestBytes, contentType)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse manifest")
	}

	return manifest, nil
}

// GetBlob retrieves a blob by digest
func (c *DefaultRegistryClient) GetBlob(ctx context.Context, imageRef ImageReference, digest string) (io.ReadCloser, error) {
	klog.V(3).Infof("Getting blob %s for image: %s", digest, imageRef.String())

	// Build blob URL
	blobURL := fmt.Sprintf("%s/v2/%s/blobs/%s",
		c.registry, imageRef.Repository, digest)

	req, err := http.NewRequestWithContext(ctx, "GET", blobURL, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create blob request")
	}

	req.Header.Set("User-Agent", c.userAgent)

	// Add authentication
	if err := c.addAuth(req, imageRef.Repository); err != nil {
		return nil, errors.Wrap(err, "failed to add authentication")
	}

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get blob")
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registry returned status %d: %s", resp.StatusCode, string(body))
	}

	return resp.Body, nil
}

// SetCredentials configures authentication for the registry
func (c *DefaultRegistryClient) SetCredentials(credentials RegistryCredentials) error {
	c.credentials = credentials
	return nil
}

// Ping tests connectivity to the registry
func (c *DefaultRegistryClient) Ping(ctx context.Context) error {
	pingURL := fmt.Sprintf("%s/v2/", c.registry)

	req, err := http.NewRequestWithContext(ctx, "GET", pingURL, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create ping request")
	}

	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to ping registry")
	}
	defer resp.Body.Close()

	// Registry should return 200 or 401 (if authentication is required)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("registry ping failed with status: %d", resp.StatusCode)
	}

	return nil
}

// getReference returns the appropriate reference (tag or digest) for the image
func (c *DefaultRegistryClient) getReference(imageRef ImageReference) string {
	if imageRef.Digest != "" {
		return imageRef.Digest
	}
	if imageRef.Tag != "" {
		return imageRef.Tag
	}
	return DefaultTag
}

// addAuth adds authentication to the request
func (c *DefaultRegistryClient) addAuth(req *http.Request, repository string) error {
	// Handle different authentication methods
	if c.credentials.Token != "" {
		// Bearer token authentication
		req.Header.Set("Authorization", "Bearer "+c.credentials.Token)
		return nil
	}

	if c.credentials.Username != "" && c.credentials.Password != "" {
		// Basic authentication
		auth := base64.StdEncoding.EncodeToString(
			[]byte(c.credentials.Username + ":" + c.credentials.Password))
		req.Header.Set("Authorization", "Basic "+auth)
		return nil
	}

	// For Docker Hub, try to get a token if no credentials provided
	if c.isDockerHub() && c.credentials.Username == "" {
		token, err := c.getDockerHubToken(req.Context(), repository)
		if err != nil {
			klog.V(2).Infof("Failed to get Docker Hub token: %v", err)
			// Continue without authentication for public images
			return nil
		}
		req.Header.Set("Authorization", "Bearer "+token)
		return nil
	}

	return nil
}

// isDockerHub checks if this is Docker Hub registry
func (c *DefaultRegistryClient) isDockerHub() bool {
	return strings.Contains(c.registry, "docker.io") ||
		strings.Contains(c.registry, "registry-1.docker.io")
}

// getDockerHubToken gets an anonymous token for Docker Hub
func (c *DefaultRegistryClient) getDockerHubToken(ctx context.Context, repository string) (string, error) {
	// Docker Hub token URL
	tokenURL := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repository)

	req, err := http.NewRequestWithContext(ctx, "GET", tokenURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed with status: %d", resp.StatusCode)
	}

	var tokenResp struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	return tokenResp.Token, nil
}

// parseManifest parses the manifest based on its media type
func (c *DefaultRegistryClient) parseManifest(data []byte, mediaType string) (Manifest, error) {
	switch mediaType {
	case DockerManifestSchema2, OCIManifestSchema1:
		return parseV2Manifest(data)
	case DockerManifestListSchema2, OCIImageIndex:
		return parseManifestList(data)
	case DockerManifestSchema1:
		return parseV1Manifest(data)
	default:
		// Try to auto-detect based on content
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, errors.Wrap(err, "failed to parse manifest JSON")
		}

		if schemaVersion, ok := raw["schemaVersion"]; ok {
			switch schemaVersion {
			case float64(2):
				if _, hasManifests := raw["manifests"]; hasManifests {
					return parseManifestList(data)
				}
				return parseV2Manifest(data)
			case float64(1):
				return parseV1Manifest(data)
			}
		}

		return nil, fmt.Errorf("unsupported manifest media type: %s", mediaType)
	}
}

// RegistryClientFactory creates registry clients for different registry types
type RegistryClientFactory struct {
	defaultOptions CollectionOptions
}

// NewRegistryClientFactory creates a new registry client factory
func NewRegistryClientFactory(options CollectionOptions) *RegistryClientFactory {
	return &RegistryClientFactory{
		defaultOptions: options,
	}
}

// CreateClient creates a registry client for the specified registry
func (f *RegistryClientFactory) CreateClient(registry string, credentials RegistryCredentials) (RegistryClient, error) {
	// Use factory default options merged with any specific credentials
	options := f.defaultOptions

	// Apply registry-specific configurations
	switch {
	case strings.Contains(registry, "amazonaws.com"):
		// AWS ECR specific configuration
		options.SkipTLSVerify = false // ECR requires TLS
	case strings.Contains(registry, "gcr.io"):
		// Google Container Registry specific configuration
		options.SkipTLSVerify = false // GCR requires TLS
	case strings.Contains(registry, "docker.io"):
		// Docker Hub specific configuration
		registry = "https://registry-1.docker.io" // Use proper Docker Hub registry URL
	}

	return NewRegistryClient(registry, credentials, options)
}

// GetSupportedRegistries returns a list of well-known registry patterns
func (f *RegistryClientFactory) GetSupportedRegistries() []string {
	return []string{
		"docker.io",
		"registry-1.docker.io",
		"gcr.io",
		"us.gcr.io",
		"eu.gcr.io",
		"asia.gcr.io",
		"*.amazonaws.com", // ECR
		"quay.io",
		"registry.redhat.io",
		"harbor.*",
	}
}
