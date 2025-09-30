package redact

import (
	"strings"
	"testing"
)

func TestTokenizer_TokenizeValue(t *testing.T) {
	// Create tokenizer with test config
	config := TokenizerConfig{
		Enabled:       true,
		Salt:          []byte("test-salt-for-deterministic-results"),
		DefaultPrefix: TokenPrefixGeneric,
		TokenFormat:   "***TOKEN_%s_%s***",
		HashLength:    6,
	}
	tokenizer := NewTokenizer(config)

	tests := []struct {
		name           string
		value          string
		context        string
		expectedPrefix string
	}{
		{
			name:           "password detection",
			value:          "mysecretpassword",
			context:        "password",
			expectedPrefix: "PASSWORD",
		},
		{
			name:           "API key detection",
			value:          "sk-1234567890abcdef",
			context:        "api_key",
			expectedPrefix: "APIKEY",
		},
		{
			name:           "database detection",
			value:          "postgres://user:pass@host:5432/db",
			context:        "database_url",
			expectedPrefix: "DATABASE",
		},
		{
			name:           "email detection",
			value:          "user@example.com",
			context:        "email",
			expectedPrefix: "EMAIL",
		},
		{
			name:           "IP address detection",
			value:          "192.168.1.100",
			context:        "server_ip",
			expectedPrefix: "IP",
		},
		{
			name:           "generic secret",
			value:          "some-random-value",
			context:        "unknown_field",
			expectedPrefix: "GENERIC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := tokenizer.TokenizeValue(tt.value, tt.context)

			// Validate token format
			if !tokenizer.ValidateToken(token) {
				t.Errorf("Generated token %q is not valid", token)
			}

			// Check if token contains expected prefix
			if !strings.Contains(token, tt.expectedPrefix) {
				t.Errorf("Expected token to contain prefix %q, got %q", tt.expectedPrefix, token)
			}

			// Test determinism - same value should produce same token
			token2 := tokenizer.TokenizeValue(tt.value, tt.context)
			if token != token2 {
				t.Errorf("Expected deterministic token generation, got %q and %q", token, token2)
			}
		})
	}
}

func TestTokenizer_CollisionResolution(t *testing.T) {
	config := TokenizerConfig{
		Enabled:       true,
		Salt:          []byte("collision-test-salt"),
		DefaultPrefix: TokenPrefixGeneric,
		TokenFormat:   "***TOKEN_%s_%s***",
		HashLength:    2, // Short hash to force collisions
	}
	tokenizer := NewTokenizer(config)

	// Generate tokens for different values that might collide
	token1 := tokenizer.TokenizeValue("value1", "test")
	token2 := tokenizer.TokenizeValue("value2", "test")

	// Tokens should be different even with short hash
	if token1 == token2 {
		t.Errorf("Expected different tokens for different values, got %q for both", token1)
	}

	// Same value should produce same token
	token1_again := tokenizer.TokenizeValue("value1", "test")
	if token1 != token1_again {
		t.Errorf("Expected same token for same value, got %q and %q", token1, token1_again)
	}
}

func TestTokenizer_ValidateToken(t *testing.T) {
	tokenizer := NewTokenizer(TokenizerConfig{})

	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{
			name:     "valid token",
			token:    "***TOKEN_PASSWORD_A1B2C3***",
			expected: true,
		},
		{
			name:     "valid token with collision suffix",
			token:    "***TOKEN_APIKEY_D4E5F6_2***",
			expected: true,
		},
		{
			name:     "invalid format - missing stars",
			token:    "TOKEN_PASSWORD_A1B2C3",
			expected: false,
		},
		{
			name:     "invalid format - wrong prefix",
			token:    "***BADTOKEN_PASSWORD_A1B2C3***",
			expected: false,
		},
		{
			name:     "empty token",
			token:    "",
			expected: false,
		},
		{
			name:     "original mask text",
			token:    "***HIDDEN***",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizer.ValidateToken(tt.token)
			if result != tt.expected {
				t.Errorf("ValidateToken(%q) = %v, expected %v", tt.token, result, tt.expected)
			}
		})
	}
}

func TestTokenizer_DisabledBehavior(t *testing.T) {
	config := TokenizerConfig{
		Enabled: false, // Disabled
	}
	tokenizer := NewTokenizer(config)

	token := tokenizer.TokenizeValue("secret-password", "password")

	// Should return original mask text when disabled
	if token != MASK_TEXT {
		t.Errorf("Expected %q when tokenization disabled, got %q", MASK_TEXT, token)
	}
}

func TestTokenizer_EnvironmentToggle(t *testing.T) {
	// Test with explicit tokenization enabled
	EnableTokenization()
	defer DisableTokenization()

	globalTokenizer := GetGlobalTokenizer()
	if !globalTokenizer.IsEnabled() {
		t.Error("Expected tokenization to be enabled when explicitly enabled")
	}

	// Test tokenization works
	token := globalTokenizer.TokenizeValue("test-secret", "password")
	if token == MASK_TEXT {
		t.Error("Expected tokenized value, got original mask text")
	}
	if !globalTokenizer.ValidateToken(token) {
		t.Errorf("Generated token %q should be valid", token)
	}
}

func TestTokenizer_GetRedactionMap(t *testing.T) {
	config := TokenizerConfig{
		Enabled: true,
		Salt:    []byte("test-salt"),
	}
	tokenizer := NewTokenizer(config)

	// Generate some tokens
	tokenizer.TokenizeValue("password123", "password")
	tokenizer.TokenizeValue("api-key-456", "api_key")
	tokenizer.TokenizeValue("user@example.com", "email")

	redactionMap := tokenizer.GetRedactionMap("test-profile")

	// Validate redaction map
	if redactionMap.Profile != "test-profile" {
		t.Errorf("Expected profile 'test-profile', got %q", redactionMap.Profile)
	}

	if redactionMap.Stats.TotalSecrets != 3 {
		t.Errorf("Expected 3 total secrets, got %d", redactionMap.Stats.TotalSecrets)
	}

	if redactionMap.Stats.UniqueSecrets != 3 {
		t.Errorf("Expected 3 unique secrets, got %d", redactionMap.Stats.UniqueSecrets)
	}

	if redactionMap.Stats.TokensGenerated != 3 {
		t.Errorf("Expected 3 tokens generated, got %d", redactionMap.Stats.TokensGenerated)
	}

	if len(redactionMap.Tokens) != 3 {
		t.Errorf("Expected 3 tokens in map, got %d", len(redactionMap.Tokens))
	}

	// Verify reverse mapping works
	for token, original := range redactionMap.Tokens {
		if !tokenizer.ValidateToken(token) {
			t.Errorf("Token %q should be valid", token)
		}
		if original == "" {
			t.Error("Original value should not be empty")
		}
	}
}

func TestTokenizer_ClassifySecret(t *testing.T) {
	tokenizer := NewTokenizer(TokenizerConfig{})

	tests := []struct {
		name           string
		context        string
		value          string
		expectedPrefix TokenPrefix
	}{
		{
			name:           "password context",
			context:        "user_password",
			value:          "secret123",
			expectedPrefix: TokenPrefixPassword,
		},
		{
			name:           "API key context",
			context:        "api-key",
			value:          "ak_1234567890",
			expectedPrefix: TokenPrefixAPIKey,
		},
		{
			name:           "database context",
			context:        "db_connection_string",
			value:          "postgresql://localhost",
			expectedPrefix: TokenPrefixDatabase,
		},
		{
			name:           "email value detection",
			context:        "unknown",
			value:          "test@example.com",
			expectedPrefix: TokenPrefixEmail,
		},
		{
			name:           "IP value detection",
			context:        "unknown",
			value:          "10.0.0.1",
			expectedPrefix: TokenPrefixIP,
		},
		{
			name:           "generic fallback",
			context:        "random_field",
			value:          "random_value",
			expectedPrefix: TokenPrefixGeneric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tokenizer.classifySecret(tt.context, tt.value)
			if result != tt.expectedPrefix {
				t.Errorf("classifySecret(%q, %q) = %v, expected %v", tt.context, tt.value, result, tt.expectedPrefix)
			}
		})
	}
}

func TestTokenizer_Reset(t *testing.T) {
	config := TokenizerConfig{
		Enabled: true,
		Salt:    []byte("test-salt"),
	}
	tokenizer := NewTokenizer(config)

	// Generate some tokens
	tokenizer.TokenizeValue("secret1", "context1")
	tokenizer.TokenizeValue("secret2", "context2")

	if tokenizer.GetTokenCount() != 2 {
		t.Errorf("Expected 2 tokens before reset, got %d", tokenizer.GetTokenCount())
	}

	// Reset tokenizer
	tokenizer.Reset()

	if tokenizer.GetTokenCount() != 0 {
		t.Errorf("Expected 0 tokens after reset, got %d", tokenizer.GetTokenCount())
	}

	// Verify maps are cleared
	redactionMap := tokenizer.GetRedactionMap("test")
	if len(redactionMap.Tokens) != 0 {
		t.Errorf("Expected empty token map after reset, got %d tokens", len(redactionMap.Tokens))
	}
}
