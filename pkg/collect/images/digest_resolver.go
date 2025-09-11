package images

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

// DigestResolver handles conversion of image tags to digests
type DigestResolver struct {
	registryFactory *RegistryClientFactory
	options         CollectionOptions
}

// NewDigestResolver creates a new digest resolver
func NewDigestResolver(options CollectionOptions) *DigestResolver {
	return &DigestResolver{
		registryFactory: NewRegistryClientFactory(options),
		options:         options,
	}
}

// ResolveTagToDigest converts an image tag to its digest
func (dr *DigestResolver) ResolveTagToDigest(ctx context.Context, imageRef ImageReference) (string, error) {
	if imageRef.Digest != "" {
		// Already has digest, return as-is
		return imageRef.Digest, nil
	}

	if imageRef.Tag == "" {
		imageRef.Tag = DefaultTag
	}

	klog.V(3).Infof("Resolving tag to digest: %s:%s", imageRef.Repository, imageRef.Tag)

	// Get registry credentials
	registry := imageRef.Registry
	if registry == "" {
		registry = DefaultRegistry
	}

	credentials := dr.getCredentialsForRegistry(registry)

	// Create registry client
	client, err := dr.registryFactory.CreateClient(registry, credentials)
	if err != nil {
		return "", errors.Wrap(err, "failed to create registry client")
	}

	// Get manifest to extract digest
	manifest, err := client.GetManifest(ctx, imageRef)
	if err != nil {
		return "", errors.Wrap(err, "failed to get manifest for digest resolution")
	}

	// For manifest lists, we need to get a specific manifest
	if manifestList, ok := manifest.(*ManifestList); ok {
		defaultPlatform := GetDefaultPlatform()
		manifestDesc, err := manifestList.GetManifestForPlatform(defaultPlatform)
		if err != nil {
			return "", errors.Wrap(err, "failed to find suitable manifest in list")
		}
		return manifestDesc.Digest, nil
	}

	// For regular manifests, we need the manifest digest
	// This would typically be returned in the Docker-Content-Digest header
	// For now, we'll compute it from the manifest content
	manifestBytes, err := manifest.Marshal()
	if err != nil {
		return "", errors.Wrap(err, "failed to marshal manifest")
	}

	digest, err := ComputeDigest(manifestBytes)
	if err != nil {
		return "", errors.Wrap(err, "failed to compute manifest digest")
	}

	return digest, nil
}

// ResolveBulkTagsToDigests resolves multiple tags to digests concurrently
func (dr *DigestResolver) ResolveBulkTagsToDigests(ctx context.Context, imageRefs []ImageReference) ([]ImageReference, error) {
	klog.V(2).Infof("Resolving %d image tags to digests", len(imageRefs))

	resolved := make([]ImageReference, len(imageRefs))

	for i, ref := range imageRefs {
		if ref.Digest != "" {
			// Already has digest
			resolved[i] = ref
			continue
		}

		digest, err := dr.ResolveTagToDigest(ctx, ref)
		if err != nil {
			klog.Warningf("Failed to resolve digest for %s: %v", ref.String(), err)
			if dr.options.ContinueOnError {
				// Keep original reference
				resolved[i] = ref
				continue
			}
			return nil, errors.Wrapf(err, "failed to resolve digest for %s", ref.String())
		}

		// Create resolved reference
		resolvedRef := ref
		resolvedRef.Digest = digest
		resolved[i] = resolvedRef
	}

	return resolved, nil
}

// ValidateDigest validates that a digest string is properly formatted
func (dr *DigestResolver) ValidateDigest(digest string) error {
	if digest == "" {
		return errors.New("digest cannot be empty")
	}

	// Digest should be in format: algorithm:hex
	parts := strings.SplitN(digest, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid digest format: %s", digest)
	}

	algorithm := parts[0]
	hex := parts[1]

	// Validate algorithm
	validAlgorithms := map[string]int{
		"sha256": 64,
		"sha512": 128,
		"sha1":   40, // deprecated but still seen
	}

	expectedLength, valid := validAlgorithms[algorithm]
	if !valid {
		return fmt.Errorf("unsupported digest algorithm: %s", algorithm)
	}

	// Validate hex length
	if len(hex) != expectedLength {
		return fmt.Errorf("invalid digest hex length for %s: expected %d, got %d",
			algorithm, expectedLength, len(hex))
	}

	// Validate hex characters
	for _, char := range hex {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f') || (char >= 'A' && char <= 'F')) {
			return fmt.Errorf("invalid hex character in digest: %c", char)
		}
	}

	return nil
}

// NormalizeImageReference normalizes an image reference to include defaults
func (dr *DigestResolver) NormalizeImageReference(imageRef ImageReference) ImageReference {
	normalized := imageRef

	// Set default registry
	if normalized.Registry == "" {
		normalized.Registry = DefaultRegistry
	}

	// Set default tag if no tag or digest
	if normalized.Tag == "" && normalized.Digest == "" {
		normalized.Tag = DefaultTag
	}

	// Handle Docker Hub library namespace
	if normalized.Registry == DefaultRegistry && !strings.Contains(normalized.Repository, "/") {
		normalized.Repository = "library/" + normalized.Repository
	}

	return normalized
}

// getCredentialsForRegistry returns credentials for a registry (same as in collector.go)
func (dr *DigestResolver) getCredentialsForRegistry(registry string) RegistryCredentials {
	if dr.options.Credentials == nil {
		return RegistryCredentials{}
	}

	// Try exact match first
	if creds, exists := dr.options.Credentials[registry]; exists {
		return creds
	}

	// Try pattern matching
	for credRegistry, creds := range dr.options.Credentials {
		if strings.Contains(registry, credRegistry) {
			return creds
		}
	}

	return RegistryCredentials{}
}

// ExtractDigestFromManifestResponse extracts digest from HTTP response headers
func ExtractDigestFromManifestResponse(headers map[string][]string) string {
	// Docker registries typically return the digest in the Docker-Content-Digest header
	if digests, exists := headers["Docker-Content-Digest"]; exists && len(digests) > 0 {
		return digests[0]
	}

	// Some registries may use different header names
	if digests, exists := headers["Content-Digest"]; exists && len(digests) > 0 {
		return digests[0]
	}

	return ""
}
