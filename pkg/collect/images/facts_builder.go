package images

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

// FactsBuilder creates and manages image facts
type FactsBuilder struct {
	collector *DefaultImageCollector
	resolver  *DigestResolver
	options   CollectionOptions
}

// NewFactsBuilder creates a new facts builder
func NewFactsBuilder(options CollectionOptions) *FactsBuilder {
	return &FactsBuilder{
		collector: NewImageCollector(options),
		resolver:  NewDigestResolver(options),
		options:   options,
	}
}

// BuildFactsFromImageReferences builds image facts from a list of image references
func (fb *FactsBuilder) BuildFactsFromImageReferences(ctx context.Context, imageRefs []ImageReference, source string) ([]ImageFacts, error) {
	if len(imageRefs) == 0 {
		return []ImageFacts{}, nil
	}

	klog.V(2).Infof("Building facts for %d images from source: %s", len(imageRefs), source)

	// Normalize image references
	normalizedRefs := make([]ImageReference, len(imageRefs))
	for i, ref := range imageRefs {
		normalizedRefs[i] = fb.resolver.NormalizeImageReference(ref)
	}

	// Resolve tags to digests if needed
	resolvedRefs, err := fb.resolver.ResolveBulkTagsToDigests(ctx, normalizedRefs)
	if err != nil {
		if !fb.options.ContinueOnError {
			return nil, errors.Wrap(err, "failed to resolve image digests")
		}
		klog.Warningf("Some digest resolutions failed: %v", err)
		// Use original refs if resolution fails
		resolvedRefs = normalizedRefs
	}

	// Collect image facts
	imageFacts, err := fb.collector.CollectMultipleImageFacts(ctx, resolvedRefs)
	if err != nil && !fb.options.ContinueOnError {
		return nil, errors.Wrap(err, "failed to collect image facts")
	}

	// Set source for all facts
	for i := range imageFacts {
		imageFacts[i].Source = source
	}

	return imageFacts, nil
}

// BuildFactsFromImageStrings builds image facts from string representations
func (fb *FactsBuilder) BuildFactsFromImageStrings(ctx context.Context, imageStrs []string, source string) ([]ImageFacts, error) {
	if len(imageStrs) == 0 {
		return []ImageFacts{}, nil
	}

	// Parse image references
	var imageRefs []ImageReference
	var parseErrors []error

	for _, imageStr := range imageStrs {
		ref, err := ParseImageReference(imageStr)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Errorf("image %s: %w", imageStr, err))
			if fb.options.ContinueOnError {
				continue
			}
			return nil, fmt.Errorf("failed to parse image references: %v", parseErrors)
		}
		imageRefs = append(imageRefs, ref)
	}

	if len(parseErrors) > 0 {
		klog.Warningf("Image parsing errors: %v", parseErrors)
	}

	return fb.BuildFactsFromImageReferences(ctx, imageRefs, source)
}

// DeduplicateImageFacts removes duplicate image facts based on digest
func (fb *FactsBuilder) DeduplicateImageFacts(imageFacts []ImageFacts) []ImageFacts {
	if len(imageFacts) <= 1 {
		return imageFacts
	}

	// Use digest as deduplication key, fallback to repository:tag
	seen := make(map[string]bool)
	var deduplicated []ImageFacts

	for _, facts := range imageFacts {
		var key string
		if facts.Digest != "" {
			key = facts.Digest
		} else {
			key = fmt.Sprintf("%s/%s:%s", facts.Registry, facts.Repository, facts.Tag)
		}

		if !seen[key] {
			seen[key] = true
			deduplicated = append(deduplicated, facts)
		} else {
			klog.V(4).Infof("Duplicate image facts filtered: %s", key)
		}
	}

	klog.V(3).Infof("Deduplicated %d image facts to %d unique images",
		len(imageFacts), len(deduplicated))

	return deduplicated
}

// SortImageFactsBySize sorts image facts by size (largest first)
func (fb *FactsBuilder) SortImageFactsBySize(imageFacts []ImageFacts) {
	sort.Slice(imageFacts, func(i, j int) bool {
		return imageFacts[i].Size > imageFacts[j].Size
	})
}

// SortImageFactsByName sorts image facts by repository name
func (fb *FactsBuilder) SortImageFactsByName(imageFacts []ImageFacts) {
	sort.Slice(imageFacts, func(i, j int) bool {
		return imageFacts[i].Repository < imageFacts[j].Repository
	})
}

// FilterImageFactsByRegistry filters image facts to only include specific registries
func (fb *FactsBuilder) FilterImageFactsByRegistry(imageFacts []ImageFacts, allowedRegistries []string) []ImageFacts {
	if len(allowedRegistries) == 0 {
		return imageFacts
	}

	registrySet := make(map[string]bool)
	for _, registry := range allowedRegistries {
		registrySet[registry] = true
	}

	var filtered []ImageFacts
	for _, facts := range imageFacts {
		if registrySet[facts.Registry] {
			filtered = append(filtered, facts)
		}
	}

	return filtered
}

// ValidateImageFacts validates that image facts are complete and consistent
func (fb *FactsBuilder) ValidateImageFacts(facts ImageFacts) error {
	var validationErrors []string

	// Required fields
	if facts.Repository == "" {
		validationErrors = append(validationErrors, "repository is required")
	}

	if facts.Registry == "" {
		validationErrors = append(validationErrors, "registry is required")
	}

	// Either tag or digest should be present
	if facts.Tag == "" && facts.Digest == "" {
		validationErrors = append(validationErrors, "either tag or digest is required")
	}

	// Validate digest format if present
	if facts.Digest != "" {
		if err := fb.resolver.ValidateDigest(facts.Digest); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("invalid digest: %v", err))
		}
	}

	// Validate platform
	if facts.Platform.Architecture == "" {
		validationErrors = append(validationErrors, "platform architecture is required")
	}

	if facts.Platform.OS == "" {
		validationErrors = append(validationErrors, "platform OS is required")
	}

	// Validate size
	if facts.Size < 0 {
		validationErrors = append(validationErrors, "size cannot be negative")
	}

	// Check collection timestamp
	if facts.CollectedAt.IsZero() {
		validationErrors = append(validationErrors, "collectedAt timestamp is required")
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("image facts validation failed: %s", strings.Join(validationErrors, "; "))
	}

	return nil
}

// SerializeFactsToJSON serializes image facts to JSON
func (fb *FactsBuilder) SerializeFactsToJSON(imageFacts []ImageFacts, namespace string) ([]byte, error) {
	bundle := CreateFactsBundle(namespace, imageFacts)

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal facts bundle")
	}

	return data, nil
}

// DeserializeFactsFromJSON deserializes image facts from JSON
func (fb *FactsBuilder) DeserializeFactsFromJSON(data []byte) (*FactsBundle, error) {
	var bundle FactsBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal facts bundle")
	}

	// Validate the bundle
	if bundle.Version == "" {
		bundle.Version = "v1" // Default version
	}

	// Validate each image facts entry
	for i, facts := range bundle.ImageFacts {
		if err := fb.ValidateImageFacts(facts); err != nil {
			klog.Warningf("Invalid image facts at index %d: %v", i, err)
			// Note: we continue with invalid facts but log the issue
		}
	}

	return &bundle, nil
}

// GetImageFactsSummary generates a summary of image facts
func (fb *FactsBuilder) GetImageFactsSummary(imageFacts []ImageFacts) FactsSummary {
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

	return FactsSummary{
		TotalImages:        len(imageFacts),
		UniqueRegistries:   len(registries),
		UniqueRepositories: len(repositories),
		TotalSize:          totalSize,
		CollectionErrors:   errors,
	}
}

// ExtractUniqueImages extracts unique images from image facts
func (fb *FactsBuilder) ExtractUniqueImages(imageFacts []ImageFacts) []string {
	seen := make(map[string]bool)
	var unique []string

	for _, facts := range imageFacts {
		imageStr := ""
		if facts.Digest != "" {
			imageStr = fmt.Sprintf("%s/%s@%s", facts.Registry, facts.Repository, facts.Digest)
		} else {
			imageStr = fmt.Sprintf("%s/%s:%s", facts.Registry, facts.Repository, facts.Tag)
		}

		if !seen[imageStr] {
			seen[imageStr] = true
			unique = append(unique, imageStr)
		}
	}

	sort.Strings(unique)
	return unique
}

// GetLargestImages returns the N largest images by size
func (fb *FactsBuilder) GetLargestImages(imageFacts []ImageFacts, count int) []ImageFacts {
	if count <= 0 || len(imageFacts) == 0 {
		return []ImageFacts{}
	}

	// Make a copy and sort by size
	factsCopy := make([]ImageFacts, len(imageFacts))
	copy(factsCopy, imageFacts)

	fb.SortImageFactsBySize(factsCopy)

	if count > len(factsCopy) {
		count = len(factsCopy)
	}

	return factsCopy[:count]
}

// GetImagesByRegistry groups images by registry
func (fb *FactsBuilder) GetImagesByRegistry(imageFacts []ImageFacts) map[string][]ImageFacts {
	registryMap := make(map[string][]ImageFacts)

	for _, facts := range imageFacts {
		registry := facts.Registry
		if registry == "" {
			registry = "unknown"
		}
		registryMap[registry] = append(registryMap[registry], facts)
	}

	return registryMap
}

// GetFailedCollections returns image facts with collection errors
func (fb *FactsBuilder) GetFailedCollections(imageFacts []ImageFacts) []ImageFacts {
	var failed []ImageFacts
	for _, facts := range imageFacts {
		if facts.Error != "" {
			failed = append(failed, facts)
		}
	}
	return failed
}

// GetSuccessfulCollections returns image facts without collection errors
func (fb *FactsBuilder) GetSuccessfulCollections(imageFacts []ImageFacts) []ImageFacts {
	var successful []ImageFacts
	for _, facts := range imageFacts {
		if facts.Error == "" {
			successful = append(successful, facts)
		}
	}
	return successful
}
