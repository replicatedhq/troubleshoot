package redact

import (
	"os"
	"strings"
	"sync"
	"testing"
)

func TestTokenizer_Phase2_SecretNormalization(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tests := []struct {
		name       string
		rawValue   string
		normalized string
		shouldNorm bool
	}{
		{
			name:       "trim whitespace",
			rawValue:   "  secret123  ",
			normalized: "secret123",
			shouldNorm: true,
		},
		{
			name:       "remove Bearer prefix",
			rawValue:   "Bearer abc123def456",
			normalized: "abc123def456",
			shouldNorm: true,
		},
		{
			name:       "remove quotes",
			rawValue:   `"mysecretpassword"`,
			normalized: "mysecretpassword",
			shouldNorm: true,
		},
		{
			name:       "normalize connection string spacing",
			rawValue:   "user : pass @ host",
			normalized: "user:pass@host",
			shouldNorm: true,
		},
		{
			name:       "API_KEY= prefix removal",
			rawValue:   "API_KEY=sk-1234567890",
			normalized: "sk-1234567890",
			shouldNorm: true,
		},
		{
			name:       "no normalization needed",
			rawValue:   "simplesecret",
			normalized: "simplesecret",
			shouldNorm: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := GetGlobalTokenizer()
			tokenizer.Reset()

			// Test normalization by generating tokens for both versions
			token1 := tokenizer.TokenizeValue(tt.rawValue, "test")
			token2 := tokenizer.TokenizeValue(tt.normalized, "test")

			// If normalization happened, both should produce the same token
			if tt.shouldNorm {
				if token1 != token2 {
					t.Errorf("Expected normalization to produce same token for '%s' and '%s', got '%s' and '%s'",
						tt.rawValue, tt.normalized, token1, token2)
				}
			}

			// Check if normalization was tracked
			if tt.shouldNorm && len(tokenizer.normalizedSecrets) == 0 {
				t.Errorf("Expected normalization to be tracked for '%s'", tt.rawValue)
			}
		})
	}
}

func TestTokenizer_Phase2_CrossFileCorrelation(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Simulate the same secret appearing in multiple files
	secret := "my-database-password"
	files := []string{
		"config/app.yaml",
		"deployment/secrets.yaml",
		"docker-compose.yml",
	}

	tokens := make([]string, len(files))
	for i, file := range files {
		tokens[i] = tokenizer.TokenizeValueWithPath(secret, "database_password", file)
	}

	// All tokens should be identical (cross-file correlation)
	for i := 1; i < len(tokens); i++ {
		if tokens[0] != tokens[i] {
			t.Errorf("Expected same token across files, got '%s' in file %s and '%s' in file %s",
				tokens[0], files[0], tokens[i], files[i])
		}
	}

	// Check that secret references were tracked
	redactionMap := tokenizer.GetRedactionMap("test")

	token := tokens[0]
	if refs, exists := redactionMap.SecretRefs[token]; exists {
		if len(refs) != len(files) {
			t.Errorf("Expected %d file references, got %d: %v", len(files), len(refs), refs)
		}

		// Verify all files are tracked
		for _, expectedFile := range files {
			found := false
			for _, ref := range refs {
				if ref == expectedFile {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected file '%s' to be tracked in references", expectedFile)
			}
		}
	} else {
		t.Errorf("Expected secret references to be tracked for token '%s'", token)
	}
}

func TestTokenizer_Phase2_DuplicateDetection(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Add the same secret in multiple files and locations
	secret := "shared-api-key-secret"
	locations := []struct {
		file    string
		context string
	}{
		{"app/config.yaml", "api_key"},
		{"deploy/secrets.yaml", "api_key"},
		{"docker/compose.yml", "api_key"},
		{"helm/values.yaml", "api_key"},
	}

	token := ""
	for _, loc := range locations {
		token = tokenizer.TokenizeValueWithPath(secret, loc.context, loc.file)
	}

	// Get duplicate groups
	duplicates := tokenizer.GetDuplicateGroups()

	if len(duplicates) != 1 {
		t.Errorf("Expected 1 duplicate group, got %d", len(duplicates))
	}

	if len(duplicates) > 0 {
		dup := duplicates[0]

		if dup.Token != token {
			t.Errorf("Expected duplicate token '%s', got '%s'", token, dup.Token)
		}

		if dup.Count != len(locations) {
			t.Errorf("Expected duplicate count %d, got %d", len(locations), dup.Count)
		}

		if len(dup.Locations) != len(locations) {
			t.Errorf("Expected %d locations, got %d: %v", len(locations), len(dup.Locations), dup.Locations)
		}

		if dup.SecretType != "APIKEY" {
			t.Errorf("Expected secret type 'APIKEY', got '%s'", dup.SecretType)
		}
	}
}

func TestTokenizer_Phase2_PerformanceCache(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// First tokenization - should be a cache miss
	secret := "cache-test-secret"
	token1 := tokenizer.TokenizeValueWithPath(secret, "test", "file1.yaml")

	stats1 := tokenizer.GetCacheStats()
	if stats1.Misses != 1 {
		t.Errorf("Expected 1 cache miss, got %d", stats1.Misses)
	}
	if stats1.Hits != 0 {
		t.Errorf("Expected 0 cache hits, got %d", stats1.Hits)
	}

	// Second tokenization - should be a cache hit
	token2 := tokenizer.TokenizeValueWithPath(secret, "test", "file2.yaml")

	stats2 := tokenizer.GetCacheStats()
	if stats2.Misses != 1 {
		t.Errorf("Expected 1 cache miss after second call, got %d", stats2.Misses)
	}
	if stats2.Hits != 1 {
		t.Errorf("Expected 1 cache hit after second call, got %d", stats2.Hits)
	}

	// Tokens should be identical
	if token1 != token2 {
		t.Errorf("Expected same token from cache, got '%s' and '%s'", token1, token2)
	}

	// Test with normalization - should also hit cache
	secretWithWhitespace := "  " + secret + "  "
	token3 := tokenizer.TokenizeValueWithPath(secretWithWhitespace, "test", "file3.yaml")

	stats3 := tokenizer.GetCacheStats()
	if stats3.Hits != 2 {
		t.Errorf("Expected 2 cache hits after normalization, got %d", stats3.Hits)
	}

	if token1 != token3 {
		t.Errorf("Expected normalization to hit cache, got '%s' and '%s'", token1, token3)
	}
}

func TestTokenizer_Phase2_CorrelationAnalysis(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Simulate database credentials in same files
	dbHost := "db.example.com"
	dbUser := "admin"
	dbPass := "supersecret"
	dbName := "production"

	files := []string{"app.yaml", "docker-compose.yml"}

	for _, file := range files {
		tokenizer.TokenizeValueWithPath(dbHost, "database_host", file)
		tokenizer.TokenizeValueWithPath(dbUser, "database_user", file)
		tokenizer.TokenizeValueWithPath(dbPass, "database_password", file)
		tokenizer.TokenizeValueWithPath(dbName, "database_name", file)
	}

	// Simulate AWS credentials
	awsAccessKey := "AKIA1234567890"
	awsSecretKey := "secret-access-key-abc123"

	for _, file := range files {
		tokenizer.TokenizeValueWithPath(awsAccessKey, "aws_access_key", file)
		tokenizer.TokenizeValueWithPath(awsSecretKey, "aws_secret_key", file)
	}

	// Get redaction map with correlations
	redactionMap := tokenizer.GetRedactionMap("test")

	// Should have detected correlations
	if len(redactionMap.Correlations) == 0 {
		t.Error("Expected correlations to be detected")
	}

	// Check for database correlation
	foundDBCorrelation := false
	foundAWSCorrelation := false

	for _, correlation := range redactionMap.Correlations {
		if correlation.Pattern == "database_credentials" {
			foundDBCorrelation = true
			if len(correlation.Tokens) < 2 {
				t.Errorf("Expected multiple database tokens in correlation, got %d", len(correlation.Tokens))
			}
			if correlation.Confidence < 0.5 {
				t.Errorf("Expected reasonable confidence for database correlation, got %f", correlation.Confidence)
			}
		}
		if correlation.Pattern == "aws_credentials" {
			foundAWSCorrelation = true
			if len(correlation.Tokens) < 2 {
				t.Errorf("Expected multiple AWS tokens in correlation, got %d", len(correlation.Tokens))
			}
		}
	}

	if !foundDBCorrelation {
		t.Error("Expected to find database credential correlation")
	}
	if !foundAWSCorrelation {
		t.Error("Expected to find AWS credential correlation")
	}
}

func TestTokenizer_Phase2_FileStatistics(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()
	tokenizer.Reset()

	// Add secrets to different files
	files := map[string][]struct {
		secret  string
		context string
	}{
		"config.yaml": {
			{"password123", "password"},
			{"api-key-456", "api_key"},
		},
		"secrets.yaml": {
			{"database-secret", "database"},
			{"email@example.com", "email"},
			{"192.168.1.100", "ip"},
		},
		"deployment.yaml": {
			{"token-789", "token"},
		},
	}

	for filePath, secrets := range files {
		for _, s := range secrets {
			tokenizer.TokenizeValueWithPath(s.secret, s.context, filePath)
		}
	}

	// Get redaction map
	redactionMap := tokenizer.GetRedactionMap("test")

	// Validate file coverage statistics
	if redactionMap.Stats.FilesCovered != len(files) {
		t.Errorf("Expected %d files covered, got %d", len(files), redactionMap.Stats.FilesCovered)
	}

	// Check individual file stats
	for filePath, expectedSecrets := range files {
		fileStats, exists := tokenizer.GetFileStats(filePath)
		if !exists {
			t.Errorf("Expected file stats for '%s'", filePath)
			continue
		}

		if fileStats.SecretsFound != len(expectedSecrets) {
			t.Errorf("Expected %d secrets in '%s', got %d",
				len(expectedSecrets), filePath, fileStats.SecretsFound)
		}

		if fileStats.TokensUsed != len(expectedSecrets) {
			t.Errorf("Expected %d tokens used in '%s', got %d",
				len(expectedSecrets), filePath, fileStats.TokensUsed)
		}

		// Verify secret types are tracked
		for _, secret := range expectedSecrets {
			expectedType := strings.ToUpper(secret.context)
			if expectedType == "API_KEY" {
				expectedType = "APIKEY"
			}

			found := false
			for secretType := range fileStats.SecretTypes {
				if secretType == expectedType {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Expected secret type '%s' to be tracked in file '%s', types: %v",
					expectedType, filePath, fileStats.SecretTypes)
			}
		}
	}
}

func TestTokenizer_Phase2_BundleIdentification(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList()

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tokenizer := GetGlobalTokenizer()

	// Test bundle ID generation
	bundleID := tokenizer.GetBundleID()
	if bundleID == "" {
		t.Error("Expected non-empty bundle ID")
	}

	if !strings.HasPrefix(bundleID, "bundle_") {
		t.Errorf("Expected bundle ID to start with 'bundle_', got '%s'", bundleID)
	}

	// Different tokenizer instances should have different bundle IDs
	tokenizer2 := NewTokenizer(TokenizerConfig{
		Enabled: true,
		Salt:    []byte("different-salt"),
	})

	bundleID2 := tokenizer2.GetBundleID()
	if bundleID == bundleID2 {
		t.Errorf("Expected different bundle IDs, got same ID: '%s'", bundleID)
	}

	// Redaction map should include bundle ID
	tokenizer.TokenizeValueWithPath("test-secret", "test", "test.yaml")
	redactionMap := tokenizer.GetRedactionMap("test")

	if redactionMap.BundleID != bundleID {
		t.Errorf("Expected redaction map bundle ID '%s', got '%s'", bundleID, redactionMap.BundleID)
	}
}
