package images

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

// DefaultImageCollector implements the ImageCollector interface
type DefaultImageCollector struct {
	registryFactory *RegistryClientFactory
	cache          *imageCache
	options        CollectionOptions
}

// NewImageCollector creates a new image collector
func NewImageCollector(options CollectionOptions) *DefaultImageCollector {
	collector := &DefaultImageCollector{
		registryFactory: NewRegistryClientFactory(options),
		options:        options,
	}

	if options.EnableCache {
		collector.cache = newImageCache(options.CacheDuration)
	}

	return collector
}

// CollectImageFacts collects metadata for a single image
func (c *DefaultImageCollector) CollectImageFacts(ctx context.Context, imageRef ImageReference) (*ImageFacts, error) {
	klog.V(2).Infof("Collecting image facts for: %s", imageRef.String())

	start := time.Now()
	defer func() {
		klog.V(3).Infof("Image collection for %s took %v", imageRef.String(), time.Since(start))
	}()

	// Check cache first
	if c.cache != nil {
		if cached := c.cache.Get(imageRef.String()); cached != nil {
			klog.V(3).Infof("Using cached image facts for: %s", imageRef.String())
			return cached, nil
		}
	}

	// Parse registry from image reference
	registry := imageRef.Registry
	if registry == "" {
		registry = DefaultRegistry
	}

	// Get credentials for this registry
	credentials := c.getCredentialsForRegistry(registry)

	// Create registry client
	client, err := c.registryFactory.CreateClient(registry, credentials)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create registry client")
	}

	// Test connectivity first
	if err := client.Ping(ctx); err != nil {
		klog.V(2).Infof("Registry ping failed for %s: %v", registry, err)
		if !c.options.ContinueOnError {
			return nil, errors.Wrap(err, "registry connectivity test failed")
		}
	}

	// Collect the facts
	facts, err := c.collectImageFactsFromRegistry(ctx, client, imageRef)
	if err != nil {
		if c.options.ContinueOnError {
			// Return partial facts with error information
			return &ImageFacts{
				Repository:  imageRef.Repository,
				Tag:        imageRef.Tag,
				Digest:     imageRef.Digest,
				Registry:   registry,
				CollectedAt: time.Now(),
				Source:     "error-recovery",
				Error:      err.Error(),
			}, nil
		}
		return nil, err
	}

	// Cache the results
	if c.cache != nil {
		c.cache.Set(imageRef.String(), facts)
	}

	return facts, nil
}

// CollectMultipleImageFacts collects metadata for multiple images concurrently
func (c *DefaultImageCollector) CollectMultipleImageFacts(ctx context.Context, imageRefs []ImageReference) ([]ImageFacts, error) {
	if len(imageRefs) == 0 {
		return []ImageFacts{}, nil
	}

	klog.V(2).Infof("Collecting facts for %d images", len(imageRefs))

	// Use semaphore to limit concurrency
	maxConcurrency := c.options.MaxConcurrency
	if maxConcurrency <= 0 {
		maxConcurrency = DefaultMaxConcurrency
	}
	
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	results := make([]ImageFacts, len(imageRefs))
	var collectErrors []error

	for i, imageRef := range imageRefs {
		wg.Add(1)
		go func(index int, ref ImageReference) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			facts, err := c.CollectImageFacts(ctx, ref)
			
			mu.Lock()
			if err != nil {
				collectErrors = append(collectErrors, 
					fmt.Errorf("image %s: %w", ref.String(), err))
				if facts != nil {
					results[index] = *facts // Store partial facts
				}
			} else if facts != nil {
				results[index] = *facts
			}
			mu.Unlock()
		}(i, imageRef)
	}

	wg.Wait()

	// Filter out empty results
	var finalResults []ImageFacts
	for _, result := range results {
		if result.Repository != "" { // Non-empty result
			finalResults = append(finalResults, result)
		}
	}

	klog.V(2).Infof("Collected facts for %d/%d images (%d errors)", 
		len(finalResults), len(imageRefs), len(collectErrors))

	if len(collectErrors) > 0 && !c.options.ContinueOnError {
		return finalResults, fmt.Errorf("collection errors: %v", collectErrors)
	}

	return finalResults, nil
}

// SetCredentials configures registry authentication
func (c *DefaultImageCollector) SetCredentials(registry string, credentials RegistryCredentials) error {
	if c.options.Credentials == nil {
		c.options.Credentials = make(map[string]RegistryCredentials)
	}
	
	c.options.Credentials[registry] = credentials
	klog.V(3).Infof("Updated credentials for registry: %s", registry)
	return nil
}

// collectImageFactsFromRegistry collects image facts using a registry client
func (c *DefaultImageCollector) collectImageFactsFromRegistry(ctx context.Context, client RegistryClient, imageRef ImageReference) (*ImageFacts, error) {
	// Get the manifest
	manifest, err := client.GetManifest(ctx, imageRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get manifest")
	}

	facts := &ImageFacts{
		Repository:    imageRef.Repository,
		Tag:          imageRef.Tag,
		Digest:       imageRef.Digest,
		Registry:     imageRef.Registry,
		MediaType:    manifest.GetMediaType(),
		SchemaVersion: manifest.GetSchemaVersion(),
		CollectedAt:  time.Now(),
	}

	// Handle manifest lists (multi-platform images)
	if manifestList, ok := manifest.(*ManifestList); ok {
		return c.handleManifestList(ctx, client, imageRef, manifestList, facts)
	}

	// Handle regular manifests
	if err := c.populateFactsFromManifest(ctx, client, imageRef, manifest, facts); err != nil {
		return nil, errors.Wrap(err, "failed to populate facts from manifest")
	}

	return facts, nil
}

// handleManifestList handles multi-platform manifest lists
func (c *DefaultImageCollector) handleManifestList(ctx context.Context, client RegistryClient, imageRef ImageReference, manifestList *ManifestList, facts *ImageFacts) (*ImageFacts, error) {
	// Get the best manifest for the default platform
	defaultPlatform := GetDefaultPlatform()
	manifestDesc, err := manifestList.GetManifestForPlatform(defaultPlatform)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find suitable manifest in list")
	}

	// Update facts with platform information
	if manifestDesc.Platform != nil {
		facts.Platform = *manifestDesc.Platform
	}

	// Create a new image reference for the specific manifest
	specificRef := ImageReference{
		Registry:   imageRef.Registry,
		Repository: imageRef.Repository,
		Digest:     manifestDesc.Digest,
	}

	// Get the specific manifest
	specificManifest, err := client.GetManifest(ctx, specificRef)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get specific manifest from list")
	}

	// Populate facts from the specific manifest
	if err := c.populateFactsFromManifest(ctx, client, specificRef, specificManifest, facts); err != nil {
		return nil, errors.Wrap(err, "failed to populate facts from specific manifest")
	}

	return facts, nil
}

// populateFactsFromManifest populates image facts from a manifest
func (c *DefaultImageCollector) populateFactsFromManifest(ctx context.Context, client RegistryClient, imageRef ImageReference, manifest Manifest, facts *ImageFacts) error {
	// Get layers
	layers := manifest.GetLayers()
	facts.Layers = ConvertToLayerInfo(layers)
	
	// Calculate total size from layers
	for _, layer := range facts.Layers {
		facts.Size += layer.Size
	}

	// Get config if available and requested
	configDesc := manifest.GetConfig()
	if configDesc.Digest != "" && c.options.IncludeConfig {
		if err := c.populateConfigFacts(ctx, client, imageRef, configDesc, facts); err != nil {
			klog.V(2).Infof("Failed to get config facts: %v", err)
			// Don't fail entirely if config collection fails
		}
	}

	return nil
}

// populateConfigFacts populates image facts from the config blob
func (c *DefaultImageCollector) populateConfigFacts(ctx context.Context, client RegistryClient, imageRef ImageReference, configDesc Descriptor, facts *ImageFacts) error {
	// Get the config blob
	configReader, err := client.GetBlob(ctx, imageRef, configDesc.Digest)
	if err != nil {
		return errors.Wrap(err, "failed to get config blob")
	}
	defer configReader.Close()

	// Read config data
	configData, err := io.ReadAll(configReader)
	if err != nil {
		return errors.Wrap(err, "failed to read config blob")
	}

	// Parse the config
	configBlob, err := ParseImageConfig(configData)
	if err != nil {
		return errors.Wrap(err, "failed to parse config blob")
	}

	// Update facts with config information
	facts.Platform = ConvertToPlatform(
		configBlob.Architecture, 
		configBlob.OS,
		configBlob.Variant,
		configBlob.OSVersion,
		configBlob.OSFeatures,
	)
	facts.Created = configBlob.Created
	facts.Config = ConvertToImageConfig(configBlob.Config)
	facts.Labels = configBlob.Config.Labels

	return nil
}

// getCredentialsForRegistry returns credentials for a specific registry
func (c *DefaultImageCollector) getCredentialsForRegistry(registry string) RegistryCredentials {
	if c.options.Credentials == nil {
		return RegistryCredentials{}
	}

	// Try exact match first
	if creds, exists := c.options.Credentials[registry]; exists {
		return creds
	}

	// Try without protocol
	registryHost := registry
	if u, err := url.Parse(registry); err == nil {
		registryHost = u.Host
	}

	if creds, exists := c.options.Credentials[registryHost]; exists {
		return creds
	}

	// Try pattern matching for known registries (more conservative)
	for credRegistry, creds := range c.options.Credentials {
		// Only match if the credential registry is a suffix of the actual registry
		// This handles cases like "gcr.io" matching "us.gcr.io"
		if strings.HasSuffix(registry, credRegistry) || strings.HasSuffix(registryHost, credRegistry) {
			return creds
		}
	}

	return RegistryCredentials{}
}

// ParseImageReference parses a full image reference string into components
func ParseImageReference(imageStr string) (ImageReference, error) {
	// Handle digest references (image@sha256:...)
	if strings.Contains(imageStr, "@") {
		parts := strings.SplitN(imageStr, "@", 2)
		if len(parts) != 2 {
			return ImageReference{}, fmt.Errorf("invalid digest reference: %s", imageStr)
		}
		
		repoRef, err := parseRepositoryReference(parts[0])
		if err != nil {
			return ImageReference{}, err
		}
		
		repoRef.Digest = parts[1]
		return repoRef, nil
	}

	// Handle tag references (image:tag)
	return parseRepositoryReference(imageStr)
}

// parseRepositoryReference parses repository and tag from a reference
func parseRepositoryReference(ref string) (ImageReference, error) {
	imageRef := ImageReference{
		Registry: DefaultRegistry,
		Tag:     DefaultTag,
	}

	// Split by ":"
	parts := strings.Split(ref, ":")
	
	if len(parts) == 1 {
		// No tag specified, use default
		imageRef.Repository = ref
	} else if len(parts) == 2 {
		// Simple case: repository:tag
		imageRef.Repository = parts[0]
		imageRef.Tag = parts[1]
	} else {
		// Complex case: might include registry with port
		// Find the last ":" which should be the tag separator
		lastColon := strings.LastIndex(ref, ":")
		imageRef.Repository = ref[:lastColon]
		imageRef.Tag = ref[lastColon+1:]
	}

	// Handle registry in repository
	repoParts := strings.Split(imageRef.Repository, "/")
	if len(repoParts) > 0 {
		// Check if the first part looks like a registry (contains "." or ":")
		firstPart := repoParts[0]
		if strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") {
			imageRef.Registry = firstPart
			imageRef.Repository = strings.Join(repoParts[1:], "/")
		}
	}

	// Handle Docker Hub shorthand
	if imageRef.Registry == DefaultRegistry && !strings.Contains(imageRef.Repository, "/") {
		imageRef.Repository = "library/" + imageRef.Repository
	}

	return imageRef, nil
}

// ExtractImageReferencesFromPodSpec extracts image references from Kubernetes pod specifications
func ExtractImageReferencesFromPodSpec(podSpec interface{}) ([]ImageReference, error) {
	// This would need to be implemented based on the actual Kubernetes types
	// For now, return empty slice
	return []ImageReference{}, nil
}

// CreateFactsBundle creates a facts bundle for serialization
func CreateFactsBundle(namespace string, imageFacts []ImageFacts) *FactsBundle {
	bundle := &FactsBundle{
		Version:     "v1",
		GeneratedAt: time.Now(),
		Namespace:   namespace,
		ImageFacts:  imageFacts,
	}

	// Calculate summary
	registries := make(map[string]bool)
	repositories := make(map[string]bool)
	var totalSize int64
	var errors int

	for _, facts := range imageFacts {
		if facts.Registry != "" {
			registries[facts.Registry] = true
		}
		if facts.Repository != "" {
			repositories[facts.Repository] = true
		}
		totalSize += facts.Size
		if facts.Error != "" {
			errors++
		}
	}

	bundle.Summary = FactsSummary{
		TotalImages:        len(imageFacts),
		UniqueRegistries:   len(registries),
		UniqueRepositories: len(repositories),
		TotalSize:         totalSize,
		CollectionErrors:   errors,
	}

	return bundle
}
