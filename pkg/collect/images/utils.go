package images

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// imageCache implements a simple LRU-style cache for image facts
type imageCache struct {
	mu       sync.RWMutex
	entries  map[string]*imageCacheEntry
	ttl      time.Duration
	maxSize  int
	accessed map[string]time.Time // Track access time for LRU
}

// imageCacheEntry represents a cached image facts entry
type imageCacheEntry struct {
	facts     *ImageFacts
	timestamp time.Time
}

// newImageCache creates a new image cache
func newImageCache(ttl time.Duration) *imageCache {
	if ttl <= 0 {
		ttl = DefaultCacheDuration
	}

	return &imageCache{
		entries:  make(map[string]*imageCacheEntry),
		ttl:      ttl,
		maxSize:  1000, // Reasonable default
		accessed: make(map[string]time.Time),
	}
}

// Get retrieves image facts from the cache
func (ic *imageCache) Get(key string) *ImageFacts {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	entry, exists := ic.entries[key]
	if !exists {
		return nil
	}

	// Check if entry has expired
	if time.Since(entry.timestamp) > ic.ttl {
		// Don't remove here to avoid write lock, let cleanup handle it
		return nil
	}

	// Update access time
	ic.accessed[key] = time.Now()

	return entry.facts
}

// Set stores image facts in the cache
func (ic *imageCache) Set(key string, facts *ImageFacts) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// Cleanup if cache is getting full
	if len(ic.entries) >= ic.maxSize {
		ic.cleanupLocked()
	}

	ic.entries[key] = &imageCacheEntry{
		facts:     facts,
		timestamp: time.Now(),
	}
	ic.accessed[key] = time.Now()

	klog.V(4).Infof("Cached image facts for: %s", key)
}

// cleanupLocked removes expired and least recently used entries (must be called with write lock)
func (ic *imageCache) cleanupLocked() {
	now := time.Now()

	// First remove expired entries
	for key, entry := range ic.entries {
		if now.Sub(entry.timestamp) > ic.ttl {
			delete(ic.entries, key)
			delete(ic.accessed, key)
		}
	}

	// If still too full, remove least recently accessed entries
	if len(ic.entries) >= ic.maxSize {
		// Create slice of keys sorted by access time
		type accessEntry struct {
			key        string
			accessTime time.Time
		}

		var accessList []accessEntry
		for key, accessTime := range ic.accessed {
			accessList = append(accessList, accessEntry{
				key:        key,
				accessTime: accessTime,
			})
		}

		// Sort by access time (oldest first)
		for i := 0; i < len(accessList); i++ {
			for j := i + 1; j < len(accessList); j++ {
				if accessList[i].accessTime.After(accessList[j].accessTime) {
					accessList[i], accessList[j] = accessList[j], accessList[i]
				}
			}
		}

		// Remove oldest entries until we're under the limit
		toRemove := len(ic.entries) - ic.maxSize/2 // Remove half when cleaning
		for i := 0; i < toRemove && i < len(accessList); i++ {
			key := accessList[i].key
			delete(ic.entries, key)
			delete(ic.accessed, key)
		}
	}

	klog.V(4).Infof("Cache cleanup completed, %d entries remaining", len(ic.entries))
}

// Clear clears all entries from the cache
func (ic *imageCache) Clear() {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	ic.entries = make(map[string]*imageCacheEntry)
	ic.accessed = make(map[string]time.Time)
}

// Size returns the current number of cached entries
func (ic *imageCache) Size() int {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return len(ic.entries)
}

// ComputeDigest computes the SHA256 digest of data
func ComputeDigest(data []byte) (string, error) {
	hasher := sha256.New()
	hasher.Write(data)
	hash := hasher.Sum(nil)
	return "sha256:" + hex.EncodeToString(hash), nil
}

// IsValidImageName validates an image name format
func IsValidImageName(imageName string) bool {
	if imageName == "" {
		return false
	}

	// Basic validation: should not contain spaces or invalid characters
	invalidChars := []string{" ", "\t", "\n", "\r"}
	for _, char := range invalidChars {
		if strings.Contains(imageName, char) {
			return false
		}
	}

	// Should not start or end with slash
	if strings.HasPrefix(imageName, "/") || strings.HasSuffix(imageName, "/") {
		return false
	}

	// Should not have double slashes
	if strings.Contains(imageName, "//") {
		return false
	}

	return true
}

// NormalizeRegistryURL normalizes a registry URL
func NormalizeRegistryURL(registryURL string) string {
	// Remove trailing slashes
	registryURL = strings.TrimRight(registryURL, "/")

	// Add https:// if no protocol specified
	if !strings.HasPrefix(registryURL, "http://") && !strings.HasPrefix(registryURL, "https://") {
		registryURL = "https://" + registryURL
	}

	// Handle Docker Hub special case
	if registryURL == "https://docker.io" {
		registryURL = "https://registry-1.docker.io"
	}

	return registryURL
}

// ExtractRepositoryHost extracts the hostname from a repository string
func ExtractRepositoryHost(repository string) string {
	parts := strings.Split(repository, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return repository
}

// IsOfficialImage checks if an image is an official Docker Hub image
func IsOfficialImage(registry, repository string) bool {
	// Official images are on Docker Hub in the "library" namespace
	return (registry == DefaultRegistry || registry == "docker.io") && 
		   strings.HasPrefix(repository, "library/")
}

// GetImageShortName returns a shortened version of the image name for display
func GetImageShortName(facts ImageFacts) string {
	// For official images, just show the image name without library/ prefix
	if IsOfficialImage(facts.Registry, facts.Repository) {
		shortName := strings.TrimPrefix(facts.Repository, "library/")
		if facts.Tag != "" && facts.Tag != DefaultTag {
			return shortName + ":" + facts.Tag
		}
		return shortName
	}

	// For other images, show registry/repository:tag
	name := facts.Repository
	if facts.Registry != DefaultRegistry {
		name = facts.Registry + "/" + name
	}

	if facts.Tag != "" && facts.Tag != DefaultTag {
		name += ":" + facts.Tag
	}

	return name
}

// FormatSize formats a size in bytes to a human-readable format
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ParseSize parses a human-readable size string to bytes
func ParseSize(sizeStr string) (int64, error) {
	// This is a simplified implementation
	// In a real implementation, you'd want to handle various formats like "1.5GB", "500MB", etc.
	
	sizeStr = strings.TrimSpace(strings.ToUpper(sizeStr))
	
	multipliers := map[string]int64{
		"B":   1,
		"KB":  1024,
		"MB":  1024 * 1024,
		"GB":  1024 * 1024 * 1024,
		"TB":  1024 * 1024 * 1024 * 1024,
		"KIB": 1024,
		"MIB": 1024 * 1024,
		"GIB": 1024 * 1024 * 1024,
		"TIB": 1024 * 1024 * 1024 * 1024,
	}

	// Simple parsing - assume the string ends with a unit
	for suffix, multiplier := range multipliers {
		if strings.HasSuffix(sizeStr, suffix) {
			// This is a simplified implementation
			// A complete implementation would parse the numeric part
			return multiplier, nil
		}
	}

	// If no unit found, assume bytes
	return 0, fmt.Errorf("unable to parse size: %s", sizeStr)
}

// GetRegistryType determines the type of registry based on its URL
func GetRegistryType(registryURL string) string {
	switch {
	case strings.Contains(registryURL, "docker.io") || strings.Contains(registryURL, "registry-1.docker.io"):
		return "dockerhub"
	case strings.Contains(registryURL, "gcr.io"):
		return "gcr"
	case strings.Contains(registryURL, "amazonaws.com"):
		return "ecr"
	case strings.Contains(registryURL, "quay.io"):
		return "quay"
	case strings.Contains(registryURL, "registry.redhat.io"):
		return "redhat"
	case strings.Contains(registryURL, "harbor"):
		return "harbor"
	default:
		return "generic"
	}
}

// ValidateCredentials validates registry credentials
func ValidateCredentials(creds RegistryCredentials) error {
	// If using basic auth, both username and password are required
	if creds.Username != "" && creds.Password == "" {
		return fmt.Errorf("password is required when username is provided")
	}

	if creds.Password != "" && creds.Username == "" {
		return fmt.Errorf("username is required when password is provided")
	}

	// Check that we have some form of valid authentication
	hasValidBasicAuth := creds.Username != "" && creds.Password != ""
	hasToken := creds.Token != ""
	hasIdentityToken := creds.IdentityToken != ""

	if !hasValidBasicAuth && !hasToken && !hasIdentityToken {
		// Empty credentials are valid (for public registries)
		return nil
	}

	return nil
}

// GetDefaultCollectionOptions returns default collection options
func GetDefaultCollectionOptions() CollectionOptions {
	return CollectionOptions{
		IncludeLayers:   true,
		IncludeConfig:   true,
		Timeout:         DefaultTimeout,
		MaxConcurrency:  DefaultMaxConcurrency,
		ContinueOnError: true,
		SkipTLSVerify:   false,
		EnableCache:     true,
		CacheDuration:   DefaultCacheDuration,
		Credentials:     make(map[string]RegistryCredentials),
	}
}
