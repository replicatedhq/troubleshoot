package api

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewInMemoryCredentialStore(t *testing.T) {
	store := NewInMemoryCredentialStore("test-encryption-key")

	assert.NotNil(t, store)
	assert.NotNil(t, store.credentials)
	assert.Len(t, store.encryptionKey, 32) // Should be SHA256 hash
}

func TestInMemoryCredentialStore_SetAndGetCredentials(t *testing.T) {
	store := NewInMemoryCredentialStore("test-key")

	// Test setting and getting credentials
	testCreds := &Credentials{
		Token:    "test-token",
		APIKey:   "test-api-key",
		Username: "testuser",
		Password: "testpass",
		Metadata: map[string]string{"source": "test"},
	}

	err := store.SetCredentials("test-auth", testCreds)
	assert.NoError(t, err)

	retrievedCreds, err := store.GetCredentials("test-auth")
	assert.NoError(t, err)
	assert.NotNil(t, retrievedCreds)
	assert.Equal(t, testCreds.Token, retrievedCreds.Token)
	assert.Equal(t, testCreds.APIKey, retrievedCreds.APIKey)
	assert.Equal(t, testCreds.Username, retrievedCreds.Username)
	assert.Equal(t, testCreds.Password, retrievedCreds.Password)

	// Test that returned credentials are a copy (not same reference)
	retrievedCreds.Token = "modified"
	retrievedCreds2, _ := store.GetCredentials("test-auth")
	assert.Equal(t, "test-token", retrievedCreds2.Token)
}

func TestInMemoryCredentialStore_ExpiredCredentials(t *testing.T) {
	store := NewInMemoryCredentialStore("test-key")

	// Set expired credentials
	expiredCreds := &Credentials{
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired 1 hour ago
	}

	err := store.SetCredentials("expired-auth", expiredCreds)
	assert.NoError(t, err)

	// Should return error for expired credentials
	_, err = store.GetCredentials("expired-auth")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credentials expired")
}

func TestInMemoryCredentialStore_NotFound(t *testing.T) {
	store := NewInMemoryCredentialStore("test-key")

	_, err := store.GetCredentials("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credentials not found")
}

func TestInMemoryCredentialStore_RefreshCredentials(t *testing.T) {
	store := NewInMemoryCredentialStore("test-key")

	_, err := store.RefreshCredentials("test-auth")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "automatic credential refresh not implemented")
}

func TestNewFileCredentialStore(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "creds.enc")

	store := NewFileCredentialStore(filePath, "test-encryption-key")

	assert.NotNil(t, store)
	assert.Equal(t, filePath, store.filePath)
	assert.Len(t, store.encryptionKey, 32)
	assert.NotNil(t, store.cache)
}

func TestFileCredentialStore_SetAndGetCredentials(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_creds.enc")

	store := NewFileCredentialStore(filePath, "test-encryption-key")

	testCreds := &Credentials{
		Token:     "file-test-token",
		APIKey:    "file-test-api-key",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		Metadata:  map[string]string{"source": "file-test"},
	}

	// Set credentials
	err := store.SetCredentials("file-auth", testCreds)
	assert.NoError(t, err)

	// Verify file was created
	assert.FileExists(t, filePath)

	// Get credentials
	retrievedCreds, err := store.GetCredentials("file-auth")
	assert.NoError(t, err)
	assert.Equal(t, testCreds.Token, retrievedCreds.Token)
	assert.Equal(t, testCreds.APIKey, retrievedCreds.APIKey)

	// Create new store instance to test persistence
	store2 := NewFileCredentialStore(filePath, "test-encryption-key")
	retrievedCreds2, err := store2.GetCredentials("file-auth")
	assert.NoError(t, err)
	assert.Equal(t, testCreds.Token, retrievedCreds2.Token)
}

func TestFileCredentialStore_EncryptionDecryption(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "encryption_test.enc")

	store := NewFileCredentialStore(filePath, "secret-key-123")

	originalCreds := &Credentials{
		Token:    "sensitive-token-data",
		APIKey:   "secret-api-key",
		Username: "admin",
		Password: "supersecret",
	}

	// Store credentials (which encrypts them)
	err := store.SetCredentials("encryption-test", originalCreds)
	assert.NoError(t, err)

	// Read raw file content to ensure it's encrypted
	rawData, err := os.ReadFile(filePath)
	assert.NoError(t, err)
	assert.NotContains(t, string(rawData), "sensitive-token-data")
	assert.NotContains(t, string(rawData), "secret-api-key")
	assert.NotContains(t, string(rawData), "supersecret")

	// Retrieve and verify decryption works
	retrievedCreds, err := store.GetCredentials("encryption-test")
	assert.NoError(t, err)
	assert.Equal(t, originalCreds.Token, retrievedCreds.Token)
	assert.Equal(t, originalCreds.APIKey, retrievedCreds.APIKey)
	assert.Equal(t, originalCreds.Password, retrievedCreds.Password)
}

func TestFileCredentialStore_WrongEncryptionKey(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "wrong_key_test.enc")

	// Create store with one key and save credentials
	store1 := NewFileCredentialStore(filePath, "correct-key")
	testCreds := &Credentials{Token: "test-token"}

	err := store1.SetCredentials("test", testCreds)
	assert.NoError(t, err)

	// Try to read with different key
	store2 := NewFileCredentialStore(filePath, "wrong-key")
	_, err = store2.GetCredentials("test")
	assert.Error(t, err)
}

func TestNewEnvironmentCredentialStore(t *testing.T) {
	store := NewEnvironmentCredentialStore()

	assert.NotNil(t, store)
	assert.NotNil(t, store.envMappings)

	// Check default mappings
	bearerMapping, exists := store.envMappings["bearer"]
	assert.True(t, exists)
	assert.Equal(t, "AUTH_BEARER_TOKEN", bearerMapping.TokenVar)

	apiKeyMapping, exists := store.envMappings["api-key"]
	assert.True(t, exists)
	assert.Equal(t, "API_KEY", apiKeyMapping.APIKeyVar)
}

func TestEnvironmentCredentialStore_GetCredentials(t *testing.T) {
	// Set up environment variables
	os.Setenv("TEST_BEARER_TOKEN", "env-bearer-token")
	os.Setenv("TEST_API_KEY", "env-api-key")
	os.Setenv("TEST_USERNAME", "env-user")
	os.Setenv("TEST_PASSWORD", "env-pass")
	os.Setenv("TEST_EXPIRY", time.Now().Add(1*time.Hour).Format(time.RFC3339))

	defer func() {
		os.Unsetenv("TEST_BEARER_TOKEN")
		os.Unsetenv("TEST_API_KEY")
		os.Unsetenv("TEST_USERNAME")
		os.Unsetenv("TEST_PASSWORD")
		os.Unsetenv("TEST_EXPIRY")
	}()

	store := NewEnvironmentCredentialStore()
	store.SetEnvMapping("test-bearer", EnvMapping{
		TokenVar:  "TEST_BEARER_TOKEN",
		ExpiryVar: "TEST_EXPIRY",
	})
	store.SetEnvMapping("test-api", EnvMapping{
		APIKeyVar: "TEST_API_KEY",
	})
	store.SetEnvMapping("test-basic", EnvMapping{
		UsernameVar: "TEST_USERNAME",
		PasswordVar: "TEST_PASSWORD",
	})

	// Test bearer token retrieval
	bearerCreds, err := store.GetCredentials("test-bearer")
	assert.NoError(t, err)
	assert.Equal(t, "env-bearer-token", bearerCreds.Token)
	assert.True(t, bearerCreds.ExpiresAt.After(time.Now()))

	// Test API key retrieval
	apiCreds, err := store.GetCredentials("test-api")
	assert.NoError(t, err)
	assert.Equal(t, "env-api-key", apiCreds.APIKey)

	// Test basic auth retrieval
	basicCreds, err := store.GetCredentials("test-basic")
	assert.NoError(t, err)
	assert.Equal(t, "env-user", basicCreds.Username)
	assert.Equal(t, "env-pass", basicCreds.Password)
}

func TestEnvironmentCredentialStore_NoCredentialsFound(t *testing.T) {
	store := NewEnvironmentCredentialStore()

	_, err := store.GetCredentials("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no environment mapping")

	// Test with mapping but no env vars set
	store.SetEnvMapping("empty-test", EnvMapping{
		TokenVar: "NONEXISTENT_TOKEN",
	})

	_, err = store.GetCredentials("empty-test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no credentials found in environment")
}

func TestEnvironmentCredentialStore_ExpiredCredentials(t *testing.T) {
	os.Setenv("TEST_TOKEN", "token-value")
	os.Setenv("TEST_EXPIRY", time.Now().Add(-1*time.Hour).Format(time.RFC3339))
	defer func() {
		os.Unsetenv("TEST_TOKEN")
		os.Unsetenv("TEST_EXPIRY")
	}()

	store := NewEnvironmentCredentialStore()
	store.SetEnvMapping("expired-test", EnvMapping{
		TokenVar:  "TEST_TOKEN",
		ExpiryVar: "TEST_EXPIRY",
	})

	_, err := store.GetCredentials("expired-test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "credentials expired")
}

func TestEnvironmentCredentialStore_SetCredentialsNotSupported(t *testing.T) {
	store := NewEnvironmentCredentialStore()

	err := store.SetCredentials("test", &Credentials{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "setting credentials not supported")
}

func TestNewChainCredentialStore(t *testing.T) {
	store1 := NewInMemoryCredentialStore("key1")
	store2 := NewInMemoryCredentialStore("key2")

	chainStore := NewChainCredentialStore(store1, store2)

	assert.NotNil(t, chainStore)
	assert.Len(t, chainStore.stores, 2)
}

func TestChainCredentialStore_GetCredentials_FirstStoreSuccess(t *testing.T) {
	store1 := NewInMemoryCredentialStore("key1")
	store2 := NewInMemoryCredentialStore("key2")

	// Set credentials in first store
	testCreds := &Credentials{Token: "chain-test-token"}
	store1.SetCredentials("chain-test", testCreds)

	chainStore := NewChainCredentialStore(store1, store2)

	retrievedCreds, err := chainStore.GetCredentials("chain-test")
	assert.NoError(t, err)
	assert.Equal(t, "chain-test-token", retrievedCreds.Token)
}

func TestChainCredentialStore_GetCredentials_FallbackToSecondStore(t *testing.T) {
	store1 := NewInMemoryCredentialStore("key1")
	store2 := NewInMemoryCredentialStore("key2")

	// Set credentials only in second store
	testCreds := &Credentials{Token: "fallback-token"}
	store2.SetCredentials("fallback-test", testCreds)

	chainStore := NewChainCredentialStore(store1, store2)

	retrievedCreds, err := chainStore.GetCredentials("fallback-test")
	assert.NoError(t, err)
	assert.Equal(t, "fallback-token", retrievedCreds.Token)
}

func TestChainCredentialStore_GetCredentials_AllStoresFail(t *testing.T) {
	store1 := NewInMemoryCredentialStore("key1")
	store2 := NewInMemoryCredentialStore("key2")

	chainStore := NewChainCredentialStore(store1, store2)

	_, err := chainStore.GetCredentials("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "all credential stores failed")
}

func TestCredentialValidator_ValidateCredentials(t *testing.T) {
	validator := &CredentialValidator{}

	tests := []struct {
		name        string
		authType    string
		creds       *Credentials
		shouldError bool
		errorMsg    string
	}{
		{
			name:        "nil credentials",
			authType:    "bearer",
			creds:       nil,
			shouldError: true,
			errorMsg:    "credentials cannot be nil",
		},
		{
			name:     "valid bearer token",
			authType: "bearer",
			creds: &Credentials{
				Token:     "valid-bearer-token-123",
				ExpiresAt: time.Now().Add(1 * time.Hour),
			},
			shouldError: false,
		},
		{
			name:     "expired bearer token",
			authType: "bearer",
			creds: &Credentials{
				Token:     "expired-token",
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			},
			shouldError: true,
			errorMsg:    "credentials are expired",
		},
		{
			name:        "bearer token too short",
			authType:    "bearer",
			creds:       &Credentials{Token: "short"},
			shouldError: true,
			errorMsg:    "bearer token appears to be too short",
		},
		{
			name:        "missing bearer token",
			authType:    "bearer",
			creds:       &Credentials{APIKey: "has-api-key-but-no-token"},
			shouldError: true,
			errorMsg:    "bearer token is required",
		},
		{
			name:     "valid api key",
			authType: "api-key",
			creds: &Credentials{
				APIKey: "valid-api-key-123456",
			},
			shouldError: false,
		},
		{
			name:        "missing api key",
			authType:    "api-key",
			creds:       &Credentials{Token: "has-token-but-no-api-key"},
			shouldError: true,
			errorMsg:    "API key is required",
		},
		{
			name:     "valid basic auth",
			authType: "basic",
			creds: &Credentials{
				Username: "testuser",
				Password: "testpass",
			},
			shouldError: false,
		},
		{
			name:        "missing username for basic auth",
			authType:    "basic",
			creds:       &Credentials{Password: "testpass"},
			shouldError: true,
			errorMsg:    "username and password are required",
		},
		{
			name:     "unknown auth type with credentials",
			authType: "unknown",
			creds: &Credentials{
				Token: "some-token",
			},
			shouldError: false, // Should pass for unknown types if any field is set
		},
		{
			name:        "unknown auth type with no credentials",
			authType:    "unknown",
			creds:       &Credentials{},
			shouldError: true,
			errorMsg:    "no credential fields are set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateCredentials(tt.authType, tt.creds)

			if tt.shouldError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSecureCredentialMasker_MaskCredentials(t *testing.T) {
	masker := &SecureCredentialMasker{}

	tests := []struct {
		name     string
		creds    *Credentials
		expected *Credentials
	}{
		{
			name:     "nil credentials",
			creds:    nil,
			expected: nil,
		},
		{
			name: "mask all sensitive fields",
			creds: &Credentials{
				Token:     "bearer-token-123456789",
				APIKey:    "api-key-abcdef123456",
				Username:  "testuser",
				Password:  "supersecret",
				ExpiresAt: time.Now().Add(1 * time.Hour),
				Metadata:  map[string]string{"source": "test"},
			},
			expected: &Credentials{
				Token:    "bea***789", // First 3 + last 3 with ***
				APIKey:   "api***456", // First 3 + last 3 with ***
				Username: "testuser",  // Username not masked
				Password: "[MASKED]",  // Password fully masked
				Metadata: map[string]string{"source": "test"},
			},
		},
		{
			name: "short token gets fully masked",
			creds: &Credentials{
				Token:  "short",
				APIKey: "tiny",
			},
			expected: &Credentials{
				Token:  "[MASKED]",
				APIKey: "[MASKED]",
			},
		},
		{
			name: "medium length strings",
			creds: &Credentials{
				Token:  "medium123", // 9 chars
				APIKey: "key567890", // 9 chars
			},
			expected: &Credentials{
				Token:  "me***23", // For 6-10 chars: first 2 + last 1
				APIKey: "ke***90", // For 6-10 chars: first 2 + last 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := masker.MaskCredentials(tt.creds)

			if tt.expected == nil {
				assert.Nil(t, result)
				return
			}

			assert.NotNil(t, result)
			assert.Equal(t, tt.expected.Token, result.Token)
			assert.Equal(t, tt.expected.APIKey, result.APIKey)
			assert.Equal(t, tt.expected.Username, result.Username)
			assert.Equal(t, tt.expected.Password, result.Password)

			// ExpiresAt should be preserved
			if !tt.creds.ExpiresAt.IsZero() {
				assert.Equal(t, tt.creds.ExpiresAt, result.ExpiresAt)
			}

			// Metadata should be copied
			if tt.creds.Metadata != nil {
				assert.Equal(t, tt.creds.Metadata, result.Metadata)
			}
		})
	}
}

func BenchmarkInMemoryCredentialStore_GetCredentials(b *testing.B) {
	store := NewInMemoryCredentialStore("benchmark-key")

	testCreds := &Credentials{
		Token:  "benchmark-token-123456789",
		APIKey: "benchmark-api-key-abcdef",
	}
	store.SetCredentials("benchmark", testCreds)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.GetCredentials("benchmark")
		if err != nil {
			b.Fatalf("GetCredentials failed: %v", err)
		}
	}
}

func BenchmarkFileCredentialStore_Encryption(b *testing.B) {
	tempDir := b.TempDir()
	store := NewFileCredentialStore(filepath.Join(tempDir, "bench.enc"), "benchmark-key")

	testData := []byte("benchmark test data for encryption performance testing")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		encrypted, err := store.encrypt(testData)
		if err != nil {
			b.Fatalf("Encryption failed: %v", err)
		}

		_, err = store.decrypt(encrypted)
		if err != nil {
			b.Fatalf("Decryption failed: %v", err)
		}
	}
}
