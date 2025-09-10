package images

import (
	"context"
	"io"
	"time"
)

// ImageFacts contains comprehensive metadata about a container image
type ImageFacts struct {
	// Basic image identification
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest"`
	Registry   string `json:"registry"`
	
	// Image metadata
	Size      int64     `json:"size"`
	Created   time.Time `json:"created"`
	Labels    map[string]string `json:"labels"`
	Platform  Platform  `json:"platform"`
	
	// Manifest information
	MediaType    string   `json:"mediaType"`
	SchemaVersion int     `json:"schemaVersion"`
	
	// Layer information
	Layers []LayerInfo `json:"layers,omitempty"`
	
	// Configuration
	Config ImageConfig `json:"config,omitempty"`
	
	// Collection metadata
	CollectedAt time.Time `json:"collectedAt"`
	Source      string    `json:"source"` // pod/deployment/etc that referenced this image
	Error       string    `json:"error,omitempty"` // any collection errors
}

// Platform represents the target platform for the image
type Platform struct {
	Architecture string `json:"architecture"`
	OS           string `json:"os"`
	Variant      string `json:"variant,omitempty"`
	OSVersion    string `json:"osVersion,omitempty"`
	OSFeatures   []string `json:"osFeatures,omitempty"`
}

// LayerInfo contains information about an image layer
type LayerInfo struct {
	Digest    string `json:"digest"`
	Size      int64  `json:"size"`
	MediaType string `json:"mediaType"`
}

// ImageConfig contains image configuration details
type ImageConfig struct {
	User         string            `json:"user,omitempty"`
	ExposedPorts map[string]struct{} `json:"exposedPorts,omitempty"`
	Env          []string          `json:"env,omitempty"`
	Entrypoint   []string          `json:"entrypoint,omitempty"`
	Cmd          []string          `json:"cmd,omitempty"`
	Volumes      map[string]struct{} `json:"volumes,omitempty"`
	WorkingDir   string            `json:"workingDir,omitempty"`
}

// ImageReference represents a reference to a container image
type ImageReference struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest,omitempty"`
}

// String returns the full image reference string
func (ir ImageReference) String() string {
	if ir.Registry == "" {
		ir.Registry = "docker.io"
	}
	
	ref := ir.Registry + "/" + ir.Repository
	if ir.Digest != "" {
		return ref + "@" + ir.Digest
	}
	if ir.Tag != "" {
		return ref + ":" + ir.Tag
	}
	return ref + ":latest"
}

// ImageCollector defines the interface for collecting image metadata
type ImageCollector interface {
	// CollectImageFacts collects metadata for a single image
	CollectImageFacts(ctx context.Context, imageRef ImageReference) (*ImageFacts, error)
	
	// CollectMultipleImageFacts collects metadata for multiple images concurrently
	CollectMultipleImageFacts(ctx context.Context, imageRefs []ImageReference) ([]ImageFacts, error)
	
	// SetCredentials configures registry authentication
	SetCredentials(registry string, credentials RegistryCredentials) error
}

// RegistryClient defines the interface for interacting with container registries
type RegistryClient interface {
	// GetManifest retrieves the image manifest
	GetManifest(ctx context.Context, imageRef ImageReference) (Manifest, error)
	
	// GetBlob retrieves a blob by digest
	GetBlob(ctx context.Context, imageRef ImageReference, digest string) (io.ReadCloser, error)
	
	// SetCredentials configures authentication for the registry
	SetCredentials(credentials RegistryCredentials) error
	
	// Ping tests connectivity to the registry
	Ping(ctx context.Context) error
}

// Manifest represents a container image manifest
type Manifest interface {
	// GetMediaType returns the manifest media type
	GetMediaType() string
	
	// GetSchemaVersion returns the manifest schema version
	GetSchemaVersion() int
	
	// GetConfig returns the config descriptor
	GetConfig() Descriptor
	
	// GetLayers returns the layer descriptors
	GetLayers() []Descriptor
	
	// GetPlatform returns the platform information
	GetPlatform() *Platform
	
	// Marshal serializes the manifest to JSON
	Marshal() ([]byte, error)
}

// Descriptor represents a content descriptor
type Descriptor struct {
	MediaType string            `json:"mediaType"`
	Size      int64             `json:"size"`
	Digest    string            `json:"digest"`
	URLs      []string          `json:"urls,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Platform  *Platform         `json:"platform,omitempty"`
}

// RegistryCredentials contains authentication information for a registry
type RegistryCredentials struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
	
	// For cloud provider authentication
	IdentityToken string `json:"identityToken,omitempty"`
	RegistryToken string `json:"registryToken,omitempty"`
	
	// TLS configuration
	Insecure bool `json:"insecure,omitempty"`
	CACert   string `json:"caCert,omitempty"`
}

// CollectionOptions configures image collection behavior
type CollectionOptions struct {
	// Registry authentication
	Credentials map[string]RegistryCredentials `json:"credentials,omitempty"`
	
	// Collection behavior
	IncludeLayers    bool          `json:"includeLayers"`
	IncludeConfig    bool          `json:"includeConfig"`
	Timeout          time.Duration `json:"timeout"`
	MaxConcurrency   int           `json:"maxConcurrency"`
	
	// Error handling
	ContinueOnError  bool `json:"continueOnError"`
	SkipTLSVerify    bool `json:"skipTLSVerify"`
	
	// Caching
	EnableCache      bool          `json:"enableCache"`
	CacheDuration    time.Duration `json:"cacheDuration"`
}

// CollectionResult contains the results of image fact collection
type CollectionResult struct {
	ImageFacts []ImageFacts `json:"imageFacts"`
	Errors     []error      `json:"errors,omitempty"`
	Duration   time.Duration `json:"duration"`
	Cached     int          `json:"cached"` // number of cached results used
}

// FactsBundle represents the facts.json output format
type FactsBundle struct {
	Version    string      `json:"version"`
	GeneratedAt time.Time  `json:"generatedAt"`
	Namespace   string     `json:"namespace,omitempty"`
	ImageFacts  []ImageFacts `json:"imageFacts"`
	Summary     FactsSummary `json:"summary"`
}

// FactsSummary provides high-level statistics about collected image facts
type FactsSummary struct {
	TotalImages       int `json:"totalImages"`
	UniqueRegistries  int `json:"uniqueRegistries"`
	UniqueRepositories int `json:"uniqueRepositories"`
	TotalSize         int64 `json:"totalSize"`
	CollectionErrors  int `json:"collectionErrors"`
}

// Known media types for Docker and OCI manifests
const (
	// Docker manifest media types
	DockerManifestSchema1     = "application/vnd.docker.distribution.manifest.v1+json"
	DockerManifestSchema2     = "application/vnd.docker.distribution.manifest.v2+json"
	DockerManifestListSchema2 = "application/vnd.docker.distribution.manifest.list.v2+json"
	
	// OCI manifest media types
	OCIManifestSchema1     = "application/vnd.oci.image.manifest.v1+json"
	OCIImageIndex          = "application/vnd.oci.image.index.v1+json"
	OCIImageConfig         = "application/vnd.oci.image.config.v1+json"
	OCIImageLayerTarGzip   = "application/vnd.oci.image.layer.v1.tar+gzip"
	OCIImageLayerTar       = "application/vnd.oci.image.layer.v1.tar"
	
	// Docker layer media types
	DockerImageLayerTarGzip = "application/vnd.docker.image.rootfs.diff.tar.gzip"
	DockerImageConfig       = "application/vnd.docker.container.image.v1+json"
	DockerImageLayer        = "application/vnd.docker.image.rootfs.diff.tar"
)

// Default values
const (
	DefaultTimeout       = 30 * time.Second
	DefaultMaxConcurrency = 5
	DefaultCacheDuration = 1 * time.Hour
	DefaultRegistry      = "docker.io"
	DefaultTag          = "latest"
)
