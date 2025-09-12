package cli

import (
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect/autodiscovery"
	"github.com/replicatedhq/troubleshoot/pkg/collect/images"
	"github.com/spf13/viper"
)

func TestGetAutoDiscoveryConfig(t *testing.T) {
	tests := []struct {
		name        string
		viperSetup  func(*viper.Viper)
		wantEnabled bool
		wantImages  bool
		wantRBAC    bool
		wantProfile string
	}{
		{
			name: "default config",
			viperSetup: func(v *viper.Viper) {
				// No flags set, should use defaults
			},
			wantEnabled: false,
			wantImages:  false,
			wantRBAC:    true, // Default is true
			wantProfile: "standard",
		},
		{
			name: "auto enabled",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", true)
			},
			wantEnabled: true,
			wantImages:  false,
			wantRBAC:    true,
			wantProfile: "standard",
		},
		{
			name: "auto with images",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", true)
				v.Set("include-images", true)
			},
			wantEnabled: true,
			wantImages:  true,
			wantRBAC:    true,
			wantProfile: "standard",
		},
		{
			name: "comprehensive profile",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", true)
				v.Set("discovery-profile", "comprehensive")
			},
			wantEnabled: true,
			wantImages:  false,
			wantRBAC:    true,
			wantProfile: "comprehensive",
		},
		{
			name: "rbac disabled",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", true)
				v.Set("rbac-check", false)
			},
			wantEnabled: true,
			wantImages:  false,
			wantRBAC:    false,
			wantProfile: "standard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()

			// Set defaults
			v.SetDefault("rbac-check", true)
			v.SetDefault("discovery-profile", "standard")

			// Apply test-specific setup
			tt.viperSetup(v)

			config := GetAutoDiscoveryConfig(v)

			if config.Enabled != tt.wantEnabled {
				t.Errorf("GetAutoDiscoveryConfig() enabled = %v, want %v", config.Enabled, tt.wantEnabled)
			}
			if config.IncludeImages != tt.wantImages {
				t.Errorf("GetAutoDiscoveryConfig() includeImages = %v, want %v", config.IncludeImages, tt.wantImages)
			}
			if config.RBACCheck != tt.wantRBAC {
				t.Errorf("GetAutoDiscoveryConfig() rbacCheck = %v, want %v", config.RBACCheck, tt.wantRBAC)
			}
			if config.Profile != tt.wantProfile {
				t.Errorf("GetAutoDiscoveryConfig() profile = %v, want %v", config.Profile, tt.wantProfile)
			}
		})
	}
}

func TestGetDiscoveryProfiles(t *testing.T) {
	profiles := GetDiscoveryProfiles()

	requiredProfiles := []string{"minimal", "standard", "comprehensive", "paranoid"}
	for _, profileName := range requiredProfiles {
		if profile, exists := profiles[profileName]; !exists {
			t.Errorf("Missing required discovery profile: %s", profileName)
		} else {
			if profile.Name != profileName {
				t.Errorf("Profile %s has wrong name: %s", profileName, profile.Name)
			}
			if profile.Description == "" {
				t.Errorf("Profile %s missing description", profileName)
			}
			if profile.Timeout <= 0 {
				t.Errorf("Profile %s has invalid timeout: %v", profileName, profile.Timeout)
			}
		}
	}

	// Check profile progression (more features as we go up)
	if profiles["comprehensive"].IncludeImages && !profiles["paranoid"].IncludeImages {
		t.Error("Paranoid profile should include at least everything comprehensive does")
	}
}

func TestValidateAutoDiscoveryFlags(t *testing.T) {
	tests := []struct {
		name       string
		viperSetup func(*viper.Viper)
		wantErr    bool
	}{
		{
			name: "valid auto discovery",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", true)
				v.Set("include-images", true)
				v.Set("discovery-profile", "standard")
			},
			wantErr: false,
		},
		{
			name: "include-images without auto",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", false)
				v.Set("include-images", true)
			},
			wantErr: true,
		},
		{
			name: "invalid discovery profile",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", true)
				v.Set("discovery-profile", "invalid-profile")
			},
			wantErr: true,
		},
		{
			name: "no auto discovery",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", false)
			},
			wantErr: false,
		},
		{
			name: "both include and exclude namespaces",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", true)
				v.Set("include-namespaces", []string{"app1"})
				v.Set("exclude-namespaces", []string{"system"})
			},
			wantErr: false, // Should warn but not error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()

			// Set defaults
			v.SetDefault("rbac-check", true)
			v.SetDefault("discovery-profile", "standard")

			// Apply test setup
			tt.viperSetup(v)

			err := ValidateAutoDiscoveryFlags(v)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAutoDiscoveryFlags() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestShouldUseAutoDiscovery(t *testing.T) {
	tests := []struct {
		name       string
		viperSetup func(*viper.Viper)
		args       []string
		want       bool
	}{
		{
			name: "auto flag enabled",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", true)
			},
			args: []string{},
			want: true,
		},
		{
			name: "auto flag disabled",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", false)
			},
			args: []string{},
			want: false,
		},
		{
			name: "auto with yaml args",
			viperSetup: func(v *viper.Viper) {
				v.Set("auto", true)
			},
			args: []string{"spec.yaml"},
			want: true,
		},
		{
			name: "no auto flag",
			viperSetup: func(v *viper.Viper) {
				// No auto flag set
			},
			args: []string{"spec.yaml"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			tt.viperSetup(v)

			got := ShouldUseAutoDiscovery(v, tt.args)
			if got != tt.want {
				t.Errorf("ShouldUseAutoDiscovery() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAutoDiscoveryMode(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		autoEnabled bool
		want        string
	}{
		{
			name:        "foundational only",
			args:        []string{},
			autoEnabled: true,
			want:        "foundational-only",
		},
		{
			name:        "yaml augmented",
			args:        []string{"spec.yaml"},
			autoEnabled: true,
			want:        "yaml-augmented",
		},
		{
			name:        "disabled",
			args:        []string{},
			autoEnabled: false,
			want:        "disabled",
		},
		{
			name:        "disabled with args",
			args:        []string{"spec.yaml"},
			autoEnabled: false,
			want:        "disabled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAutoDiscoveryMode(tt.args, tt.autoEnabled)
			if got != tt.want {
				t.Errorf("GetAutoDiscoveryMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateImageCollectionOptions(t *testing.T) {
	tests := []struct {
		name      string
		config    AutoDiscoveryConfig
		checkFunc func(t *testing.T, opts images.CollectionOptions)
	}{
		{
			name: "standard profile",
			config: AutoDiscoveryConfig{
				Profile: "standard",
			},
			checkFunc: func(t *testing.T, opts images.CollectionOptions) {
				if opts.Timeout != 30*time.Second {
					t.Errorf("Expected standard profile timeout 30s, got %v", opts.Timeout)
				}
				if !opts.ContinueOnError {
					t.Error("Should continue on error for auto-discovery")
				}
			},
		},
		{
			name: "comprehensive profile",
			config: AutoDiscoveryConfig{
				Profile: "comprehensive",
			},
			checkFunc: func(t *testing.T, opts images.CollectionOptions) {
				if opts.Timeout != 60*time.Second {
					t.Errorf("Expected comprehensive profile timeout 60s, got %v", opts.Timeout)
				}
				if !opts.IncludeConfig {
					t.Error("Comprehensive profile should include config")
				}
			},
		},
		{
			name: "paranoid profile",
			config: AutoDiscoveryConfig{
				Profile: "paranoid",
			},
			checkFunc: func(t *testing.T, opts images.CollectionOptions) {
				if opts.Timeout != 120*time.Second {
					t.Errorf("Expected paranoid profile timeout 120s, got %v", opts.Timeout)
				}
				if !opts.IncludeLayers {
					t.Error("Paranoid profile should include layers")
				}
			},
		},
		{
			name: "custom timeout",
			config: AutoDiscoveryConfig{
				Profile: "standard",
				Timeout: 45 * time.Second,
			},
			checkFunc: func(t *testing.T, opts images.CollectionOptions) {
				if opts.Timeout != 45*time.Second {
					t.Errorf("Expected custom timeout 45s, got %v", opts.Timeout)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := CreateImageCollectionOptions(tt.config)
			tt.checkFunc(t, opts)
		})
	}
}

func TestConvertToCollectorSpecs(t *testing.T) {
	// This test would need actual troubleshootv1beta2.Collect instances
	// For now, test with nil input
	specs, err := convertToCollectorSpecs([]*troubleshootv1beta2.Collect{})
	if err != nil {
		t.Errorf("convertToCollectorSpecs() with empty input should not error: %v", err)
	}
	if len(specs) != 0 {
		t.Errorf("convertToCollectorSpecs() with empty input should return empty slice, got %d items", len(specs))
	}
}

func TestConvertToTroubleshootCollectors(t *testing.T) {
	// This test would need actual autodiscovery.CollectorSpec instances
	// For now, test with nil input
	collectors, err := convertToTroubleshootCollectors([]autodiscovery.CollectorSpec{})
	if err != nil {
		t.Errorf("convertToTroubleshootCollectors() with empty input should not error: %v", err)
	}
	if len(collectors) != 0 {
		t.Errorf("convertToTroubleshootCollectors() with empty input should return empty slice, got %d items", len(collectors))
	}
}
