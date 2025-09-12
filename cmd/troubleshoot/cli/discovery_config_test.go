package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadDiscoveryConfig(t *testing.T) {
	tests := []struct {
		name        string
		setupConfig func(string) error
		configPath  string
		wantErr     bool
		checkFunc   func(*testing.T, *DiscoveryConfig)
	}{
		{
			name:       "no config path - use defaults",
			configPath: "",
			wantErr:    false,
			checkFunc: func(t *testing.T, config *DiscoveryConfig) {
				if config.Version != "v1" {
					t.Errorf("Expected version v1, got %s", config.Version)
				}
				if len(config.Profiles) == 0 {
					t.Error("Default config should have profiles")
				}
			},
		},
		{
			name:       "non-existent config file - use defaults",
			configPath: "/non/existent/path.json",
			wantErr:    false,
			checkFunc: func(t *testing.T, config *DiscoveryConfig) {
				if config.Version != "v1" {
					t.Errorf("Expected version v1, got %s", config.Version)
				}
			},
		},
		{
			name: "valid config file",
			setupConfig: func(path string) error {
				configContent := `{
					"version": "v1",
					"profiles": {
						"minimal": {
							"name": "minimal",
							"description": "Minimal collection",
							"includeImages": false,
							"rbacCheck": true,
							"maxDepth": 1,
							"timeout": 15000000000
						},
						"standard": {
							"name": "standard",
							"description": "Standard collection",
							"includeImages": false,
							"rbacCheck": true,
							"maxDepth": 2,
							"timeout": 30000000000
						},
						"comprehensive": {
							"name": "comprehensive",
							"description": "Comprehensive collection",
							"includeImages": true,
							"rbacCheck": true,
							"maxDepth": 3,
							"timeout": 60000000000
						}
					},
					"patterns": {
						"namespacePatterns": {
							"include": ["app-*"],
							"exclude": ["kube-*"]
						}
					}
				}`
				return os.WriteFile(path, []byte(configContent), 0644)
			},
			wantErr: false,
			checkFunc: func(t *testing.T, config *DiscoveryConfig) {
				if config.Version != "v1" {
					t.Errorf("Expected version v1, got %s", config.Version)
				}
				if len(config.Profiles) == 0 {
					t.Error("Config should have profiles")
				}
			},
		},
		{
			name: "invalid json config",
			setupConfig: func(path string) error {
				return os.WriteFile(path, []byte(`{invalid json`), 0644)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var configPath string
			if tt.configPath != "" {
				configPath = tt.configPath
			}

			// Setup config file if needed
			if tt.setupConfig != nil {
				tempDir := t.TempDir()
				configPath = filepath.Join(tempDir, "config.json")
				if err := tt.setupConfig(configPath); err != nil {
					t.Fatalf("Failed to setup config: %v", err)
				}
			}

			config, err := LoadDiscoveryConfig(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadDiscoveryConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, config)
			}
		})
	}
}

func TestApplyDiscoveryPatterns(t *testing.T) {
	tests := []struct {
		name     string
		items    []string
		patterns PatternConfig
		want     []string
	}{
		{
			name:  "no patterns",
			items: []string{"app1", "app2", "kube-system"},
			patterns: PatternConfig{
				Include: []string{},
				Exclude: []string{},
			},
			want: []string{"app1", "app2", "kube-system"},
		},
		{
			name:  "exclude patterns only",
			items: []string{"app1", "app2", "kube-system", "kube-public"},
			patterns: PatternConfig{
				Include: []string{},
				Exclude: []string{"kube-*"},
			},
			want: []string{"app1", "app2"},
		},
		{
			name:  "include patterns only",
			items: []string{"app1", "app2", "kube-system"},
			patterns: PatternConfig{
				Include: []string{"app*"},
				Exclude: []string{},
			},
			want: []string{"app1", "app2"},
		},
		{
			name:  "include and exclude patterns",
			items: []string{"app1", "app2", "app-system", "kube-system"},
			patterns: PatternConfig{
				Include: []string{"app*"},
				Exclude: []string{"*system"},
			},
			want: []string{"app1", "app2"}, // app-system excluded, kube-system not included
		},
		{
			name:  "exact match patterns",
			items: []string{"app1", "app2", "special"},
			patterns: PatternConfig{
				Include: []string{"special", "app1"},
				Exclude: []string{},
			},
			want: []string{"app1", "special"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ApplyDiscoveryPatterns(tt.items, tt.patterns)
			if err != nil {
				t.Errorf("ApplyDiscoveryPatterns() error = %v", err)
				return
			}

			if len(got) != len(tt.want) {
				t.Errorf("ApplyDiscoveryPatterns() length = %v, want %v", len(got), len(tt.want))
				return
			}

			// Check that all expected items are present
			gotMap := make(map[string]bool)
			for _, item := range got {
				gotMap[item] = true
			}

			for _, wantItem := range tt.want {
				if !gotMap[wantItem] {
					t.Errorf("ApplyDiscoveryPatterns() missing expected item: %s", wantItem)
				}
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		item    string
		pattern string
		want    bool
		wantErr bool
	}{
		{
			name:    "exact match",
			item:    "app1",
			pattern: "app1",
			want:    true,
		},
		{
			name:    "wildcard all",
			item:    "anything",
			pattern: "*",
			want:    true,
		},
		{
			name:    "prefix wildcard",
			item:    "app-namespace",
			pattern: "app*",
			want:    true,
		},
		{
			name:    "suffix wildcard",
			item:    "kube-system",
			pattern: "*system",
			want:    true,
		},
		{
			name:    "substring wildcard",
			item:    "my-app-namespace",
			pattern: "*app*",
			want:    true,
		},
		{
			name:    "no match",
			item:    "different",
			pattern: "app*",
			want:    false,
		},
		{
			name:    "prefix no match",
			item:    "other-app",
			pattern: "app*",
			want:    false,
		},
		{
			name:    "suffix no match",
			item:    "system-app",
			pattern: "*system",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matchPattern(tt.item, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("matchPattern() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("matchPattern() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSaveDiscoveryConfig(t *testing.T) {
	config := getDefaultDiscoveryConfig()

	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	err := SaveDiscoveryConfig(config, configPath)
	if err != nil {
		t.Fatalf("SaveDiscoveryConfig() error = %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("SaveDiscoveryConfig() did not create config file")
	}

	// Verify we can load it back
	loadedConfig, err := LoadDiscoveryConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to reload saved config: %v", err)
	}

	if loadedConfig.Version != config.Version {
		t.Errorf("Reloaded config version = %v, want %v", loadedConfig.Version, config.Version)
	}
}

func TestGetDiscoveryConfigPath(t *testing.T) {
	path := GetDiscoveryConfigPath()

	if path == "" {
		t.Error("GetDiscoveryConfigPath() should not return empty string")
	}

	// Should end with expected filename
	expectedSuffix := "discovery.json"
	if !strings.HasSuffix(path, expectedSuffix) {
		t.Errorf("GetDiscoveryConfigPath() should end with %s, got %s", expectedSuffix, path)
	}
}

func TestValidateDiscoveryConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *DiscoveryConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &DiscoveryConfig{
				Version:  "v1",
				Profiles: GetDiscoveryProfiles(),
			},
			wantErr: false,
		},
		{
			name: "missing version gets default",
			config: &DiscoveryConfig{
				Profiles: GetDiscoveryProfiles(),
			},
			wantErr: false,
		},
		{
			name: "nil profiles",
			config: &DiscoveryConfig{
				Version:  "v1",
				Profiles: nil,
			},
			wantErr: true,
		},
		{
			name: "missing required profile",
			config: &DiscoveryConfig{
				Version: "v1",
				Profiles: map[string]DiscoveryProfile{
					"custom": {Name: "custom"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDiscoveryConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDiscoveryConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkMatchPattern(b *testing.B) {
	testCases := []struct {
		item    string
		pattern string
	}{
		{"app-namespace", "app*"},
		{"kube-system", "*system"},
		{"my-app-test", "*app*"},
		{"exact-match", "exact-match"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			_, err := matchPattern(tc.item, tc.pattern)
			if err != nil {
				b.Fatalf("matchPattern failed: %v", err)
			}
		}
	}
}

func BenchmarkApplyDiscoveryPatterns(b *testing.B) {
	items := make([]string, 100)
	for i := 0; i < 100; i++ {
		if i%3 == 0 {
			items[i] = fmt.Sprintf("app-%d", i)
		} else if i%3 == 1 {
			items[i] = fmt.Sprintf("kube-system-%d", i)
		} else {
			items[i] = fmt.Sprintf("other-%d", i)
		}
	}

	patterns := PatternConfig{
		Include: []string{"app*", "other*"},
		Exclude: []string{"*system*"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ApplyDiscoveryPatterns(items, patterns)
		if err != nil {
			b.Fatalf("ApplyDiscoveryPatterns failed: %v", err)
		}
	}
}
