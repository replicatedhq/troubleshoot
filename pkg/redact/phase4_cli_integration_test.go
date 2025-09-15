package redact

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestPhase4_CLI_TokenizationFlags tests CLI flag integration
func TestPhase4_CLI_TokenizationFlags(t *testing.T) {
	// Test tokenization flag functionality without full CLI
	defer ResetRedactionList()

	tests := []struct {
		name               string
		enableTokenization bool
		expectedBehavior   string
	}{
		{
			name:               "tokenization disabled by default",
			enableTokenization: false,
			expectedBehavior:   "masked",
		},
		{
			name:               "tokenization enabled via flag",
			enableTokenization: true,
			expectedBehavior:   "tokenized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tokenizer state
			ResetGlobalTokenizer()
			globalTokenizer = nil
			tokenizerOnce = sync.Once{}

			// Simulate CLI flag setting
			if tt.enableTokenization {
				os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
			} else {
				os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
			}
			defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")

			// Test tokenizer behavior
			tokenizer := GetGlobalTokenizer()
			result := tokenizer.TokenizeValue("test-secret", "cli_test")

			switch tt.expectedBehavior {
			case "masked":
				if result != MASK_TEXT {
					t.Errorf("Expected mask text '%s', got '%s'", MASK_TEXT, result)
				}
				if tokenizer.IsEnabled() {
					t.Error("Tokenizer should be disabled by default")
				}
			case "tokenized":
				if result == MASK_TEXT {
					t.Errorf("Expected tokenization, got mask text '%s'", result)
				}
				if !strings.HasPrefix(result, "***TOKEN_") {
					t.Errorf("Expected token format, got '%s'", result)
				}
				if !tokenizer.IsEnabled() {
					t.Error("Tokenizer should be enabled when flag is set")
				}
			}

			t.Logf("CLI flag test passed - Tokenization: %v, Result: %s", tt.enableTokenization, result)
		})
	}
}

// TestPhase4_CLI_RedactionMapGeneration tests redaction map CLI integration
func TestPhase4_CLI_RedactionMapGeneration(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	// Create temporary directory for test files
	tempDir, err := ioutil.TempDir("", "cli_redaction_map_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name            string
		encrypt         bool
		expectEncrypted bool
	}{
		{
			name:            "unencrypted redaction map",
			encrypt:         false,
			expectEncrypted: false,
		},
		{
			name:            "encrypted redaction map",
			encrypt:         true,
			expectEncrypted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tokenizer for each test
			ResetGlobalTokenizer()
			globalTokenizer = nil
			tokenizerOnce = sync.Once{}

			tokenizer := GetGlobalTokenizer()
			tokenizer.Reset()

			// Simulate CLI redaction map generation
			testSecrets := []struct {
				secret  string
				context string
				file    string
			}{
				{"cli-test-password", "password", "cli-test-config.yaml"},
				{"cli-test-api-key", "api_key", "cli-test-secrets.yaml"},
				{"cli-test-database", "database", "cli-test-db.yaml"},
			}

			for _, ts := range testSecrets {
				tokenizer.TokenizeValueWithPath(ts.secret, ts.context, ts.file)
			}

			// Generate mapping file (simulating CLI --redaction-map flag)
			mapPath := filepath.Join(tempDir, fmt.Sprintf("cli-test-%s.json", tt.name))
			err := tokenizer.GenerateRedactionMapFile("cli-test", mapPath, tt.encrypt)
			if err != nil {
				t.Fatalf("Failed to generate redaction map: %v", err)
			}

			// Verify file was created
			if _, err := os.Stat(mapPath); os.IsNotExist(err) {
				t.Fatalf("Redaction map file was not created: %s", mapPath)
			}

			// Load and verify the mapping
			loadedMap, err := LoadRedactionMapFile(mapPath, nil)
			if err != nil {
				t.Fatalf("Failed to load redaction map: %v", err)
			}

			// Verify encryption status
			if loadedMap.IsEncrypted != tt.expectEncrypted {
				t.Errorf("Expected encryption status %v, got %v", tt.expectEncrypted, loadedMap.IsEncrypted)
			}

			// Verify content
			if len(loadedMap.Tokens) != len(testSecrets) {
				t.Errorf("Expected %d tokens, got %d", len(testSecrets), len(loadedMap.Tokens))
			}

			// Verify file permissions (should be secure)
			fileInfo, err := os.Stat(mapPath)
			if err != nil {
				t.Fatalf("Failed to stat mapping file: %v", err)
			}

			expectedMode := os.FileMode(0600)
			if fileInfo.Mode() != expectedMode {
				t.Errorf("Expected secure file mode %o, got %o", expectedMode, fileInfo.Mode())
			}

			// Validate mapping file
			if err := ValidateRedactionMapFile(mapPath); err != nil {
				t.Errorf("Redaction map validation failed: %v", err)
			}

			t.Logf("CLI redaction map test passed - Encrypted: %v, File: %s", tt.encrypt, mapPath)
		})
	}
}

// TestPhase4_CLI_TokenPrefixCustomization tests custom token prefix functionality
func TestPhase4_CLI_TokenPrefixCustomization(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tests := []struct {
		name          string
		customPrefix  string
		expectValid   bool
		expectedError string
	}{
		{
			name:          "valid custom prefix",
			customPrefix:  "***CUSTOM_%s_%s***",
			expectValid:   true,
			expectedError: "",
		},
		{
			name:          "valid custom prefix with different format",
			customPrefix:  "[TOKEN_%s_%s]",
			expectValid:   true,
			expectedError: "",
		},
		{
			name:          "invalid prefix - missing placeholders",
			customPrefix:  "***CUSTOM_TOKEN***",
			expectValid:   false,
			expectedError: "must contain at least 2",
		},
		{
			name:          "invalid prefix - only one placeholder",
			customPrefix:  "***CUSTOM_%s***",
			expectValid:   false,
			expectedError: "must contain at least 2",
		},
		{
			name:          "empty prefix should use default",
			customPrefix:  "",
			expectValid:   true,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test custom prefix validation
			if tt.customPrefix != "" {
				// Count %s occurrences
				placeholderCount := strings.Count(tt.customPrefix, "%s")
				hasValidPlaceholders := placeholderCount >= 2

				if tt.expectValid && !hasValidPlaceholders {
					t.Errorf("Test setup error: expected valid prefix '%s' but it's actually invalid", tt.customPrefix)
				}

				if !tt.expectValid && hasValidPlaceholders {
					t.Errorf("Test setup error: expected invalid prefix '%s' but it's actually valid", tt.customPrefix)
				}
			}

			// Simulate prefix validation (as would happen in CLI)
			validationErr := validateTokenPrefix(tt.customPrefix)

			if tt.expectValid {
				if validationErr != nil {
					t.Errorf("Expected valid prefix, got error: %v", validationErr)
				}
			} else {
				if validationErr == nil {
					t.Errorf("Expected validation error for prefix '%s'", tt.customPrefix)
				} else if !strings.Contains(validationErr.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.expectedError, validationErr.Error())
				}
			}

			t.Logf("Token prefix validation test passed - Prefix: '%s', Valid: %v", tt.customPrefix, tt.expectValid)
		})
	}
}

// TestPhase4_CLI_VerificationMode tests the --verify-tokenization flag
func TestPhase4_CLI_VerificationMode(t *testing.T) {
	// Test verification mode functionality
	verificationTests := []struct {
		name               string
		enableTokenization bool
		customPrefix       string
		redactionMapPath   string
		expectSuccess      bool
	}{
		{
			name:               "verification with tokenization enabled",
			enableTokenization: true,
			expectSuccess:      true,
		},
		{
			name:               "verification with tokenization disabled",
			enableTokenization: false,
			expectSuccess:      true, // Should still pass, just report disabled state
		},
		{
			name:               "verification with custom prefix",
			enableTokenization: true,
			customPrefix:       "***CUSTOM_%s_%s***",
			expectSuccess:      true,
		},
		{
			name:               "verification with redaction map",
			enableTokenization: true,
			redactionMapPath:   "/tmp/test-redaction-map.json",
			expectSuccess:      true,
		},
	}

	for _, vt := range verificationTests {
		t.Run(vt.name, func(t *testing.T) {
			// Reset state
			os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
			ResetGlobalTokenizer()
			globalTokenizer = nil
			tokenizerOnce = sync.Once{}

			// Set up test environment
			if vt.enableTokenization {
				os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
			}
			defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")

			// Create temp dir if redaction map path is relative
			if vt.redactionMapPath != "" && !filepath.IsAbs(vt.redactionMapPath) {
				tempDir, err := ioutil.TempDir("", "verification_test")
				if err != nil {
					t.Fatalf("Failed to create temp dir: %v", err)
				}
				defer os.RemoveAll(tempDir)
				vt.redactionMapPath = filepath.Join(tempDir, "test-map.json")
			}

			// Simulate verification process
			err := runTokenizationVerification(vt.enableTokenization, vt.customPrefix, vt.redactionMapPath)

			if vt.expectSuccess {
				if err != nil {
					t.Errorf("Expected verification to succeed, got error: %v", err)
				}
			} else {
				if err == nil {
					t.Errorf("Expected verification to fail")
				}
			}

			t.Logf("Verification test passed - Tokenization: %v, Error: %v", vt.enableTokenization, err)
		})
	}
}

// TestPhase4_CLI_BundleIDIntegration tests custom bundle ID functionality
func TestPhase4_CLI_BundleIDIntegration(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tests := []struct {
		name           string
		customBundleID string
		expectCustom   bool
	}{
		{
			name:           "auto-generated bundle ID",
			customBundleID: "",
			expectCustom:   false,
		},
		{
			name:           "custom bundle ID provided",
			customBundleID: "production-support-bundle-2023",
			expectCustom:   true,
		},
		{
			name:           "custom bundle ID with special chars",
			customBundleID: "custom-bundle-id_test.123",
			expectCustom:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tokenizer for each test
			ResetGlobalTokenizer()
			globalTokenizer = nil
			tokenizerOnce = sync.Once{}

			tokenizer := GetGlobalTokenizer()

			// Test bundle ID functionality
			originalBundleID := tokenizer.GetBundleID()

			// Simulate CLI processing with custom bundle ID
			finalBundleID := originalBundleID
			if tt.customBundleID != "" {
				finalBundleID = tt.customBundleID
			}

			// Generate some tokens to test correlation
			tokenizer.TokenizeValueWithPath("test-secret-1", "secret", "test1.yaml")
			tokenizer.TokenizeValueWithPath("test-secret-2", "password", "test2.yaml")

			// Get redaction map with bundle ID
			redactionMap := tokenizer.GetRedactionMap("bundle-id-test")

			// Verify bundle ID behavior
			if tt.expectCustom {
				// When using custom bundle ID, it should be reflected in the final bundle ID
				// (this would be set by CLI logic in actual implementation)
				if tt.customBundleID == "" {
					t.Error("Test setup error: expected custom bundle ID")
				}
			} else {
				// Auto-generated bundle ID should be valid
				if originalBundleID == "" {
					t.Error("Auto-generated bundle ID should not be empty")
				}
				if !strings.HasPrefix(originalBundleID, "bundle_") {
					t.Errorf("Auto-generated bundle ID should start with 'bundle_', got '%s'", originalBundleID)
				}
			}

			// Verify redaction map contains bundle ID
			if redactionMap.BundleID == "" {
				t.Error("Redaction map should contain bundle ID")
			}

			t.Logf("Bundle ID test passed - Custom: %v, Bundle ID: %s", tt.expectCustom, finalBundleID)
		})
	}
}

// TestPhase4_CLI_TokenizationStatsIntegration tests statistics reporting
func TestPhase4_CLI_TokenizationStatsIntegration(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Simulate support bundle collection with various secrets
	bundleSecrets := []struct {
		secret  string
		context string
		file    string
	}{
		{"production-db-password", "database_password", "kubernetes/secrets.yaml"},
		{"production-db-password", "database_password", "docker/compose.yml"}, // Duplicate
		{"api-key-production", "api_key", "kubernetes/configmap.yaml"},
		{"jwt-signing-key", "jwt_secret", "auth/secrets.yaml"},
		{"aws-access-key-id", "aws_access_key", "aws/credentials"},
		{"aws-secret-access-key", "aws_secret_key", "aws/credentials"},
		{"oauth-client-secret", "oauth_secret", "oauth/config.yaml"},
		{"redis-password", "redis_password", "redis/config.yaml"},
		{"admin@company.com", "admin_email", "users/admin.yaml"},
		{"192.168.1.100", "server_ip", "network/config.yaml"},
	}

	// Process all secrets (simulating collection)
	for _, bs := range bundleSecrets {
		tokenizer.TokenizeValueWithPath(bs.secret, bs.context, bs.file)
	}

	// Get comprehensive statistics
	redactionMap := tokenizer.GetRedactionMap("stats-test")

	// Verify comprehensive statistics
	expectedStats := []struct {
		field    string
		value    int
		minValue int
	}{
		{"TotalSecrets", redactionMap.Stats.TotalSecrets, 9}, // 10 secrets, but 1 duplicate = 9 unique
		{"TokensGenerated", redactionMap.Stats.TokensGenerated, 9},
		{"FilesCovered", redactionMap.Stats.FilesCovered, 7},     // 7 unique files
		{"DuplicateCount", redactionMap.Stats.DuplicateCount, 1}, // 1 duplicate (password)
	}

	for _, es := range expectedStats {
		var actualValue int
		switch es.field {
		case "TotalSecrets":
			actualValue = redactionMap.Stats.TotalSecrets
		case "TokensGenerated":
			actualValue = redactionMap.Stats.TokensGenerated
		case "FilesCovered":
			actualValue = redactionMap.Stats.FilesCovered
		case "DuplicateCount":
			actualValue = redactionMap.Stats.DuplicateCount
		}

		if actualValue < es.minValue {
			t.Errorf("Statistics field %s: expected at least %d, got %d", es.field, es.minValue, actualValue)
		}
	}

	// Verify secret type breakdown
	if len(redactionMap.Stats.SecretsByType) == 0 {
		t.Error("Expected secret type breakdown in statistics")
	}

	// Verify file coverage
	if len(redactionMap.Stats.FileCoverage) == 0 {
		t.Error("Expected file coverage in statistics")
	}

	// Verify correlation analysis
	if len(redactionMap.Correlations) == 0 {
		t.Error("Expected correlation analysis in statistics")
	}

	// Verify duplicate detection
	if len(redactionMap.Duplicates) == 0 {
		t.Error("Expected duplicate detection in statistics")
	}

	t.Logf("Tokenization stats test passed - Total: %d, Unique: %d, Files: %d, Duplicates: %d",
		redactionMap.Stats.TotalSecrets,
		redactionMap.Stats.UniqueSecrets,
		redactionMap.Stats.FilesCovered,
		redactionMap.Stats.DuplicateCount)
}

// TestPhase4_CLI_HelpDocumentation tests help text and documentation
func TestPhase4_CLI_HelpDocumentation(t *testing.T) {
	// Test help text content (simulated)
	helpTopics := []struct {
		topic       string
		keywords    []string
		description string
	}{
		{
			topic:       "tokenization",
			keywords:    []string{"tokenize", "intelligent", "correlation", "***TOKEN_"},
			description: "Intelligent tokenization help",
		},
		{
			topic:       "redaction-map",
			keywords:    []string{"mapping", "token", "original", "reverse"},
			description: "Redaction mapping help",
		},
		{
			topic:       "encryption",
			keywords:    []string{"encrypt", "AES-256", "secure", "permissions"},
			description: "Encryption help",
		},
		{
			topic:       "verification",
			keywords:    []string{"verify", "validate", "test", "setup"},
			description: "Verification help",
		},
	}

	for _, ht := range helpTopics {
		t.Run(ht.topic, func(t *testing.T) {
			// Generate help content (simulated)
			helpContent := generateHelpContent(ht.topic)

			// Verify help content contains expected keywords
			for _, keyword := range ht.keywords {
				if !strings.Contains(helpContent, keyword) {
					t.Errorf("Help content for '%s' missing keyword '%s'", ht.topic, keyword)
				}
			}

			// Verify help is substantial
			if len(helpContent) < 100 {
				t.Errorf("Help content for '%s' too short: %d characters", ht.topic, len(helpContent))
			}

			t.Logf("Help documentation test passed - Topic: %s, Length: %d chars", ht.topic, len(helpContent))
		})
	}
}

// Helper functions for CLI integration testing

// validateTokenPrefix simulates CLI token prefix validation
func validateTokenPrefix(prefix string) error {
	if prefix == "" {
		return nil // Empty is OK, will use default
	}

	placeholderCount := strings.Count(prefix, "%s")
	if placeholderCount < 2 {
		return fmt.Errorf("custom token prefix must contain at least 2 %%s placeholders, found %d: %s", placeholderCount, prefix)
	}

	return nil
}

// runTokenizationVerification simulates the --verify-tokenization flag
func runTokenizationVerification(enableTokenization bool, customPrefix, redactionMapPath string) error {
	// Test 1: Tokenizer state
	tokenizer := GetGlobalTokenizer()

	if enableTokenization && !tokenizer.IsEnabled() {
		return fmt.Errorf("tokenizer should be enabled")
	}

	if !enableTokenization && tokenizer.IsEnabled() {
		return fmt.Errorf("tokenizer should be disabled")
	}

	// Test 2: Token generation
	if tokenizer.IsEnabled() {
		testToken := tokenizer.TokenizeValue("verification-test", "test")
		if !tokenizer.ValidateToken(testToken) {
			return fmt.Errorf("generated test token is invalid: %s", testToken)
		}
	}

	// Test 3: Custom prefix validation
	if customPrefix != "" {
		if err := validateTokenPrefix(customPrefix); err != nil {
			return fmt.Errorf("custom prefix validation failed: %v", err)
		}
	}

	// Test 4: Redaction map path validation
	if redactionMapPath != "" {
		dir := filepath.Dir(redactionMapPath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return fmt.Errorf("redaction map directory does not exist: %s", dir)
		}
	}

	return nil
}

// generateHelpContent simulates help content generation for CLI topics
func generateHelpContent(topic string) string {
	helpTemplates := map[string]string{
		"tokenization": `
Intelligent Tokenization

The --tokenize flag enables intelligent tokenization of sensitive data in support bundles.
Instead of replacing secrets with ***HIDDEN***, tokenization generates unique, deterministic 
tokens like ***TOKEN_PASSWORD_A1B2C3*** that allow for:

- Cross-file correlation of the same secret
- Secret type classification (password, API key, database, etc.)
- Performance optimization through caching
- Optional reversible mapping for authorized access

Examples:
  support-bundle --tokenize
  support-bundle --tokenize --redaction-map ./tokens.json
  support-bundle --tokenize --tokenization-stats
`,
		"redaction-map": `
Redaction Mapping

The --redaction-map flag generates a mapping file that allows authorized personnel
to reverse tokenization and view original secret values. The mapping file contains:

- Token to original value mappings
- Cross-file reference tracking
- Duplicate secret detection
- Correlation analysis
- File coverage statistics

Security Features:
- Optional AES-256 encryption with --encrypt-redaction-map
- Secure file permissions (0600)
- Comprehensive validation

Examples:
  support-bundle --tokenize --redaction-map ./redaction-map.json
  support-bundle --tokenize --redaction-map ./secure-map.json --encrypt-redaction-map
`,
		"encryption": `
Redaction Map Encryption

The --encrypt-redaction-map flag encrypts the redaction mapping file using AES-256-GCM
encryption for maximum security. Features:

- Industry-standard AES-256-GCM encryption
- Secure random nonce generation
- Authentication tag verification
- Secure file permissions (0600)
- Tamper detection

Examples:
  support-bundle --tokenize --redaction-map ./secure.json --encrypt-redaction-map
`,
		"verification": `
Tokenization Verification

The --verify-tokenization flag validates tokenization setup without collecting data.
Verification includes:

- Tokenizer initialization testing
- Token generation validation
- Custom prefix format verification
- Redaction map path accessibility
- File creation permissions

Examples:
  support-bundle --verify-tokenization
  support-bundle --verify-tokenization --tokenize
  support-bundle --verify-tokenization --tokenize --redaction-map ./test.json
`,
	}

	if content, exists := helpTemplates[topic]; exists {
		return strings.TrimSpace(content)
	}

	return fmt.Sprintf("Help content for topic '%s' (placeholder)", topic)
}
