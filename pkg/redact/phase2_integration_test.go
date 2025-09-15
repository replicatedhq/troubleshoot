package redact

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPhase2_Integration_CrossFileCorrelation(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	// Create temporary directory for test output
	tempDir, err := ioutil.TempDir("", "phase2_integration_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Test data: Same database credentials across different file types and redactor types
	testFiles := []struct {
		name     string
		content  string
		redactor func() Redactor
	}{
		{
			name: "kubernetes_secret.yaml",
			content: `apiVersion: v1
kind: Secret
data:
  database_password: "shared-db-password-123"
  database_user: "shared-db-user"
  database_host: "shared-db-host.example.com"`,
			redactor: func() Redactor {
				return NewYamlRedactor("data.database_password", "*.yaml", "k8s_secret_password")
			},
		},
		{
			name: "docker_compose.yml",
			content: `version: '3'
services:
  app:
    environment:
      - DB_PASSWORD=shared-db-password-123
      - DB_USER=shared-db-user
      - DB_HOST=shared-db-host.example.com`,
			redactor: func() Redactor {
				redactor, _ := NewSingleLineRedactor(LineRedactor{
					regex: `DB_PASSWORD=(?P<mask>[^\s]+)`,
				}, MASK_TEXT, "docker_compose.yml", "docker_db_password", false)
				return redactor
			},
		},
		{
			name: "config.json",
			content: `{
  "database": {
    "password": "shared-db-password-123",
    "user": "shared-db-user",
    "host": "shared-db-host.example.com"
  }
}`,
			redactor: func() Redactor {
				return literalString([]byte("shared-db-password-123"), "config.json", "json_db_password")
			},
		},
		{
			name: "env_vars.txt",
			content: `DB_PASSWORD=shared-db-password-123
DB_USER=shared-db-user
DB_HOST=shared-db-host.example.com`,
			redactor: func() Redactor {
				redactor, _ := NewSingleLineRedactor(LineRedactor{
					regex: `DB_PASSWORD=(?P<mask>[^\s]+)`,
				}, MASK_TEXT, "env_vars.txt", "env_db_password", false)
				return redactor
			},
		},
	}

	// Process each file with its respective redactor
	tokens := make(map[string]string) // file -> token for shared password
	for _, testFile := range testFiles {
		input := strings.NewReader(testFile.content)
		redactor := testFile.redactor()

		output := redactor.Redact(input, testFile.name)
		result, err := io.ReadAll(output)
		if err != nil {
			t.Fatalf("Failed to redact file %s: %v", testFile.name, err)
		}

		resultStr := string(result)
		t.Logf("File: %s", testFile.name)
		t.Logf("Output:\n%s", resultStr)

		// Extract the token that was generated
		if strings.Contains(resultStr, "***TOKEN_") {
			// Find the token in the output
			parts := strings.Split(resultStr, "***TOKEN_")
			if len(parts) >= 2 {
				tokenPart := strings.Split(parts[1], "***")[0]
				fullToken := "***TOKEN_" + tokenPart + "***"
				tokens[testFile.name] = fullToken
			}
		}
	}

	// Verify cross-file correlation: All files should use the same token for the same secret
	if len(tokens) < 2 {
		t.Fatalf("Expected tokens to be generated for multiple files, got %d", len(tokens))
	}

	expectedToken := ""
	for file, token := range tokens {
		if expectedToken == "" {
			expectedToken = token
		} else if token != expectedToken {
			t.Errorf("Expected same token across files for shared secret, got '%s' in %s and '%s' in first file",
				token, file, expectedToken)
		}
	}

	// Generate comprehensive redaction map
	tokenizer := GetGlobalTokenizer()
	mappingPath := filepath.Join(tempDir, "integration-redaction-map.json")
	err = tokenizer.GenerateRedactionMapFile("integration-test", mappingPath, false)
	if err != nil {
		t.Fatalf("Failed to generate redaction map: %v", err)
	}

	// Load and validate the redaction map
	redactionMap, err := LoadRedactionMapFile(mappingPath, nil)
	if err != nil {
		t.Fatalf("Failed to load redaction map: %v", err)
	}

	// Should have detected duplicates
	if len(redactionMap.Duplicates) == 0 {
		t.Error("Expected duplicate detection across files")
	}

	// Should have file coverage for all test files
	if redactionMap.Stats.FilesCovered < len(testFiles) {
		t.Errorf("Expected at least %d files covered, got %d",
			len(testFiles), redactionMap.Stats.FilesCovered)
	}

	// Should have secret references tracking
	foundRefs := false
	for token, refs := range redactionMap.SecretRefs {
		if len(refs) > 1 {
			foundRefs = true
			t.Logf("Token %s found in files: %v", token, refs)
		}
	}
	if !foundRefs {
		t.Error("Expected to find tokens referenced in multiple files")
	}

	// Validate the redaction map file
	if err := ValidateRedactionMapFile(mappingPath); err != nil {
		t.Errorf("Redaction map validation failed: %v", err)
	}
}

func TestPhase2_Integration_EncryptedWorkflow(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	// Create temporary directory for test output
	tempDir, err := ioutil.TempDir("", "encrypted_workflow_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Simulate processing multiple files with sensitive data
	sensitiveData := []struct {
		file    string
		secrets map[string]string // context -> secret
	}{
		{
			file: "production_config.yaml",
			secrets: map[string]string{
				"database_password": "prod-db-secret-2023",
				"api_key":           "sk-live-1234567890abcdef",
				"jwt_secret":        "jwt-signing-key-production",
			},
		},
		{
			file: "deployment_manifest.yaml",
			secrets: map[string]string{
				"database_password": "prod-db-secret-2023", // Same as above
				"redis_password":    "redis-secret-456",
				"oauth_secret":      "oauth-client-secret-789",
			},
		},
		{
			file: "docker_secrets.env",
			secrets: map[string]string{
				"database_url":   "postgres://user:prod-db-secret-2023@db.prod.com/app",
				"encryption_key": "aes-256-encryption-key-abc",
			},
		},
	}

	// Process all files
	for _, fileData := range sensitiveData {
		for context, secret := range fileData.secrets {
			tokenizer.TokenizeValueWithPath(secret, context, fileData.file)
		}
	}

	// Generate encrypted redaction map
	encryptedMapPath := filepath.Join(tempDir, "encrypted-redaction-map.json")
	err = tokenizer.GenerateRedactionMapFile("production-profile", encryptedMapPath, true)
	if err != nil {
		t.Fatalf("Failed to generate encrypted redaction map: %v", err)
	}

	// Load encrypted map (should work without decryption key for metadata)
	encryptedMap, err := LoadRedactionMapFile(encryptedMapPath, nil)
	if err != nil {
		t.Fatalf("Failed to load encrypted map: %v", err)
	}

	// Verify encryption
	if !encryptedMap.IsEncrypted {
		t.Error("Expected map to be marked as encrypted")
	}

	// Should have comprehensive statistics
	if encryptedMap.Stats.TotalSecrets == 0 {
		t.Error("Expected total secrets to be tracked")
	}

	if encryptedMap.Stats.FilesCovered != len(sensitiveData) {
		t.Errorf("Expected %d files covered, got %d",
			len(sensitiveData), encryptedMap.Stats.FilesCovered)
	}

	if encryptedMap.Stats.DuplicateCount == 0 {
		t.Error("Expected duplicate detection (same database password used twice)")
	}

	// Should have detected correlations
	if len(encryptedMap.Correlations) == 0 {
		t.Error("Expected correlation analysis")
	}

	// Verify secure file permissions
	fileInfo, err := os.Stat(encryptedMapPath)
	if err != nil {
		t.Fatalf("Failed to stat encrypted file: %v", err)
	}

	expectedMode := os.FileMode(0600)
	if fileInfo.Mode() != expectedMode {
		t.Errorf("Expected secure file mode %o, got %o", expectedMode, fileInfo.Mode())
	}

	// Validate file structure
	if err := ValidateRedactionMapFile(encryptedMapPath); err != nil {
		t.Errorf("Encrypted redaction map validation failed: %v", err)
	}
}

func TestPhase2_Integration_PerformanceMetrics(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Generate a large number of secrets to test performance tracking
	baseSecret := "performance-test-secret"
	numFiles := 10
	secretsPerFile := 20

	start := time.Now()

	for fileIdx := 0; fileIdx < numFiles; fileIdx++ {
		fileName := fmt.Sprintf("file_%d.yaml", fileIdx)

		for secretIdx := 0; secretIdx < secretsPerFile; secretIdx++ {
			// Create some unique secrets and some duplicates
			var secret string
			if secretIdx%3 == 0 {
				// Every 3rd secret is a duplicate of the base secret
				secret = baseSecret
			} else {
				secret = fmt.Sprintf("%s_%d_%d", baseSecret, fileIdx, secretIdx)
			}

			context := fmt.Sprintf("secret_%d", secretIdx%5) // Rotate through contexts
			tokenizer.TokenizeValueWithPath(secret, context, fileName)
		}
	}

	processingTime := time.Since(start)

	// Get performance statistics
	cacheStats := tokenizer.GetCacheStats()
	redactionMap := tokenizer.GetRedactionMap("performance-test")

	t.Logf("Processing time: %v", processingTime)
	t.Logf("Cache hits: %d, misses: %d, total: %d", cacheStats.Hits, cacheStats.Misses, cacheStats.Total)
	t.Logf("Total secrets: %d, unique: %d", redactionMap.Stats.TotalSecrets, redactionMap.Stats.UniqueSecrets)
	t.Logf("Files covered: %d", redactionMap.Stats.FilesCovered)
	t.Logf("Duplicates detected: %d", redactionMap.Stats.DuplicateCount)

	// Verify performance expectations
	if cacheStats.Total == 0 {
		t.Error("Expected cache statistics to be tracked")
	}

	// Should have cache hits from duplicates
	expectedDuplicates := numFiles * (secretsPerFile / 3)     // Every 3rd secret is duplicate
	if cacheStats.Hits < int64(expectedDuplicates-numFiles) { // Account for first occurrence
		t.Errorf("Expected at least %d cache hits from duplicates, got %d",
			expectedDuplicates-numFiles, cacheStats.Hits)
	}

	// Should have detected the duplicate secret across files
	if redactionMap.Stats.DuplicateCount == 0 {
		t.Error("Expected duplicate detection across files")
	}

	// File coverage should match
	if redactionMap.Stats.FilesCovered != numFiles {
		t.Errorf("Expected %d files covered, got %d", numFiles, redactionMap.Stats.FilesCovered)
	}

	// Should have reasonable unique vs total secret ratio
	expectedUnique := numFiles*secretsPerFile - expectedDuplicates + 1 // +1 for the base secret
	tolerance := 10                                                    // Allow some tolerance
	if abs(redactionMap.Stats.UniqueSecrets-expectedUnique) > tolerance {
		t.Errorf("Expected approximately %d unique secrets, got %d",
			expectedUnique, redactionMap.Stats.UniqueSecrets)
	}
}

func TestPhase2_Integration_RealisticeScenario(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	// Create temporary directory for test output
	tempDir, err := ioutil.TempDir("", "realistic_scenario_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Realistic scenario: Kubernetes deployment with shared secrets
	// 1. Process ConfigMap with database configuration
	configMapData := `apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
data:
  database_host: "postgres.production.svc.cluster.local"
  database_port: "5432"
  database_name: "production_app"`

	configRedactor := NewYamlRedactor("data.database_host", "*.yaml", "configmap_db_host")
	configInput := strings.NewReader(configMapData)
	configOutput := configRedactor.Redact(configInput, "configmap.yaml")
	configResult, _ := io.ReadAll(configOutput)

	// 2. Process Secret with same database credentials
	secretData := `apiVersion: v1
kind: Secret
metadata:
  name: app-secrets
data:
  database_password: "prod-db-password-secure-2023"
  database_user: "app_user"
  api_key: "sk-live-abcdef1234567890"`

	secretRedactor := NewYamlRedactor("data.database_password", "*.yaml", "secret_db_password")
	secretInput := strings.NewReader(secretData)
	secretOutput := secretRedactor.Redact(secretInput, "secret.yaml")
	secretResult, _ := io.ReadAll(secretOutput)

	// 3. Process Deployment referencing the same secrets via env vars
	deploymentData := `{"name":"DATABASE_PASSWORD","value":"prod-db-password-secure-2023"}
{"name":"DATABASE_USER","value":"app_user"}
{"name":"API_KEY","value":"sk-live-abcdef1234567890"}`

	envRedactor, _ := NewSingleLineRedactor(LineRedactor{
		regex: `("name":"DATABASE_PASSWORD","value":")(?P<mask>[^"]*)(",?)`,
	}, MASK_TEXT, "deployment.yaml", "deployment_db_password", false)
	envInput := strings.NewReader(deploymentData)
	envOutput := envRedactor.Redact(envInput, "deployment.yaml")
	envResult, _ := io.ReadAll(envOutput)

	// 4. Process Docker Compose with literal matches
	dockerData := `version: '3'
services:
  app:
    environment:
      DB_PASSWORD: prod-db-password-secure-2023
      DB_USER: app_user
      API_KEY: sk-live-abcdef1234567890`

	dockerRedactor := literalString([]byte("prod-db-password-secure-2023"), "docker-compose.yml", "docker_db_password")
	dockerInput := strings.NewReader(dockerData)
	dockerOutput := dockerRedactor.Redact(dockerInput, "docker-compose.yml")
	dockerResult, _ := io.ReadAll(dockerOutput)

	// Log all results
	t.Logf("ConfigMap result:\n%s", string(configResult))
	t.Logf("Secret result:\n%s", string(secretResult))
	t.Logf("Deployment result:\n%s", string(envResult))
	t.Logf("Docker result:\n%s", string(dockerResult))

	// Generate comprehensive redaction map
	mappingPath := filepath.Join(tempDir, "realistic-redaction-map.json")
	err = tokenizer.GenerateRedactionMapFile("realistic-test", mappingPath, false)
	if err != nil {
		t.Fatalf("Failed to generate redaction map: %v", err)
	}

	// Load and analyze the redaction map
	redactionMap, err := LoadRedactionMapFile(mappingPath, nil)
	if err != nil {
		t.Fatalf("Failed to load redaction map: %v", err)
	}

	// Comprehensive validation (may be less than input files due to wildcard patterns in YAML redactor)
	if redactionMap.Stats.FilesCovered < 2 {
		t.Errorf("Expected at least 2 files processed, got %d", redactionMap.Stats.FilesCovered)
	}

	if redactionMap.Stats.DuplicateCount == 0 {
		t.Error("Expected duplicate secrets to be detected (same password used across files)")
	}

	if len(redactionMap.Duplicates) == 0 {
		t.Error("Expected duplicate groups to be created")
	}

	// Should have correlation analysis
	if len(redactionMap.Correlations) == 0 {
		t.Error("Expected correlation analysis for database/API credentials")
	}

	// Verify secret references show cross-file usage
	foundCrossFileRefs := false
	for token, refs := range redactionMap.SecretRefs {
		if len(refs) > 1 {
			foundCrossFileRefs = true
			t.Logf("Cross-file token usage: %s in files %v", token, refs)
		}
	}
	if !foundCrossFileRefs {
		t.Error("Expected to find tokens used across multiple files")
	}

	// Cache performance should show hits from duplicates
	if redactionMap.Stats.CacheHits == 0 {
		t.Error("Expected cache hits from duplicate secret processing")
	}

	// Test file validation
	if err := ValidateRedactionMapFile(mappingPath); err != nil {
		t.Errorf("Realistic scenario redaction map validation failed: %v", err)
	}

	t.Logf("Final statistics: %+v", redactionMap.Stats)
}

// Helper function for absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
