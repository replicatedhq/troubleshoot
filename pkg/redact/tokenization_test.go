package redact

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func TestTokenization(t *testing.T) {
	// Test data with various secret patterns that should be tokenized
	testData := `# Comprehensive tokenization test data

# Environment variables (KEY=value format)
DATABASE_PASSWORD=mysqlpassword123
API_TOKEN=sk-1234567890abcdef
USER_PASSWORD=userpass123
OPENAI_API_KEY=sk-openai123456789
STRIPE_SECRET_KEY=sk_test_stripe123
GITHUB_TOKEN=ghp_github123456789
SLACK_BOT_TOKEN=xoxb-slack123456789

# YAML key-value patterns
password: "simplepass"
api-key: "simple-api-key"
secret: "simple-secret"
token: "simple-token"
openai-key: "sk-openai-dynamic-key"
stripe-secret: "sk_test_stripe_dynamic"
github-token: "ghp_github_dynamic_token"
slack-webhook-secret: "webhook-secret-123"
database-password: "db-pass-dynamic"
redis-password: "redis-pass-123"
jwt-secret: "jwt-secret-key-123"
encryption-key: "encrypt-key-456"
client-secret: "oauth-client-secret"
private-key: "-----BEGIN PRIVATE KEY-----"

# Connection strings
database-url: "mysql://user:password@host:3306/db"
redis-url: "redis://user:pass@redis:6379"
postgres-url: "postgresql://user:pass@postgres:5432/db"

# Database connection string format
Data Source=server;User ID=user;password=pass;Database=db;

# Nested YAML structures
config:
  openai:
    api-key: "sk-openai-nested-key"
    secret: "openai-secret-nested"
  stripe:
    publishable-key: "pk_test_stripe123"
    secret-key: "sk_test_stripe456"
  database:
    username: "dbuser"
    password: "dbpass123"
    connection-string: "postgresql://user:pass@host:5432/db"

# Environment-style in YAML
env:
  - name: OPENAI_API_KEY
    value: "sk-openai-env-style"
  - name: DATABASE_PASSWORD  
    value: "db-pass-env-style"
  - name: CUSTOM_SECRET_TOKEN
    value: "custom-secret-123"

# JSON-like environment variables
env_vars: |
  "name":"PASSWORD","value":"secret123"
  "name":"API_KEY","value":"key123"
  "name":"DATABASE_URL","value":"mysql://user:pass@host/db"

# AWS-style keys (multiline patterns)
aws_config: |
  "name": "AWS_SECRET_ACCESS_KEY"
  "value": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
`

	tests := []struct {
		name                string
		enableTokenization  bool
		expectedTokenCount  int
		shouldContainTokens bool
		shouldContainHidden bool
	}{
		{
			name:                "Without Tokenization",
			enableTokenization:  false,
			expectedTokenCount:  0,
			shouldContainTokens: false,
			shouldContainHidden: true,
		},
		{
			name:                "With Tokenization",
			enableTokenization:  true,
			expectedTokenCount:  20, // Approximate - should be > 15
			shouldContainTokens: true,
			shouldContainHidden: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set tokenization environment
			if tt.enableTokenization {
				os.Setenv("TROUBLESHOOT_TOKENIZATION", "1")
			} else {
				os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
			}

			// Reset the enableTokenization flag (it's set in init())
			if tt.enableTokenization {
				enableTokenization = true
			} else {
				enableTokenization = false
			}

			// Apply redaction
			input := strings.NewReader(testData)
			redacted, err := Redact(input, "test-tokenization.yaml", []*troubleshootv1beta2.Redact{})
			if err != nil {
				t.Fatalf("Redaction failed: %v", err)
			}

			// Read the result
			var output bytes.Buffer
			_, err = io.Copy(&output, redacted)
			if err != nil {
				t.Fatalf("Failed to read redacted output: %v", err)
			}

			result := output.String()

			// Count tokens
			tokenCount := strings.Count(result, "***TOKEN_")
			hiddenCount := strings.Count(result, "***HIDDEN***")

			t.Logf("Token count: %d, Hidden count: %d", tokenCount, hiddenCount)

			// Verify token expectations
			if tt.shouldContainTokens {
				if tokenCount < tt.expectedTokenCount {
					t.Errorf("Expected at least %d tokens, got %d", tt.expectedTokenCount, tokenCount)
				}
				if !strings.Contains(result, "***TOKEN_") {
					t.Error("Expected to find tokenized values, but none found")
				}
			} else {
				if tokenCount > 0 {
					t.Errorf("Expected no tokens, but found %d", tokenCount)
				}
			}

			// Verify hidden expectations
			if tt.shouldContainHidden {
				if hiddenCount == 0 {
					t.Error("Expected to find ***HIDDEN*** values, but none found")
				}
			}

			// Verify specific patterns are redacted
			secretPatterns := []string{
				"mysqlpassword123",
				"sk-1234567890abcdef",
				"simplepass",
				"simple-api-key",
				"simple-secret",
				"sk-openai-dynamic-key",
				"webhook-secret-123",
				"db-pass-dynamic",
				"oauth-client-secret",
			}

			for _, pattern := range secretPatterns {
				if strings.Contains(result, pattern) {
					t.Errorf("Secret value '%s' was not redacted", pattern)
				}
			}

			// Verify non-secret values are preserved
			preservedPatterns := []string{
				"# Comprehensive tokenization test data",
				"password:",
				"api-key:",
				"secret:",
				"config:",
				"openai:",
				"stripe:",
			}

			for _, pattern := range preservedPatterns {
				if !strings.Contains(result, pattern) {
					t.Errorf("Non-secret pattern '%s' was incorrectly redacted", pattern)
				}
			}
		})
	}
}

func TestTokenUniqueness(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "1")
	enableTokenization = true

	testData := `
password1: "secret123"
password2: "secret456"
password3: "secret123"
api-key1: "key-abc"
api-key2: "key-def"
api-key3: "key-abc"
`

	input := strings.NewReader(testData)
	redacted, err := Redact(input, "test-uniqueness.yaml", []*troubleshootv1beta2.Redact{})
	if err != nil {
		t.Fatalf("Redaction failed: %v", err)
	}

	var output bytes.Buffer
	io.Copy(&output, redacted)
	result := output.String()

	t.Logf("Redacted result:\n%s", result)

	// Extract all tokens
	lines := strings.Split(result, "\n")
	tokens := make(map[string][]string) // token -> lines that contain it

	for _, line := range lines {
		if strings.Contains(line, "***TOKEN_") {
			// Extract token
			start := strings.Index(line, "***TOKEN_")
			if start != -1 {
				end := strings.Index(line[start+3:], "***")
				if end != -1 {
					token := line[start : start+3+end+3]
					tokens[token] = append(tokens[token], strings.TrimSpace(line))
				}
			}
		}
	}

	// Verify that same values get same tokens
	// password1 and password3 should have the same token (both "secret123")
	// api-key1 and api-key3 should have the same token (both "key-abc")
	// password2 and api-key2 should have different tokens

	if len(tokens) < 2 {
		t.Errorf("Expected at least 2 unique tokens, got %d", len(tokens))
	}

	// Count how many times each token appears
	tokenCounts := make(map[string]int)
	for token, lines := range tokens {
		tokenCounts[token] = len(lines)
		t.Logf("Token %s appears %d times in lines: %v", token, len(lines), lines)
	}

	// We should have some tokens that appear multiple times (same values)
	// and some that appear once (unique values)
	hasRepeatedToken := false
	for _, count := range tokenCounts {
		if count > 1 {
			hasRepeatedToken = true
			break
		}
	}

	if !hasRepeatedToken {
		t.Error("Expected some tokens to appear multiple times (for same values), but all tokens are unique")
	}
}

func TestTokenDeterminism(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "1")
	enableTokenization = true

	testData := `password: "test123"`

	// Run redaction twice
	var results []string
	for i := 0; i < 2; i++ {
		input := strings.NewReader(testData)
		redacted, err := Redact(input, "test-determinism.yaml", []*troubleshootv1beta2.Redact{})
		if err != nil {
			t.Fatalf("Redaction failed on iteration %d: %v", i, err)
		}

		var output bytes.Buffer
		io.Copy(&output, redacted)
		results = append(results, output.String())
	}

	// Results should be identical (deterministic)
	if results[0] != results[1] {
		t.Errorf("Tokenization is not deterministic.\nFirst result:\n%s\nSecond result:\n%s", results[0], results[1])
	}

	// Should contain a token
	if !strings.Contains(results[0], "***TOKEN_") {
		t.Error("Expected tokenized output, but no tokens found")
	}
}

func TestComprehensiveSampleSecrets(t *testing.T) {
	// Load the comprehensive sample secrets file
	sampleFile := filepath.Join("testdata", "sample_secrets.yaml")
	data, err := os.ReadFile(sampleFile)
	if err != nil {
		t.Fatalf("Failed to read sample secrets file: %v", err)
	}

	tests := []struct {
		name                string
		enableTokenization  bool
		minExpectedTokens   int
		shouldContainTokens bool
		shouldContainHidden bool
	}{
		{
			name:                "Sample Secrets Without Tokenization",
			enableTokenization:  false,
			minExpectedTokens:   0,
			shouldContainTokens: false,
			shouldContainHidden: true,
		},
		{
			name:                "Sample Secrets With Tokenization",
			enableTokenization:  true,
			minExpectedTokens:   100, // Should have many tokens from comprehensive data
			shouldContainTokens: true,
			shouldContainHidden: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set tokenization environment
			if tt.enableTokenization {
				os.Setenv("TROUBLESHOOT_TOKENIZATION", "1")
				enableTokenization = true
			} else {
				os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
				enableTokenization = false
			}

			// Apply redaction
			input := strings.NewReader(string(data))
			redacted, err := Redact(input, "sample_secrets.yaml", []*troubleshootv1beta2.Redact{})
			if err != nil {
				t.Fatalf("Redaction failed: %v", err)
			}

			// Read the result
			var output bytes.Buffer
			_, err = io.Copy(&output, redacted)
			if err != nil {
				t.Fatalf("Failed to read redacted output: %v", err)
			}

			result := output.String()

			// Count tokens and hidden values
			tokenCount := strings.Count(result, "***TOKEN_")
			hiddenCount := strings.Count(result, "***HIDDEN***")

			t.Logf("Sample file processed: %d bytes input, %d bytes output", len(data), len(result))
			t.Logf("Token count: %d, Hidden count: %d", tokenCount, hiddenCount)

			// Verify token expectations
			if tt.shouldContainTokens {
				if tokenCount < tt.minExpectedTokens {
					t.Errorf("Expected at least %d tokens, got %d", tt.minExpectedTokens, tokenCount)
				}
				if !strings.Contains(result, "***TOKEN_") {
					t.Error("Expected to find tokenized values, but none found")
				}
			} else {
				if tokenCount > 0 {
					t.Errorf("Expected no tokens, but found %d", tokenCount)
				}
			}

			// Verify hidden expectations
			if tt.shouldContainHidden {
				if hiddenCount == 0 {
					t.Error("Expected to find ***HIDDEN*** values, but none found")
				}
			}

			// Verify that critical secrets are redacted
			criticalSecrets := []string{
				"super_secret_db_password_123",
				"sk-1234567890abcdefghijklmnopqrstuvwxyz",
				"MyComplexPassword!@#$%",
				"jwt_signing_key_very_long_and_secure_ghi789",
				"sk-openai1234567890abcdefghijklmnopqrstuvwxyz1234567890",
				"sk_test_51234567890abcdefghijklmnopqrstuvwxyz",
				"ghp_github_personal_access_token_stu901",
				"wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
				"basic_yaml_password_123",
				"nested_db_password_123",
				"k8s_db_password_123",
				"json_db_password_123456789abcdef",
				"docker_registry_password_123456789abcdef",
				"datadog_api_key_123456789abcdefghijklmnopqrstuvwxyz",
			}

			for _, secret := range criticalSecrets {
				if strings.Contains(result, secret) {
					t.Errorf("Critical secret '%s' was not redacted", secret)
				}
			}

			// Verify that non-secret structure is preserved
			preservedStructure := []string{
				"# Sample Secrets Test Data",
				"# Environment Variables",
				"# YAML Key-Value Patterns",
				"# Connection Strings",
				"application:",
				"database:",
				"external_services:",
				"security:",
				"env:",
				"monitoring:",
				"cicd:",
			}

			for _, structure := range preservedStructure {
				if !strings.Contains(result, structure) {
					t.Errorf("Structure element '%s' was incorrectly redacted", structure)
				}
			}

			// Log some sample redacted lines for manual inspection
			lines := strings.Split(result, "\n")
			t.Log("Sample redacted lines:")
			count := 0
			for _, line := range lines {
				if (strings.Contains(line, "***TOKEN_") || strings.Contains(line, "***HIDDEN***")) && count < 10 {
					t.Logf("  %s", strings.TrimSpace(line))
					count++
				}
			}
		})
	}
}

func TestSampleSecretsTokenTypes(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "1")
	enableTokenization = true

	// Load the comprehensive sample secrets file
	sampleFile := filepath.Join("testdata", "sample_secrets.yaml")
	data, err := os.ReadFile(sampleFile)
	if err != nil {
		t.Fatalf("Failed to read sample secrets file: %v", err)
	}

	// Apply redaction
	input := strings.NewReader(string(data))
	redacted, err := Redact(input, "sample_secrets.yaml", []*troubleshootv1beta2.Redact{})
	if err != nil {
		t.Fatalf("Redaction failed: %v", err)
	}

	var output bytes.Buffer
	io.Copy(&output, redacted)
	result := output.String()

	// Analyze token types
	tokenTypes := make(map[string]int)
	lines := strings.Split(result, "\n")

	for _, line := range lines {
		if strings.Contains(line, "***TOKEN_") {
			// Extract token type
			start := strings.Index(line, "***TOKEN_")
			if start != -1 {
				end := strings.Index(line[start+9:], "_")
				if end != -1 {
					tokenType := line[start+9 : start+9+end]
					tokenTypes[tokenType]++
				}
			}
		}
	}

	t.Logf("Token types found in sample secrets:")
	for tokenType, count := range tokenTypes {
		t.Logf("  %s: %d occurrences", tokenType, count)
	}

	// Verify we have different token types
	expectedTypes := []string{"PASSWORD", "SECRET", "TOKEN", "USER", "DATABASE"}
	for _, expectedType := range expectedTypes {
		if tokenTypes[expectedType] == 0 {
			t.Errorf("Expected to find token type '%s', but none found", expectedType)
		}
	}

	// Verify we have a reasonable number of total tokens
	totalTokens := 0
	for _, count := range tokenTypes {
		totalTokens += count
	}

	if totalTokens < 100 {
		t.Errorf("Expected at least 100 total tokens from comprehensive sample, got %d", totalTokens)
	}

	t.Logf("Total tokens generated from sample secrets: %d", totalTokens)
}
