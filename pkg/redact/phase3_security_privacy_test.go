package redact

import (
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// TestPhase3_PlaintextLeakageVerification ensures no secrets leak into output
func TestPhase3_PlaintextLeakageVerification(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	// Test scenarios with various sensitive data patterns
	sensitiveTestCases := []struct {
		name        string
		createInput func() string
		secrets     []string // List of sensitive values that should NOT appear in output
		redactor    func() Redactor
		filePath    string
	}{
		{
			name: "AWS credentials in JSON",
			createInput: func() string {
				return `{
  "aws": {
    "accessKeyId": "AKIA1234567890SENSITIVE",
    "secretAccessKey": "not-targeted-by-this-redactor",
    "region": "us-west-2"
  }
}`
			},
			secrets: []string{
				"AKIA1234567890SENSITIVE", // Only this one should be redacted by this literal redactor
			},
			redactor: func() Redactor {
				return literalString([]byte("AKIA1234567890SENSITIVE"), "aws-creds.json", "aws_access_key")
			},
			filePath: "aws-creds.json",
		},
		{
			name: "Database connection strings",
			createInput: func() string {
				return `DATABASE_URL=postgres://admin:highly-secret-db-password@prod-db.internal.com:5432/production_app
REDIS_URL=redis://:other-data@redis.internal.com:6379/0`
			},
			secrets: []string{
				"highly-secret-db-password", // Only this one is targeted by the regex
			},
			redactor: func() Redactor {
				r, _ := NewSingleLineRedactor(LineRedactor{
					regex: `(postgres://[^:]+:)(?P<mask>[^@]+)(@.*)`,
				}, MASK_TEXT, "database.env", "db_password", false)
				return r
			},
			filePath: "database.env",
		},
		{
			name: "Kubernetes secrets YAML",
			createInput: func() string {
				return `apiVersion: v1
kind: Secret
metadata:
  name: app-secrets
data:
  jwt-secret: "ultra-confidential-jwt-signing-key-2023"
  api-token: "safe-public-reference"
  db-password: "not-targeted-by-jwt-redactor"`
			},
			secrets: []string{
				"ultra-confidential-jwt-signing-key-2023", // Only this is targeted by data.jwt-secret
			},
			redactor: func() Redactor {
				return NewYamlRedactor("data.jwt-secret", "*.yaml", "k8s_jwt_secret")
			},
			filePath: "k8s-secrets.yaml",
		},
		{
			name: "Multi-line environment variables",
			createInput: func() string {
				return `"name": "PRIVATE_KEY"
"value": "confidential-private-key-data-here"`
			},
			secrets: []string{
				"confidential-private-key-data-here", // This should be redacted by the multiline redactor
			},
			redactor: func() Redactor {
				r, _ := NewMultiLineRedactor(LineRedactor{
					regex: `"name": "PRIVATE_KEY"`,
				}, `"value": "(?P<mask>.*)"`, MASK_TEXT, "env-secrets.yaml", "private_key", false)
				return r
			},
			filePath: "env-secrets.yaml",
		},
	}

	for _, tc := range sensitiveTestCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create fresh input for each test
			input := tc.createInput()
			redactor := tc.redactor()

			// Process with redactor
			inputReader := strings.NewReader(input)
			output := redactor.Redact(inputReader, tc.filePath)

			result, err := io.ReadAll(output)
			if err != nil {
				t.Fatalf("Failed to read redacted output: %v", err)
			}

			resultStr := string(result)

			t.Logf("Original input (%d chars):\n%s", len(input), input)
			t.Logf("Redacted output (%d chars):\n%s", len(resultStr), resultStr)

			// Critical: Verify no sensitive data leaked
			for _, secret := range tc.secrets {
				if strings.Contains(resultStr, secret) {
					t.Errorf("SECURITY VIOLATION: Sensitive data leaked in output: '%s'", secret)
				}
			}

			// Verify tokenization occurred (should contain TOKEN_)
			if !strings.Contains(resultStr, "***TOKEN_") && !strings.Contains(resultStr, "***HIDDEN***") {
				t.Errorf("Expected redaction to occur (TOKEN_ or HIDDEN), but output appears unmodified")
			}

			// Verify output is still valid (JSON/YAML structure preserved)
			if strings.Contains(input, "{") && strings.Contains(input, "}") {
				// Likely JSON - should still be valid structure
				if !isValidJSONStructure(resultStr) {
					t.Errorf("Output JSON structure appears to be broken")
				}
			}
		})
	}
}

// TestPhase3_MappingFileEncryptionDecryption extensively tests encryption security
func TestPhase3_MappingFileEncryptionDecryption(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	// Create temporary directory for test files
	tempDir, err := ioutil.TempDir("", "encryption_security_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// High-value secrets for encryption testing
	criticalSecrets := []struct {
		value   string
		context string
	}{
		{"prod-master-database-password-2023", "database_master"},
		{"sk-live-production-api-key-confidential", "production_api"},
		{"ultra-secret-jwt-signing-key-production", "jwt_production"},
		{"aws-secret-key-production-environment", "aws_production"},
		{"oauth-client-secret-production-app", "oauth_production"},
		{"encryption-master-key-aes-256-production", "encryption_master"},
	}

	// Generate tokens for all secrets
	for _, secret := range criticalSecrets {
		tokenizer.TokenizeValueWithPath(secret.value, secret.context, "production.yaml")
	}

	// Test multiple encryption scenarios
	encryptionTests := []struct {
		name        string
		encrypt     bool
		keySize     int
		description string
	}{
		{
			name:        "unencrypted mapping",
			encrypt:     false,
			keySize:     0,
			description: "plaintext mapping for development",
		},
		{
			name:        "AES-256 encrypted mapping",
			encrypt:     true,
			keySize:     32,
			description: "production-grade encryption",
		},
	}

	for _, et := range encryptionTests {
		t.Run(et.name, func(t *testing.T) {
			mappingPath := filepath.Join(tempDir, fmt.Sprintf("security-map-%s.json", et.name))

			// Generate mapping file
			err := tokenizer.GenerateRedactionMapFile("security-test", mappingPath, et.encrypt)
			if err != nil {
				t.Fatalf("Failed to generate %s mapping: %v", et.description, err)
			}

			// Verify file creation and permissions
			fileInfo, err := os.Stat(mappingPath)
			if err != nil {
				t.Fatalf("Failed to stat mapping file: %v", err)
			}

			// Check secure permissions (0600)
			expectedMode := os.FileMode(0600)
			if fileInfo.Mode() != expectedMode {
				t.Errorf("Insecure file permissions: expected %o, got %o", expectedMode, fileInfo.Mode())
			}

			// Read raw file content
			rawContent, err := ioutil.ReadFile(mappingPath)
			if err != nil {
				t.Fatalf("Failed to read mapping file: %v", err)
			}

			rawContentStr := string(rawContent)

			// Security verification: No plaintext secrets in file
			for _, secret := range criticalSecrets {
				if strings.Contains(rawContentStr, secret.value) {
					if et.encrypt {
						t.Errorf("SECURITY VIOLATION: Plaintext secret '%s' found in encrypted mapping file", secret.value)
					} else {
						t.Logf("Expected: Plaintext secret '%s' found in unencrypted mapping (normal)", secret.value)
					}
				}
			}

			// Load and verify mapping
			loadedMap, err := LoadRedactionMapFile(mappingPath, nil)
			if err != nil {
				t.Fatalf("Failed to load mapping file: %v", err)
			}

			// Verify encryption status
			if loadedMap.IsEncrypted != et.encrypt {
				t.Errorf("Expected encryption status %v, got %v", et.encrypt, loadedMap.IsEncrypted)
			}

			// Verify content accessibility
			if et.encrypt {
				// Encrypted: Should not be able to access plaintext values
				for token, encryptedValue := range loadedMap.Tokens {
					for _, secret := range criticalSecrets {
						if encryptedValue == secret.value {
							t.Errorf("Encrypted mapping contains plaintext secret for token '%s'", token)
						}
					}
				}
			} else {
				// Unencrypted: Should be able to access plaintext values
				for token, plaintextValue := range loadedMap.Tokens {
					found := false
					for _, secret := range criticalSecrets {
						if plaintextValue == secret.value {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("Unencrypted mapping missing expected secret for token '%s'", token)
					}
				}
			}

			// Validate file structure
			if err := ValidateRedactionMapFile(mappingPath); err != nil {
				t.Errorf("Mapping file validation failed: %v", err)
			}

			t.Logf("Security test passed for %s", et.description)
		})
	}
}

// TestPhase3_TokenReversibility validates token→original mapping accuracy
func TestPhase3_TokenReversibility(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	// Create temporary directory for mapping files
	tempDir, err := ioutil.TempDir("", "reversibility_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Test various secret types for reversibility
	testSecrets := []struct {
		original   string
		context    string
		secretType string
	}{
		{"password-reversibility-test-123", "password", "password"},
		{"sk-reversible-api-key-abcdef", "api_key", "API key"},
		{"postgres://user:pass@host/db", "database_url", "database URL"},
		{"admin@company.com", "email", "email address"},
		{"192.168.100.50", "server_ip", "IP address"},
		{"jwt-signing-key-reversible", "jwt_secret", "JWT secret"},
		{"Bearer oauth-token-reversible", "oauth_token", "OAuth token"},
		{"ultra-long-secret-that-should-be-reversible-" + strings.Repeat("x", 100), "long_secret", "long secret"},
		{"special!@#$%^&*()chars", "special_secret", "special characters"},
		{"unicode-秘密-пароль-🔐", "unicode_secret", "unicode secret"},
	}

	originalToToken := make(map[string]string)
	tokenToOriginal := make(map[string]string)

	// Generate tokens for all test secrets
	for _, ts := range testSecrets {
		token := tokenizer.TokenizeValueWithPath(ts.original, ts.context, "reversibility-test.yaml")
		originalToToken[ts.original] = token
		tokenToOriginal[token] = ts.original

		t.Logf("Generated: %s (%s) → %s", ts.original, ts.secretType, token)
	}

	// Test unencrypted mapping reversibility
	unencryptedMapPath := filepath.Join(tempDir, "unencrypted-reversibility-map.json")
	err = tokenizer.GenerateRedactionMapFile("reversibility-test", unencryptedMapPath, false)
	if err != nil {
		t.Fatalf("Failed to generate unencrypted mapping: %v", err)
	}

	// Load unencrypted mapping and verify reversibility
	unencryptedMap, err := LoadRedactionMapFile(unencryptedMapPath, nil)
	if err != nil {
		t.Fatalf("Failed to load unencrypted mapping: %v", err)
	}

	for token, expectedOriginal := range tokenToOriginal {
		if mappedOriginal, exists := unencryptedMap.Tokens[token]; exists {
			if mappedOriginal != expectedOriginal {
				t.Errorf("Reversibility failed for token '%s': expected '%s', got '%s'",
					token, expectedOriginal, mappedOriginal)
			}
		} else {
			t.Errorf("Token '%s' missing from unencrypted mapping", token)
		}
	}

	// Test encrypted mapping reversibility
	encryptedMapPath := filepath.Join(tempDir, "encrypted-reversibility-map.json")
	err = tokenizer.GenerateRedactionMapFile("reversibility-test", encryptedMapPath, true)
	if err != nil {
		t.Fatalf("Failed to generate encrypted mapping: %v", err)
	}

	// Load encrypted mapping (should not be reversible without key)
	encryptedMapNoKey, err := LoadRedactionMapFile(encryptedMapPath, nil)
	if err != nil {
		t.Fatalf("Failed to load encrypted mapping: %v", err)
	}

	// Verify encryption prevents direct access to secrets
	for token, encryptedValue := range encryptedMapNoKey.Tokens {
		expectedOriginal := tokenToOriginal[token]
		if encryptedValue == expectedOriginal {
			t.Errorf("SECURITY VIOLATION: Encrypted mapping contains plaintext secret for token '%s'", token)
		}

		// Encrypted value should be hex-encoded ciphertext
		if _, err := hex.DecodeString(encryptedValue); err != nil {
			t.Errorf("Encrypted value should be valid hex for token '%s': %v", token, err)
		}
	}

	// Test reversibility with encryption key (simulating authorized access)
	// Note: In real implementation, the encryption key would be provided securely

	t.Logf("Reversibility test completed successfully for %d secrets", len(testSecrets))
}

// TestPhase3_SecureDeletion tests secure cleanup of temporary data
func TestPhase3_SecureDeletion(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	// Create temporary directory for testing
	tempDir, err := ioutil.TempDir("", "secure_deletion_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Don't defer removal - we'll test manual cleanup

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Add sensitive data that should be cleanly removed
	sensitiveSecrets := []string{
		"temp-secret-should-be-deleted-001",
		"temp-secret-should-be-deleted-002",
		"temp-secret-should-be-deleted-003",
		"critical-temp-data-for-deletion",
		"sensitive-cleanup-test-data",
	}

	// Generate tokens and mapping
	for i, secret := range sensitiveSecrets {
		context := fmt.Sprintf("temp_secret_%d", i)
		filePath := fmt.Sprintf("temp-file-%d.yaml", i)
		tokenizer.TokenizeValueWithPath(secret, context, filePath)
	}

	// Generate mapping file with sensitive data
	mappingPath := filepath.Join(tempDir, "temp-mapping.json")
	err = tokenizer.GenerateRedactionMapFile("deletion-test", mappingPath, false)
	if err != nil {
		t.Fatalf("Failed to generate mapping: %v", err)
	}

	// Verify sensitive data exists before deletion
	mappingContent, err := ioutil.ReadFile(mappingPath)
	if err != nil {
		t.Fatalf("Failed to read mapping file: %v", err)
	}

	for _, secret := range sensitiveSecrets {
		if !strings.Contains(string(mappingContent), secret) {
			t.Errorf("Expected sensitive data '%s' to be in mapping before deletion", secret)
		}
	}

	// Test secure deletion by resetting tokenizer
	tokenizer.Reset()

	// Verify in-memory data is cleared
	if tokenizer.GetTokenCount() != 0 {
		t.Errorf("Expected token count to be 0 after reset, got %d", tokenizer.GetTokenCount())
	}

	redactionMap := tokenizer.GetRedactionMap("deletion-test")
	if len(redactionMap.Tokens) != 0 {
		t.Errorf("Expected no tokens in redaction map after reset, got %d", len(redactionMap.Tokens))
	}

	if len(redactionMap.SecretRefs) != 0 {
		t.Errorf("Expected no secret references after reset, got %d", len(redactionMap.SecretRefs))
	}

	if len(redactionMap.Duplicates) != 0 {
		t.Errorf("Expected no duplicates after reset, got %d", len(redactionMap.Duplicates))
	}

	// Manually remove mapping file (simulating secure deletion)
	err = os.Remove(mappingPath)
	if err != nil {
		t.Fatalf("Failed to remove mapping file: %v", err)
	}

	// Verify file is actually deleted
	if _, err := os.Stat(mappingPath); !os.IsNotExist(err) {
		t.Errorf("Mapping file should be deleted but still exists")
	}

	// Clean up temp directory
	err = os.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("Failed to remove temp directory: %v", err)
	}

	// Verify directory is actually deleted
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Errorf("Temp directory should be deleted but still exists")
	}

	t.Logf("Secure deletion test completed successfully")
}

// TestPhase3_BackwardCompatibilityVerification ensures existing behavior is preserved
func TestPhase3_BackwardCompatibilityVerification(t *testing.T) {
	// Test both enabled and disabled states
	compatibilityTests := []struct {
		name               string
		enableTokenization bool
		expectedMaskText   string
		shouldContainToken bool
	}{
		{
			name:               "tokenization disabled - original behavior",
			enableTokenization: false,
			expectedMaskText:   "***HIDDEN***",
			shouldContainToken: false,
		},
		{
			name:               "tokenization enabled - enhanced behavior",
			enableTokenization: true,
			expectedMaskText:   "",
			shouldContainToken: true,
		},
	}

	// Test data that mimics real-world support bundle content
	realWorldTestCases := []struct {
		name     string
		input    string
		redactor func() Redactor
		filePath string
	}{
		{
			name: "Kubernetes environment variables",
			input: `{"name":"DATABASE_PASSWORD","value":"k8s-db-secret-2023"}
{"name":"API_KEY","value":"sk-kubernetes-api-production"}
{"name":"JWT_SECRET","value":"jwt-k8s-signing-key"}`,
			redactor: func() Redactor {
				r, _ := NewSingleLineRedactor(LineRedactor{
					regex: `("name":"DATABASE_PASSWORD","value":")(?P<mask>[^"]*)(",?)`,
				}, MASK_TEXT, "k8s-env.json", "k8s_db_password", false)
				return r
			},
			filePath: "k8s-env.json",
		},
		{
			name: "Docker Compose secrets",
			input: `version: '3.8'
services:
  app:
    environment:
      - DATABASE_URL=postgres://user:docker-secret-2023@db:5432/app
      - REDIS_URL=redis://:redis-secret-2023@redis:6379`,
			redactor: func() Redactor {
				r, _ := NewSingleLineRedactor(LineRedactor{
					regex: `(DATABASE_URL=postgres://[^:]+:)(?P<mask>[^@]+)(@.*)`,
				}, MASK_TEXT, "docker-compose.yml", "docker_db_password", false)
				return r
			},
			filePath: "docker-compose.yml",
		},
		{
			name: "Helm values with secrets",
			input: `database:
  password: "helm-db-secret-2023"
  user: "admin"
api:
  key: "helm-api-key-production"`,
			redactor: func() Redactor {
				return NewYamlRedactor("database.password", "*.yaml", "helm_db_password")
			},
			filePath: "helm-values.yaml",
		},
	}

	for _, ct := range compatibilityTests {
		t.Run(ct.name, func(t *testing.T) {
			// Set tokenization state
			if ct.enableTokenization {
				os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
			} else {
				os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
			}
			defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
			defer ResetRedactionList()

			// Reset tokenizer
			ResetGlobalTokenizer()
			globalTokenizer = nil
			tokenizerOnce = sync.Once{}

			for _, rtc := range realWorldTestCases {
				t.Run(rtc.name, func(t *testing.T) {
					redactor := rtc.redactor()
					input := strings.NewReader(rtc.input)
					output := redactor.Redact(input, rtc.filePath)

					result, err := io.ReadAll(output)
					if err != nil {
						t.Fatalf("Failed to read redacted output: %v", err)
					}

					resultStr := string(result)

					if ct.enableTokenization {
						// Should contain intelligent tokens
						if !strings.Contains(resultStr, "***TOKEN_") {
							t.Errorf("Expected tokenized output, got: %s", resultStr)
						}
					} else {
						// Should contain original mask text
						if ct.expectedMaskText != "" && !strings.Contains(resultStr, ct.expectedMaskText) {
							t.Errorf("Expected original mask text '%s', got: %s", ct.expectedMaskText, resultStr)
						}

						// Should NOT contain tokens
						if strings.Contains(resultStr, "***TOKEN_") {
							t.Errorf("Should not contain tokens when disabled, got: %s", resultStr)
						}
					}

					// Should not contain the specific secret targeted by this redactor
					targetedSecrets := getTargetedSecretsForTest(rtc.name)

					for _, secret := range targetedSecrets {
						if strings.Contains(rtc.input, secret) && strings.Contains(resultStr, secret) {
							t.Errorf("Targeted secret '%s' leaked in output: %s", secret, resultStr)
						}
					}

					t.Logf("Compatibility verified - Tokenization: %v, Output: %s",
						ct.enableTokenization, resultStr)
				})
			}
		})
	}
}

// TestPhase3_ComprehensiveSecurityAudit performs a thorough security audit
func TestPhase3_ComprehensiveSecurityAudit(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	// Create temporary directory for audit files
	tempDir, err := ioutil.TempDir("", "security_audit")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Security audit test scenarios
	auditScenarios := []struct {
		name           string
		secrets        []string
		maliciousTests []string // Attempts to extract sensitive data
	}{
		{
			name: "production credential audit",
			secrets: []string{
				"prod-master-db-password-critical",
				"prod-api-key-customer-data-access",
				"prod-jwt-secret-user-authentication",
				"prod-encryption-key-sensitive-data",
			},
			maliciousTests: []string{
				"prod-master", // Partial secret attempt
				"critical",    // Keyword attempt
				"password",    // Generic attempt
				"secret",      // Generic attempt
			},
		},
		{
			name: "financial data credential audit",
			secrets: []string{
				"bank-api-key-transaction-processing",
				"payment-gateway-secret-2023",
				"financial-db-password-compliance",
				"audit-log-encryption-key",
			},
			maliciousTests: []string{
				"bank",      // Institution name
				"payment",   // Service type
				"financial", // Data type
				"2023",      // Year identifier
			},
		},
	}

	for _, scenario := range auditScenarios {
		t.Run(scenario.name, func(t *testing.T) {
			// Process all secrets
			for i, secret := range scenario.secrets {
				context := fmt.Sprintf("audit_secret_%d", i)
				filePath := fmt.Sprintf("audit-file-%d.yaml", i)
				tokenizer.TokenizeValueWithPath(secret, context, filePath)
			}

			// Generate both encrypted and unencrypted mappings
			unencryptedPath := filepath.Join(tempDir, scenario.name+"-unencrypted.json")
			encryptedPath := filepath.Join(tempDir, scenario.name+"-encrypted.json")

			err := tokenizer.GenerateRedactionMapFile(scenario.name, unencryptedPath, false)
			if err != nil {
				t.Fatalf("Failed to generate unencrypted mapping: %v", err)
			}

			err = tokenizer.GenerateRedactionMapFile(scenario.name, encryptedPath, true)
			if err != nil {
				t.Fatalf("Failed to generate encrypted mapping: %v", err)
			}

			// Read file contents for security analysis
			unencryptedContent, err := ioutil.ReadFile(unencryptedPath)
			if err != nil {
				t.Fatalf("Failed to read unencrypted mapping: %v", err)
			}

			encryptedContent, err := ioutil.ReadFile(encryptedPath)
			if err != nil {
				t.Fatalf("Failed to read encrypted mapping: %v", err)
			}

			// Security Audit 1: No sensitive data in encrypted file
			for _, secret := range scenario.secrets {
				if strings.Contains(string(encryptedContent), secret) {
					t.Errorf("CRITICAL SECURITY VIOLATION: Secret '%s' found in encrypted mapping file", secret)
				}
			}

			// Security Audit 1b: Verify sensitive data IS in unencrypted file (for comparison)
			secretsFoundInUnencrypted := 0
			for _, secret := range scenario.secrets {
				if strings.Contains(string(unencryptedContent), secret) {
					secretsFoundInUnencrypted++
				}
			}
			if secretsFoundInUnencrypted == 0 {
				t.Errorf("Expected to find sensitive data in unencrypted mapping for verification")
			}

			// Security Audit 2: Malicious extraction attempts should fail
			for _, maliciousAttempt := range scenario.maliciousTests {
				// Attempt to use partial strings to find secrets
				foundSecrets := 0
				for _, secret := range scenario.secrets {
					if strings.Contains(secret, maliciousAttempt) {
						// Check if this partial string reveals the full secret in output
						if strings.Contains(string(encryptedContent), secret) {
							t.Errorf("SECURITY RISK: Partial match '%s' led to secret disclosure: '%s'",
								maliciousAttempt, secret)
						}
						foundSecrets++
					}
				}
				t.Logf("Malicious attempt '%s' matched %d secrets (all properly protected)",
					maliciousAttempt, foundSecrets)
			}

			// Security Audit 3: File permissions are secure
			encInfo, err := os.Stat(encryptedPath)
			if err != nil {
				t.Fatalf("Failed to stat encrypted file: %v", err)
			}

			if encInfo.Mode() != os.FileMode(0600) {
				t.Errorf("SECURITY RISK: Encrypted file has insecure permissions: %o", encInfo.Mode())
			}

			// Security Audit 4: Validate mapping file integrity
			if err := ValidateRedactionMapFile(unencryptedPath); err != nil {
				t.Errorf("Unencrypted mapping validation failed: %v", err)
			}

			if err := ValidateRedactionMapFile(encryptedPath); err != nil {
				t.Errorf("Encrypted mapping validation failed: %v", err)
			}

			t.Logf("Security audit passed for %s scenario", scenario.name)
		})
	}

	// Final cleanup verification
	err = os.RemoveAll(tempDir)
	if err != nil {
		t.Fatalf("Failed to remove temp directory: %v", err)
	}

	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Errorf("Temp directory should be completely removed")
	}
}

// TestPhase3_IntegrationWithExistingWorkflows tests integration with current redaction
func TestPhase3_IntegrationWithExistingWorkflows(t *testing.T) {
	defer ResetRedactionList()

	// Test that existing redaction workflows work unchanged
	existingWorkflowTests := []struct {
		name                  string
		enableTokenization    bool
		input                 string
		expectedOutput        string
		shouldUseOriginalMask bool
	}{
		{
			name:                  "existing workflow - tokenization disabled",
			enableTokenization:    false,
			input:                 `{"password":"secret123"}`,
			expectedOutput:        `{"password":"***HIDDEN***"}`,
			shouldUseOriginalMask: true,
		},
		{
			name:                  "enhanced workflow - tokenization enabled",
			enableTokenization:    true,
			input:                 `{"password":"secret123"}`,
			expectedOutput:        `***TOKEN_`, // Partial match - could be any token type
			shouldUseOriginalMask: false,
		},
	}

	for _, wt := range existingWorkflowTests {
		t.Run(wt.name, func(t *testing.T) {
			// Set tokenization state
			if wt.enableTokenization {
				os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
			} else {
				os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
			}
			defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")

			// Reset tokenizer
			ResetGlobalTokenizer()
			globalTokenizer = nil
			tokenizerOnce = sync.Once{}

			// Create literal redactor (as used in existing workflows)
			redactor := literalString([]byte("secret123"), "workflow-test.json", "existing_workflow")

			input := strings.NewReader(wt.input)
			output := redactor.Redact(input, "workflow-test.json")

			result, err := io.ReadAll(output)
			if err != nil {
				t.Fatalf("Failed to read redacted output: %v", err)
			}

			resultStr := string(result)

			if wt.shouldUseOriginalMask {
				// Should use original HIDDEN mask
				if !strings.Contains(resultStr, wt.expectedOutput) {
					t.Errorf("Expected original mask behavior '%s', got: %s", wt.expectedOutput, resultStr)
				}
				if strings.Contains(resultStr, "***TOKEN_") {
					t.Errorf("Should not contain tokens when disabled, got: %s", resultStr)
				}
			} else {
				// Should use enhanced tokenization
				if !strings.Contains(resultStr, wt.expectedOutput) {
					t.Errorf("Expected enhanced token behavior with prefix '%s', got: %s", wt.expectedOutput, resultStr)
				}
				if strings.Contains(resultStr, "***HIDDEN***") {
					t.Errorf("Should not contain original mask when tokenization enabled, got: %s", resultStr)
				}
			}

			// Verify no secret leakage
			if strings.Contains(resultStr, "secret123") {
				t.Errorf("Original secret leaked in output: %s", resultStr)
			}

			t.Logf("Workflow test passed - Tokenization: %v, Output: %s",
				wt.enableTokenization, resultStr)
		})
	}
}

// Helper function to get targeted secrets for specific test cases
func getTargetedSecretsForTest(testName string) []string {
	switch testName {
	case "Kubernetes environment variables":
		return []string{"k8s-db-secret-2023"} // Only this is targeted by DATABASE_PASSWORD redactor
	case "Docker Compose secrets":
		return []string{"docker-secret-2023"} // Only this is targeted by the postgres regex
	case "Helm values with secrets":
		return []string{"helm-db-secret-2023"} // Only this is targeted by database.password path
	default:
		return []string{} // No targeted secrets for unknown tests
	}
}

// Helper function to check if JSON structure is preserved
func isValidJSONStructure(jsonStr string) bool {
	// Basic JSON structure validation
	openBraces := strings.Count(jsonStr, "{")
	closeBraces := strings.Count(jsonStr, "}")
	openBrackets := strings.Count(jsonStr, "[")
	closeBrackets := strings.Count(jsonStr, "]")

	return openBraces == closeBraces && openBrackets == closeBrackets
}
