package cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"k8s.io/klog/v2"
)

// DiscoveryConfig represents the configuration for auto-discovery
type DiscoveryConfig struct {
	Version  string                      `json:"version" yaml:"version"`
	Profiles map[string]DiscoveryProfile `json:"profiles" yaml:"profiles"`
	Patterns DiscoveryPatterns           `json:"patterns" yaml:"patterns"`
}

// DiscoveryPatterns defines inclusion/exclusion patterns for discovery
type DiscoveryPatterns struct {
	NamespacePatterns    PatternConfig `json:"namespacePatterns" yaml:"namespacePatterns"`
	ResourceTypePatterns PatternConfig `json:"resourceTypePatterns" yaml:"resourceTypePatterns"`
	RegistryPatterns     PatternConfig `json:"registryPatterns" yaml:"registryPatterns"`
}

// PatternConfig defines include/exclude patterns
type PatternConfig struct {
	Include []string `json:"include" yaml:"include"`
	Exclude []string `json:"exclude" yaml:"exclude"`
}

// LoadDiscoveryConfig loads discovery configuration from file or returns defaults
func LoadDiscoveryConfig(configPath string) (*DiscoveryConfig, error) {
	// If no config path specified, use built-in defaults
	if configPath == "" {
		return getDefaultDiscoveryConfig(), nil
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		klog.V(2).Infof("Discovery config file not found: %s, using defaults", configPath)
		return getDefaultDiscoveryConfig(), nil
	}

	// Read config file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read discovery config file")
	}

	// Parse config file (support JSON for now)
	var config DiscoveryConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrap(err, "failed to parse discovery config")
	}

	// Validate config
	if err := validateDiscoveryConfig(&config); err != nil {
		return nil, errors.Wrap(err, "invalid discovery config")
	}

	return &config, nil
}

// getDefaultDiscoveryConfig returns the built-in default configuration
func getDefaultDiscoveryConfig() *DiscoveryConfig {
	return &DiscoveryConfig{
		Version:  "v1",
		Profiles: GetDiscoveryProfiles(),
		Patterns: DiscoveryPatterns{
			NamespacePatterns: PatternConfig{
				Include: []string{"*"}, // Include all by default
				Exclude: []string{
					"kube-system",
					"kube-public",
					"kube-node-lease",
					"kubernetes-dashboard",
					"cattle-*",
					"rancher-*",
				},
			},
			ResourceTypePatterns: PatternConfig{
				Include: []string{
					"pods",
					"deployments",
					"services",
					"configmaps",
					"secrets",
					"events",
				},
				Exclude: []string{
					"*.tmp",
					"*.log", // Exclude raw log files in favor of structured logs
				},
			},
			RegistryPatterns: PatternConfig{
				Include: []string{"*"}, // Include all registries
				Exclude: []string{},    // No exclusions by default
			},
		},
	}
}

// validateDiscoveryConfig validates a discovery configuration
func validateDiscoveryConfig(config *DiscoveryConfig) error {
	if config.Version == "" {
		config.Version = "v1" // Default version
	}

	if config.Profiles == nil {
		return errors.New("profiles section is required")
	}

	// Validate each profile
	requiredProfiles := []string{"minimal", "standard", "comprehensive"}
	for _, profileName := range requiredProfiles {
		if _, exists := config.Profiles[profileName]; !exists {
			return fmt.Errorf("required profile missing: %s", profileName)
		}
	}

	return nil
}

// ApplyDiscoveryPatterns applies include/exclude patterns to a list
func ApplyDiscoveryPatterns(items []string, patterns PatternConfig) ([]string, error) {
	if len(patterns.Include) == 0 && len(patterns.Exclude) == 0 {
		return items, nil // No patterns to apply
	}

	var result []string

	for _, item := range items {
		include := true

		// Check exclude patterns first
		for _, excludePattern := range patterns.Exclude {
			if matched, err := matchPattern(item, excludePattern); err != nil {
				return nil, errors.Wrapf(err, "invalid exclude pattern: %s", excludePattern)
			} else if matched {
				include = false
				break
			}
		}

		// If not excluded, check include patterns
		if include && len(patterns.Include) > 0 {
			include = false // Default to exclude if include patterns exist
			for _, includePattern := range patterns.Include {
				if matched, err := matchPattern(item, includePattern); err != nil {
					return nil, errors.Wrapf(err, "invalid include pattern: %s", includePattern)
				} else if matched {
					include = true
					break
				}
			}
		}

		if include {
			result = append(result, item)
		}
	}

	return result, nil
}

// matchPattern checks if an item matches a glob pattern
func matchPattern(item, pattern string) (bool, error) {
	// Simple glob pattern matching
	if pattern == "*" {
		return true, nil
	}

	if pattern == item {
		return true, nil
	}

	// Handle basic wildcard patterns
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		// Pattern is "*substring*"
		substring := pattern[1 : len(pattern)-1]
		return strings.Contains(item, substring), nil
	}

	if strings.HasPrefix(pattern, "*") {
		// Pattern is "*suffix"
		suffix := pattern[1:]
		return strings.HasSuffix(item, suffix), nil
	}

	if strings.HasSuffix(pattern, "*") {
		// Pattern is "prefix*"
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(item, prefix), nil
	}

	return false, nil
}

// SaveDiscoveryConfig saves discovery configuration to a file
func SaveDiscoveryConfig(config *DiscoveryConfig, configPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return errors.Wrap(err, "failed to create config directory")
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal config")
	}

	// Write to file
	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return errors.Wrap(err, "failed to write config file")
	}

	return nil
}

// GetDiscoveryConfigPath returns the default path for discovery configuration
func GetDiscoveryConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "./troubleshoot-discovery.json"
	}

	return filepath.Join(homeDir, ".troubleshoot", "discovery.json")
}

// CreateDefaultDiscoveryConfigFile creates a default discovery config file
func CreateDefaultDiscoveryConfigFile(configPath string) error {
	config := getDefaultDiscoveryConfig()
	return SaveDiscoveryConfig(config, configPath)
}
