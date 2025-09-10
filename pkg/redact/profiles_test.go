package redact

import (
	"strings"
	"testing"
)

func TestProfileManager(t *testing.T) {
	pm := NewProfileManager()

	// Test that built-in profiles are loaded
	profiles := pm.GetAvailableProfiles()
	expectedProfiles := []string{"minimal", "standard", "comprehensive", "paranoid"}

	for _, expected := range expectedProfiles {
		found := false
		for _, profile := range profiles {
			if profile == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected built-in profile '%s' not found", expected)
		}
	}

	// Test default active profile
	activeProfile, err := pm.GetActiveProfile()
	if err != nil {
		t.Fatalf("Failed to get active profile: %v", err)
	}
	if activeProfile.Name != "standard" {
		t.Errorf("Expected default active profile to be 'standard', got '%s'", activeProfile.Name)
	}
}

func TestBuiltinProfiles(t *testing.T) {
	pm := NewProfileManager()

	tests := []struct {
		profileName       string
		minPatterns       int
		shouldHavePattern string
	}{
		{"minimal", 5, "yaml-password"},
		{"standard", 15, "yaml-secret"},
		{"comprehensive", 25, "yaml-username"},
		{"paranoid", 35, "long-alphanumeric-strings"},
	}

	for _, tt := range tests {
		t.Run(tt.profileName, func(t *testing.T) {
			profile, err := pm.GetProfile(tt.profileName)
			if err != nil {
				t.Fatalf("Failed to get profile '%s': %v", tt.profileName, err)
			}

			if len(profile.Patterns) < tt.minPatterns {
				t.Errorf("Profile '%s' should have at least %d patterns, got %d",
					tt.profileName, tt.minPatterns, len(profile.Patterns))
			}

			// Check for specific pattern
			found := false
			for _, pattern := range profile.Patterns {
				if pattern.Name == tt.shouldHavePattern {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Profile '%s' should have pattern '%s'", tt.profileName, tt.shouldHavePattern)
			}

			// Verify all patterns are enabled by default
			for _, pattern := range profile.Patterns {
				if !pattern.Enabled {
					t.Errorf("Pattern '%s' in profile '%s' should be enabled by default",
						pattern.Name, tt.profileName)
				}
			}
		})
	}
}

func TestProfileValidation(t *testing.T) {
	pm := NewProfileManager()

	tests := []struct {
		name        string
		profile     *RedactionProfile
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid profile",
			profile: &RedactionProfile{
				Name:        "test-profile",
				Description: "Test profile",
				Patterns: []RedactionPattern{
					{
						Name:    "test-pattern",
						Type:    "single-line",
						Enabled: true,
						Regex:   `test-regex`,
						Scan:    `test`,
					},
				},
			},
			shouldError: false,
		},
		{
			name: "empty name",
			profile: &RedactionProfile{
				Name:        "",
				Description: "Test profile",
				Patterns: []RedactionPattern{
					{
						Name:    "test-pattern",
						Type:    "single-line",
						Enabled: true,
						Regex:   `test-regex`,
						Scan:    `test`,
					},
				},
			},
			shouldError: true,
			errorMsg:    "profile name cannot be empty",
		},
		{
			name: "no patterns",
			profile: &RedactionProfile{
				Name:        "test-profile",
				Description: "Test profile",
				Patterns:    []RedactionPattern{},
			},
			shouldError: true,
			errorMsg:    "profile must have at least one pattern",
		},
		{
			name: "invalid pattern type",
			profile: &RedactionProfile{
				Name:        "test-profile",
				Description: "Test profile",
				Patterns: []RedactionPattern{
					{
						Name:    "test-pattern",
						Type:    "invalid-type",
						Enabled: true,
						Regex:   `test-regex`,
					},
				},
			},
			shouldError: true,
			errorMsg:    "invalid pattern type",
		},
		{
			name: "single-line pattern missing regex",
			profile: &RedactionProfile{
				Name:        "test-profile",
				Description: "Test profile",
				Patterns: []RedactionPattern{
					{
						Name:    "test-pattern",
						Type:    "single-line",
						Enabled: true,
					},
				},
			},
			shouldError: true,
			errorMsg:    "single-line pattern must have regex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := pm.ValidateProfile(tt.profile)

			if tt.shouldError {
				if err == nil {
					t.Errorf("Expected validation error, but got none")
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

func TestProfileInheritance(t *testing.T) {
	pm := NewProfileManager()

	// Create a parent profile
	parentProfile := &RedactionProfile{
		Name:        "parent",
		Description: "Parent profile",
		Patterns: []RedactionPattern{
			{
				Name:    "parent-pattern-1",
				Type:    "single-line",
				Enabled: true,
				Regex:   `parent-regex-1`,
				Scan:    `parent1`,
			},
			{
				Name:    "parent-pattern-2",
				Type:    "single-line",
				Enabled: true,
				Regex:   `parent-regex-2`,
				Scan:    `parent2`,
			},
		},
		Metadata: map[string]interface{}{
			"parent-key": "parent-value",
		},
	}

	// Create a child profile that extends the parent
	childProfile := &RedactionProfile{
		Name:        "child",
		Description: "Child profile",
		Extends:     "parent",
		Patterns: []RedactionPattern{
			{
				Name:    "child-pattern-1",
				Type:    "single-line",
				Enabled: true,
				Regex:   `child-regex-1`,
				Scan:    `child1`,
			},
		},
		Metadata: map[string]interface{}{
			"child-key":  "child-value",
			"parent-key": "overridden-value", // Override parent metadata
		},
	}

	// Add profiles to manager
	err := pm.AddCustomProfile(parentProfile)
	if err != nil {
		t.Fatalf("Failed to add parent profile: %v", err)
	}

	err = pm.AddCustomProfile(childProfile)
	if err != nil {
		t.Fatalf("Failed to add child profile: %v", err)
	}

	// Resolve child profile
	resolved, err := pm.ResolveProfile("child")
	if err != nil {
		t.Fatalf("Failed to resolve child profile: %v", err)
	}

	// Check that child has all patterns (parent + child)
	expectedPatterns := 3 // 2 from parent + 1 from child
	if len(resolved.Patterns) != expectedPatterns {
		t.Errorf("Expected %d patterns in resolved profile, got %d", expectedPatterns, len(resolved.Patterns))
	}

	// Check pattern order (parent first, then child)
	if resolved.Patterns[0].Name != "parent-pattern-1" {
		t.Errorf("Expected first pattern to be 'parent-pattern-1', got '%s'", resolved.Patterns[0].Name)
	}
	if resolved.Patterns[2].Name != "child-pattern-1" {
		t.Errorf("Expected third pattern to be 'child-pattern-1', got '%s'", resolved.Patterns[2].Name)
	}

	// Check metadata inheritance and override
	if resolved.Metadata["parent-key"] != "overridden-value" {
		t.Errorf("Expected parent-key to be overridden to 'overridden-value', got '%v'", resolved.Metadata["parent-key"])
	}
	if resolved.Metadata["child-key"] != "child-value" {
		t.Errorf("Expected child-key to be 'child-value', got '%v'", resolved.Metadata["child-key"])
	}
}

func TestProfileSwitching(t *testing.T) {
	pm := NewProfileManager()

	// Test switching to different built-in profiles
	profiles := []string{"minimal", "standard", "comprehensive", "paranoid"}

	for _, profileName := range profiles {
		err := pm.SetActiveProfile(profileName)
		if err != nil {
			t.Errorf("Failed to set active profile to '%s': %v", profileName, err)
		}

		activeProfile, err := pm.GetActiveProfile()
		if err != nil {
			t.Errorf("Failed to get active profile after setting to '%s': %v", profileName, err)
		}

		if activeProfile.Name != profileName {
			t.Errorf("Expected active profile to be '%s', got '%s'", profileName, activeProfile.Name)
		}
	}

	// Test switching to non-existent profile
	err := pm.SetActiveProfile("non-existent")
	if err == nil {
		t.Error("Expected error when setting active profile to non-existent profile")
	}
}

func TestCustomProfileManagement(t *testing.T) {
	pm := NewProfileManager()

	customProfile := &RedactionProfile{
		Name:        "custom-test",
		Description: "Custom test profile",
		Patterns: []RedactionPattern{
			{
				Name:    "custom-pattern",
				Type:    "single-line",
				Enabled: true,
				Regex:   `custom-regex`,
				Scan:    `custom`,
			},
		},
	}

	// Add custom profile
	err := pm.AddCustomProfile(customProfile)
	if err != nil {
		t.Fatalf("Failed to add custom profile: %v", err)
	}

	// Verify it's available
	profiles := pm.GetAvailableProfiles()
	found := false
	for _, profile := range profiles {
		if profile == "custom-test" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Custom profile not found in available profiles")
	}

	// Retrieve and verify custom profile
	retrieved, err := pm.GetProfile("custom-test")
	if err != nil {
		t.Fatalf("Failed to retrieve custom profile: %v", err)
	}

	if retrieved.Name != customProfile.Name {
		t.Errorf("Expected custom profile name '%s', got '%s'", customProfile.Name, retrieved.Name)
	}

	if len(retrieved.Patterns) != len(customProfile.Patterns) {
		t.Errorf("Expected %d patterns in custom profile, got %d", len(customProfile.Patterns), len(retrieved.Patterns))
	}

	// Set as active profile
	err = pm.SetActiveProfile("custom-test")
	if err != nil {
		t.Fatalf("Failed to set custom profile as active: %v", err)
	}

	activeProfile, err := pm.GetActiveProfile()
	if err != nil {
		t.Fatalf("Failed to get active profile: %v", err)
	}

	if activeProfile.Name != "custom-test" {
		t.Errorf("Expected active profile to be 'custom-test', got '%s'", activeProfile.Name)
	}
}
