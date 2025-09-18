package redact

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestTokenizer_Phase2_MappingFileGeneration(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Create temporary directory for test files
	tempDir, err := ioutil.TempDir("", "redaction_map_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Add some secrets for testing
	secrets := []struct {
		value   string
		context string
		file    string
	}{
		{"password123", "password", "config.yaml"},
		{"api-key-456", "api_key", "config.yaml"},
		{"db-secret", "database", "secrets.yaml"},
		{"password123", "password", "deployment.yaml"}, // Duplicate
	}

	for _, s := range secrets {
		tokenizer.TokenizeValueWithPath(s.value, s.context, s.file)
	}

	// Generate unencrypted mapping file
	mappingPath := filepath.Join(tempDir, "redaction-map.json")
	err = tokenizer.GenerateRedactionMapFile("test-profile", mappingPath, false)
	if err != nil {
		t.Fatalf("Failed to generate mapping file: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(mappingPath); os.IsNotExist(err) {
		t.Fatalf("Mapping file was not created: %s", mappingPath)
	}

	// Load and validate the mapping file
	loadedMap, err := LoadRedactionMapFile(mappingPath, nil)
	if err != nil {
		t.Fatalf("Failed to load mapping file: %v", err)
	}

	// Validate content
	if loadedMap.Profile != "test-profile" {
		t.Errorf("Expected profile 'test-profile', got '%s'", loadedMap.Profile)
	}

	if loadedMap.IsEncrypted {
		t.Error("Expected unencrypted mapping file")
	}

	if len(loadedMap.Tokens) == 0 {
		t.Error("Expected tokens in mapping file")
	}

	if len(loadedMap.Duplicates) == 0 {
		t.Error("Expected duplicates to be detected and included")
	}

	// Should have correlations (database + password patterns)
	if len(loadedMap.Correlations) == 0 {
		t.Error("Expected correlations to be detected")
	}

	// Validate file statistics
	if loadedMap.Stats.FilesCovered != 3 {
		t.Errorf("Expected 3 files covered, got %d", loadedMap.Stats.FilesCovered)
	}

	if len(loadedMap.Stats.FileCoverage) != 3 {
		t.Errorf("Expected 3 file coverage entries, got %d", len(loadedMap.Stats.FileCoverage))
	}
}

func TestTokenizer_Phase2_EncryptedMappingFile(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Create temporary directory for test files
	tempDir, err := ioutil.TempDir("", "encrypted_redaction_map_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Add secrets for testing
	originalSecrets := map[string]string{
		"password": "super-secret-password",
		"api_key":  "sk-1234567890abcdef",
		"token":    "Bearer abc123def456ghi789",
	}

	tokens := make(map[string]string)
	for context, secret := range originalSecrets {
		token := tokenizer.TokenizeValueWithPath(secret, context, "test.yaml")
		tokens[context] = token
	}

	// Generate encrypted mapping file
	encryptedPath := filepath.Join(tempDir, "redaction-map-encrypted.json")
	err = tokenizer.GenerateRedactionMapFile("encrypted-profile", encryptedPath, true)
	if err != nil {
		t.Fatalf("Failed to generate encrypted mapping file: %v", err)
	}

	// Load the encrypted file (without decryption key)
	encryptedMap, err := LoadRedactionMapFile(encryptedPath, nil)
	if err != nil {
		t.Fatalf("Failed to load encrypted mapping file: %v", err)
	}

	// Should be marked as encrypted
	if !encryptedMap.IsEncrypted {
		t.Error("Expected mapping file to be marked as encrypted")
	}

	// Token values should be hex-encoded encrypted data (not plaintext)
	for token, encryptedValue := range encryptedMap.Tokens {
		// Original secret should not appear in encrypted value
		for _, originalSecret := range originalSecrets {
			if strings.Contains(encryptedValue, originalSecret) {
				t.Errorf("Found plaintext secret '%s' in encrypted value for token '%s'",
					originalSecret, token)
			}
		}

		// Encrypted value should be hex-encoded
		if len(encryptedValue) < 24 { // At least nonce + some ciphertext
			t.Errorf("Encrypted value too short for token '%s': %s", token, encryptedValue)
		}
	}

	// Test file validation
	if err := ValidateRedactionMapFile(encryptedPath); err != nil {
		t.Errorf("Encrypted mapping file should validate: %v", err)
	}
}

func TestTokenizer_Phase2_RedactionMapValidation(t *testing.T) {
	// Create temporary directory for test files
	tempDir, err := ioutil.TempDir("", "validation_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name        string
		mapData     RedactionMap
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid mapping file",
			mapData: RedactionMap{
				BundleID: "test-bundle-123",
				Tokens: map[string]string{
					"***TOKEN_PASSWORD_ABC123***": "secret123",
					"***TOKEN_APIKEY_DEF456***":   "sk-1234567890",
				},
				Stats: RedactionStats{
					TotalSecrets: 2,
				},
				Timestamp: time.Now(),
			},
			expectError: false,
		},
		{
			name: "missing bundle ID",
			mapData: RedactionMap{
				BundleID: "",
				Tokens: map[string]string{
					"***TOKEN_PASSWORD_ABC123***": "secret123",
				},
				Stats: RedactionStats{
					TotalSecrets: 1,
				},
			},
			expectError: true,
			errorMsg:    "missing bundle ID",
		},
		{
			name: "stats mismatch",
			mapData: RedactionMap{
				BundleID: "test-bundle-456",
				Tokens: map[string]string{
					"***TOKEN_PASSWORD_ABC123***": "secret123",
					"***TOKEN_APIKEY_DEF456***":   "sk-1234567890",
				},
				Stats: RedactionStats{
					TotalSecrets: 5, // Wrong count
				},
			},
			expectError: true,
			errorMsg:    "stats mismatch",
		},
		{
			name: "invalid token format",
			mapData: RedactionMap{
				BundleID: "test-bundle-789",
				Tokens: map[string]string{
					"INVALID_TOKEN_FORMAT": "secret123",
				},
				Stats: RedactionStats{
					TotalSecrets: 1,
				},
			},
			expectError: true,
			errorMsg:    "invalid token format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test mapping file
			testPath := filepath.Join(tempDir, tt.name+".json")
			jsonData, err := json.MarshalIndent(tt.mapData, "", "  ")
			if err != nil {
				t.Fatalf("Failed to marshal test data: %v", err)
			}

			err = ioutil.WriteFile(testPath, jsonData, 0600)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Validate the file
			err = ValidateRedactionMapFile(testPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected validation error containing '%s', but got no error", tt.errorMsg)
				} else if !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no validation error, got: %v", err)
				}
			}
		})
	}
}

func TestTokenizer_Phase2_EncryptionDecryption(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Add some test secrets
	secrets := map[string]string{
		"password": "my-super-secret-password",
		"api_key":  "sk-abcdef1234567890",
		"database": "postgres://user:pass@host/db",
		"email":    "admin@example.com",
	}

	for context, secret := range secrets {
		tokenizer.TokenizeValueWithPath(secret, context, "test.yaml")
	}

	// Get original redaction map
	originalMap := tokenizer.GetRedactionMap("encryption-test")

	// Test encryption (exactly 32 bytes for AES-256)
	encryptionKey := []byte("12345678901234567890123456789012") // Exactly 32 bytes
	encryptedMap, err := tokenizer.encryptRedactionMap(originalMap, encryptionKey)
	if err != nil {
		t.Fatalf("Failed to encrypt redaction map: %v", err)
	}

	// Verify encryption worked
	for token, encryptedValue := range encryptedMap.Tokens {
		originalValue := originalMap.Tokens[token]

		// Encrypted value should not contain original secret
		if strings.Contains(encryptedValue, originalValue) {
			t.Errorf("Encrypted value contains plaintext secret for token '%s'", token)
		}

		// Encrypted value should be longer than original (due to nonce + auth tag)
		if len(encryptedValue) <= len(originalValue) {
			t.Errorf("Encrypted value should be longer than original for token '%s'", token)
		}
	}

	// Test decryption
	decryptedMap, err := tokenizer.decryptRedactionMap(encryptedMap, encryptionKey)
	if err != nil {
		t.Fatalf("Failed to decrypt redaction map: %v", err)
	}

	// Verify decryption worked
	if decryptedMap.IsEncrypted {
		t.Error("Decrypted map should not be marked as encrypted")
	}

	for token, decryptedValue := range decryptedMap.Tokens {
		expectedValue := originalMap.Tokens[token]
		if decryptedValue != expectedValue {
			t.Errorf("Decrypted value mismatch for token '%s': expected '%s', got '%s'",
				token, expectedValue, decryptedValue)
		}
	}

	// Test with wrong encryption key (must be exactly 32 bytes for AES-256)
	wrongKey := []byte("abcdefghijklmnopqrstuvwxyz123456") // Exactly 32 bytes
	wrongDecrypted, err := tokenizer.decryptRedactionMap(encryptedMap, wrongKey)
	if err != nil {
		// This is expected if decryption fails with error
		t.Logf("Decryption correctly failed with wrong key: %v", err)
	} else {
		// If no error, the decrypted values should not match original values
		for token, wrongValue := range wrongDecrypted.Tokens {
			originalValue := originalMap.Tokens[token]
			if wrongValue == originalValue {
				t.Errorf("Decryption with wrong key should not produce correct value for token '%s'", token)
			}
		}
	}
}

func TestTokenizer_Phase2_AdvancedDuplicateDetection(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Test scenario: Same secret in different formats across files
	baseSecret := "my-shared-secret"
	variations := []struct {
		value   string
		file    string
		context string
	}{
		{baseSecret, "app.yaml", "secret"},
		{" " + baseSecret + " ", "config.yaml", "secret"},        // Whitespace
		{`"` + baseSecret + `"`, "docker-compose.yml", "secret"}, // Quoted
		{"Bearer " + baseSecret, "auth.yaml", "token"},           // Prefixed
	}

	tokens := make([]string, len(variations))
	for i, v := range variations {
		tokens[i] = tokenizer.TokenizeValueWithPath(v.value, v.context, v.file)
	}

	// All tokens should be the same due to normalization
	for i := 1; i < len(tokens); i++ {
		if tokens[0] != tokens[i] {
			t.Errorf("Expected normalized tokens to be identical, got '%s' and '%s' for variations '%s' and '%s'",
				tokens[0], tokens[i], variations[0].value, variations[i].value)
		}
	}

	// Get duplicates
	duplicates := tokenizer.GetDuplicateGroups()
	if len(duplicates) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(duplicates))
	}

	if len(duplicates) > 0 {
		dup := duplicates[0]
		if dup.Count != len(variations) {
			t.Errorf("Expected duplicate count %d, got %d", len(variations), dup.Count)
		}

		if len(dup.Locations) != len(variations) {
			t.Errorf("Expected %d locations, got %d", len(variations), len(dup.Locations))
		}

		// Verify all file locations are tracked
		for _, variation := range variations {
			found := false
			for _, location := range dup.Locations {
				if location == variation.file {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected location '%s' to be tracked in duplicates", variation.file)
			}
		}
	}
}

func TestTokenizer_Phase2_ComplexCorrelationScenarios(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Scenario 1: Complete database configuration across multiple files
	dbSecrets := []struct {
		value   string
		context string
		files   []string
	}{
		{"db.example.com", "database_host", []string{"app.yaml", "docker-compose.yml"}},
		{"admin", "database_user", []string{"app.yaml", "docker-compose.yml"}},
		{"db-password-123", "database_password", []string{"app.yaml", "secrets.yaml"}},
		{"production_db", "database_name", []string{"app.yaml", "docker-compose.yml"}},
	}

	for _, secret := range dbSecrets {
		for _, file := range secret.files {
			tokenizer.TokenizeValueWithPath(secret.value, secret.context, file)
		}
	}

	// Scenario 2: AWS credentials in same deployment
	awsSecrets := []struct {
		value   string
		context string
		files   []string
	}{
		{"AKIA1234567890EXAMPLE", "aws_access_key", []string{"deployment.yaml", "configmap.yaml"}},
		{"aws-secret-key-abcdef", "aws_secret_key", []string{"deployment.yaml", "secret.yaml"}},
		{"us-west-2", "aws_region", []string{"deployment.yaml", "configmap.yaml"}},
	}

	for _, secret := range awsSecrets {
		for _, file := range secret.files {
			tokenizer.TokenizeValueWithPath(secret.value, secret.context, file)
		}
	}

	// Get redaction map with correlations
	redactionMap := tokenizer.GetRedactionMap("correlation-test")

	// Should detect database and AWS correlations
	dbCorrelationFound := false
	awsCorrelationFound := false

	for _, correlation := range redactionMap.Correlations {
		switch correlation.Pattern {
		case "database_credentials":
			dbCorrelationFound = true
			if len(correlation.Tokens) < 2 {
				t.Errorf("Expected multiple database tokens, got %d", len(correlation.Tokens))
			}
			if len(correlation.Files) < 2 {
				t.Errorf("Expected multiple files for database correlation, got %d", len(correlation.Files))
			}
			if correlation.Confidence < 0.5 {
				t.Errorf("Expected reasonable confidence for database correlation, got %f", correlation.Confidence)
			}

		case "aws_credentials":
			awsCorrelationFound = true
			if len(correlation.Tokens) < 2 {
				t.Errorf("Expected multiple AWS tokens, got %d", len(correlation.Tokens))
			}
			if correlation.Confidence < 0.7 {
				t.Errorf("Expected high confidence for AWS correlation, got %f", correlation.Confidence)
			}
		}
	}

	if !dbCorrelationFound {
		t.Error("Expected database credential correlation to be detected")
	}
	if !awsCorrelationFound {
		t.Error("Expected AWS credential correlation to be detected")
	}

	// Verify advanced statistics
	if redactionMap.Stats.DuplicateCount == 0 {
		t.Error("Expected some duplicate secrets to be detected")
	}

	if redactionMap.Stats.CorrelationCount == 0 {
		t.Error("Expected correlations to be counted in stats")
	}

	if redactionMap.Stats.FilesCovered < 4 {
		t.Errorf("Expected at least 4 files covered, got %d", redactionMap.Stats.FilesCovered)
	}
}

func TestTokenizer_Phase2_FileSecurityPermissions(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Create temporary directory for test files
	tempDir, err := ioutil.TempDir("", "security_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Add a secret
	tokenizer.TokenizeValueWithPath("sensitive-data", "secret", "test.yaml")

	// Generate mapping file
	mappingPath := filepath.Join(tempDir, "redaction-map.json")
	err = tokenizer.GenerateRedactionMapFile("security-test", mappingPath, false)
	if err != nil {
		t.Fatalf("Failed to generate mapping file: %v", err)
	}

	// Check file permissions (should be 0600 - owner read/write only)
	fileInfo, err := os.Stat(mappingPath)
	if err != nil {
		t.Fatalf("Failed to stat mapping file: %v", err)
	}

	mode := fileInfo.Mode()
	expectedMode := os.FileMode(0600)
	if mode != expectedMode {
		t.Errorf("Expected file mode %o, got %o", expectedMode, mode)
	}

	// Verify file content is valid JSON
	content, err := ioutil.ReadFile(mappingPath)
	if err != nil {
		t.Fatalf("Failed to read mapping file: %v", err)
	}

	var redactionMap RedactionMap
	if err := json.Unmarshal(content, &redactionMap); err != nil {
		t.Errorf("Mapping file should contain valid JSON: %v", err)
	}

	// Should contain expected data
	if len(redactionMap.Tokens) == 0 {
		t.Error("Expected tokens in mapping file")
	}

	if redactionMap.BundleID == "" {
		t.Error("Expected bundle ID in mapping file")
	}
}
