package redact

import (
	"fmt"
)

// RedactionProfile defines a set of redaction patterns and behaviors
type RedactionProfile struct {
	Name        string                 `json:"name" yaml:"name"`
	Description string                 `json:"description" yaml:"description"`
	Patterns    []RedactionPattern     `json:"patterns" yaml:"patterns"`
	Extends     string                 `json:"extends,omitempty" yaml:"extends,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// RedactionPattern defines a single redaction rule within a profile
type RedactionPattern struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Type        string `json:"type" yaml:"type"` // "single-line", "multi-line", "yaml", "literal"
	Enabled     bool   `json:"enabled" yaml:"enabled"`

	// Pattern configuration
	Regex    string `json:"regex,omitempty" yaml:"regex,omitempty"`
	Scan     string `json:"scan,omitempty" yaml:"scan,omitempty"`
	FilePath string `json:"filePath,omitempty" yaml:"filePath,omitempty"`

	// Multi-line specific
	SelectorRegex string `json:"selectorRegex,omitempty" yaml:"selectorRegex,omitempty"`
	RedactorRegex string `json:"redactorRegex,omitempty" yaml:"redactorRegex,omitempty"`

	// YAML specific
	YamlPath string `json:"yamlPath,omitempty" yaml:"yamlPath,omitempty"`

	// Literal specific
	Match string `json:"match,omitempty" yaml:"match,omitempty"`

	// Metadata
	Severity string                 `json:"severity,omitempty" yaml:"severity,omitempty"` // "low", "medium", "high", "critical"
	Tags     []string               `json:"tags,omitempty" yaml:"tags,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
}

// ProfileLevel represents the built-in profile levels
type ProfileLevel string

const (
	ProfileMinimal       ProfileLevel = "minimal"
	ProfileStandard      ProfileLevel = "standard"
	ProfileComprehensive ProfileLevel = "comprehensive"
	ProfileParanoid      ProfileLevel = "paranoid"
)

// ProfileManager manages redaction profiles
type ProfileManager struct {
	profiles       map[string]*RedactionProfile
	activeProfile  string
	customProfiles map[string]*RedactionProfile
}

// NewProfileManager creates a new profile manager with built-in profiles
func NewProfileManager() *ProfileManager {
	pm := &ProfileManager{
		profiles:       make(map[string]*RedactionProfile),
		customProfiles: make(map[string]*RedactionProfile),
		activeProfile:  string(ProfileStandard), // Default to standard
	}

	// Load built-in profiles
	pm.loadBuiltinProfiles()

	return pm
}

// GetProfile returns a profile by name
func (pm *ProfileManager) GetProfile(name string) (*RedactionProfile, error) {
	// Check custom profiles first
	if profile, exists := pm.customProfiles[name]; exists {
		return profile, nil
	}

	// Check built-in profiles
	if profile, exists := pm.profiles[name]; exists {
		return profile, nil
	}

	return nil, fmt.Errorf("profile '%s' not found", name)
}

// SetActiveProfile sets the active profile
func (pm *ProfileManager) SetActiveProfile(name string) error {
	_, err := pm.GetProfile(name)
	if err != nil {
		return err
	}

	pm.activeProfile = name
	return nil
}

// GetActiveProfile returns the currently active profile
func (pm *ProfileManager) GetActiveProfile() (*RedactionProfile, error) {
	return pm.GetProfile(pm.activeProfile)
}

// AddCustomProfile adds a custom profile
func (pm *ProfileManager) AddCustomProfile(profile *RedactionProfile) error {
	if err := pm.ValidateProfile(profile); err != nil {
		return fmt.Errorf("invalid profile: %w", err)
	}

	pm.customProfiles[profile.Name] = profile
	return nil
}

// ValidateProfile validates a redaction profile
func (pm *ProfileManager) ValidateProfile(profile *RedactionProfile) error {
	if profile.Name == "" {
		return fmt.Errorf("profile name cannot be empty")
	}

	if len(profile.Patterns) == 0 {
		return fmt.Errorf("profile must have at least one pattern")
	}

	// Validate each pattern
	for i, pattern := range profile.Patterns {
		if err := pm.validatePattern(&pattern); err != nil {
			return fmt.Errorf("pattern %d (%s): %w", i, pattern.Name, err)
		}
	}

	// Validate extends relationship
	if profile.Extends != "" {
		if _, err := pm.GetProfile(profile.Extends); err != nil {
			return fmt.Errorf("extended profile '%s' not found", profile.Extends)
		}
	}

	return nil
}

// validatePattern validates a single redaction pattern
func (pm *ProfileManager) validatePattern(pattern *RedactionPattern) error {
	if pattern.Name == "" {
		return fmt.Errorf("pattern name cannot be empty")
	}

	validTypes := map[string]bool{
		"single-line": true,
		"multi-line":  true,
		"yaml":        true,
		"literal":     true,
	}

	if !validTypes[pattern.Type] {
		return fmt.Errorf("invalid pattern type '%s'", pattern.Type)
	}

	// Type-specific validation
	switch pattern.Type {
	case "single-line":
		if pattern.Regex == "" {
			return fmt.Errorf("single-line pattern must have regex")
		}
	case "multi-line":
		if pattern.SelectorRegex == "" || pattern.RedactorRegex == "" {
			return fmt.Errorf("multi-line pattern must have both selectorRegex and redactorRegex")
		}
	case "yaml":
		if pattern.YamlPath == "" {
			return fmt.Errorf("yaml pattern must have yamlPath")
		}
	case "literal":
		if pattern.Match == "" {
			return fmt.Errorf("literal pattern must have match string")
		}
	}

	return nil
}

// ResolveProfile resolves a profile with inheritance
func (pm *ProfileManager) ResolveProfile(name string) (*RedactionProfile, error) {
	profile, err := pm.GetProfile(name)
	if err != nil {
		return nil, err
	}

	// If no inheritance, return as-is
	if profile.Extends == "" {
		return profile, nil
	}

	// Resolve parent profile
	parentProfile, err := pm.ResolveProfile(profile.Extends)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve parent profile '%s': %w", profile.Extends, err)
	}

	// Merge profiles (child overrides parent)
	resolved := &RedactionProfile{
		Name:        profile.Name,
		Description: profile.Description,
		Patterns:    make([]RedactionPattern, 0),
		Metadata:    make(map[string]interface{}),
	}

	// Copy parent metadata
	for k, v := range parentProfile.Metadata {
		resolved.Metadata[k] = v
	}

	// Override with child metadata
	for k, v := range profile.Metadata {
		resolved.Metadata[k] = v
	}

	// Merge patterns (parent first, then child)
	resolved.Patterns = append(resolved.Patterns, parentProfile.Patterns...)
	resolved.Patterns = append(resolved.Patterns, profile.Patterns...)

	return resolved, nil
}

// GetAvailableProfiles returns all available profile names
func (pm *ProfileManager) GetAvailableProfiles() []string {
	var profiles []string

	// Add built-in profiles
	for name := range pm.profiles {
		profiles = append(profiles, name)
	}

	// Add custom profiles
	for name := range pm.customProfiles {
		profiles = append(profiles, name)
	}

	return profiles
}

// loadBuiltinProfiles loads the built-in redaction profiles
func (pm *ProfileManager) loadBuiltinProfiles() {
	// Minimal Profile - Only critical secrets
	pm.profiles[string(ProfileMinimal)] = &RedactionProfile{
		Name:        string(ProfileMinimal),
		Description: "Minimal redaction - only passwords, API keys, and tokens",
		Patterns:    getMinimalPatterns(),
		Metadata: map[string]interface{}{
			"level":    "minimal",
			"builtin":  true,
			"patterns": len(getMinimalPatterns()),
		},
	}

	// Standard Profile - Common secrets
	pm.profiles[string(ProfileStandard)] = &RedactionProfile{
		Name:        string(ProfileStandard),
		Description: "Standard redaction - passwords, keys, tokens, IPs, URLs, emails",
		Patterns:    getStandardPatterns(),
		Metadata: map[string]interface{}{
			"level":    "standard",
			"builtin":  true,
			"patterns": len(getStandardPatterns()),
		},
	}

	// Comprehensive Profile - Extensive redaction
	pm.profiles[string(ProfileComprehensive)] = &RedactionProfile{
		Name:        string(ProfileComprehensive),
		Description: "Comprehensive redaction - includes usernames, hostnames, file paths",
		Patterns:    getComprehensivePatterns(),
		Metadata: map[string]interface{}{
			"level":    "comprehensive",
			"builtin":  true,
			"patterns": len(getComprehensivePatterns()),
		},
	}

	// Paranoid Profile - Maximum redaction
	pm.profiles[string(ProfileParanoid)] = &RedactionProfile{
		Name:        string(ProfileParanoid),
		Description: "Paranoid redaction - redacts any potentially sensitive data",
		Patterns:    getParanoidPatterns(),
		Metadata: map[string]interface{}{
			"level":    "paranoid",
			"builtin":  true,
			"patterns": len(getParanoidPatterns()),
		},
	}
}

// Global profile manager instance
var globalProfileManager *ProfileManager

// GetProfileManager returns the global profile manager
func GetProfileManager() *ProfileManager {
	if globalProfileManager == nil {
		globalProfileManager = NewProfileManager()
	}
	return globalProfileManager
}

// SetRedactionProfile sets the active redaction profile
func SetRedactionProfile(profileName string) error {
	pm := GetProfileManager()
	return pm.SetActiveProfile(profileName)
}

// GetRedactionProfile returns the active redaction profile
func GetRedactionProfile() (*RedactionProfile, error) {
	pm := GetProfileManager()
	return pm.GetActiveProfile()
}
