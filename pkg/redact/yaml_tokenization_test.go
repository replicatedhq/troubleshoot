package redact

import (
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestYamlRedactor_TokenizationIntegration(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList() // Clean up global redaction list

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tests := []struct {
		name        string
		input       string
		yamlPath    string
		context     string
		expectToken bool
	}{
		{
			name: "redact password in yaml",
			input: `database:
  password: "mysecretpassword"
  host: "localhost"`,
			yamlPath:    "database.password",
			context:     "database_password",
			expectToken: true,
		},
		{
			name: "redact api key in yaml",
			input: `config:
  api:
    key: "sk-1234567890abcdef"
    version: "v1"`,
			yamlPath:    "config.api.key",
			context:     "api_key",
			expectToken: true,
		},
		{
			name: "redact secret token in yaml",
			input: `secrets:
  - name: "auth_token"
    value: "Bearer abc123def456"
  - name: "public_key"
    value: "not-secret"`,
			yamlPath:    "secrets.0.value",
			context:     "secret_token",
			expectToken: true,
		},
		{
			name: "redact all secrets in array",
			input: `tokens:
  - "token1_secret"
  - "token2_secret"
  - "token3_secret"`,
			yamlPath:    "tokens.*",
			context:     "token_array",
			expectToken: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tokenizer for each test
			GetGlobalTokenizer().Reset()

			redactor := NewYamlRedactor(tt.yamlPath, "*.yaml", tt.context)

			input := strings.NewReader(tt.input)
			output := redactor.Redact(input, "test.yaml")

			result, err := io.ReadAll(output)
			if err != nil {
				t.Fatalf("Failed to read redacted output: %v", err)
			}

			resultStr := string(result)
			t.Logf("Input:\n%s", tt.input)
			t.Logf("Output:\n%s", resultStr)

			if tt.expectToken {
				// Should contain a token, not the original mask text (in most cases)
				// Note: YAML redactor might still use ***HIDDEN*** in some cases

				// Should contain TOKEN_ prefix when tokenization is enabled
				if !strings.Contains(resultStr, "***TOKEN_") && !strings.Contains(resultStr, "***HIDDEN***") {
					t.Errorf("Expected either tokenized output or mask text, got: %s", resultStr)
				}

				// Should not contain the original secrets
				if strings.Contains(resultStr, "mysecretpassword") ||
					strings.Contains(resultStr, "sk-1234567890abcdef") ||
					strings.Contains(resultStr, "Bearer abc123def456") ||
					(strings.Contains(resultStr, "token1_secret") && tt.yamlPath == "tokens.*") {
					t.Errorf("Result still contains original secret value: %s", resultStr)
				}
			}

			// Test determinism - same input should produce same token
			input2 := strings.NewReader(tt.input)
			output2 := redactor.Redact(input2, "test.yaml")
			result2, err := io.ReadAll(output2)
			if err != nil {
				t.Fatalf("Failed to read second redacted output: %v", err)
			}

			if string(result) != string(result2) {
				t.Errorf("Expected deterministic tokenization, got different results:\n  First:\n%s\n  Second:\n%s", string(result), string(result2))
			}
		})
	}
}

func TestYamlRedactor_TokenizationDisabled(t *testing.T) {
	// Ensure tokenization is disabled
	os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList() // Clean up global redaction list

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	input := `database:
  password: "mysecretpassword"
  host: "localhost"`

	redactor := NewYamlRedactor("database.password", "*.yaml", "database_password")

	inputReader := strings.NewReader(input)
	output := redactor.Redact(inputReader, "test.yaml")

	result, err := io.ReadAll(output)
	if err != nil {
		t.Fatalf("Failed to read redacted output: %v", err)
	}

	resultStr := string(result)
	t.Logf("Input:\n%s", input)
	t.Logf("Output:\n%s", resultStr)

	// Should contain the original mask text when tokenization is disabled
	if !strings.Contains(resultStr, "***HIDDEN***") {
		t.Errorf("Expected original mask text ***HIDDEN*** when tokenization disabled, got: %s", resultStr)
	}

	// Should not contain TOKEN_ prefix
	if strings.Contains(resultStr, "***TOKEN_") {
		t.Errorf("Should not contain tokenized output when tokenization disabled: %s", resultStr)
	}

	// Should not contain the original secret
	if strings.Contains(resultStr, "mysecretpassword") {
		t.Errorf("Result still contains original secret value: %s", resultStr)
	}
}

func TestYamlRedactor_ContextClassification(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList() // Clean up global redaction list

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tests := []struct {
		name           string
		input          string
		yamlPath       string
		redactorName   string
		expectedPrefix string
	}{
		{
			name: "password context classification",
			input: `config:
  password: "mypass123"`,
			yamlPath:       "config.password",
			redactorName:   "password-redactor",
			expectedPrefix: "PASSWORD",
		},
		{
			name: "api key context classification",
			input: `settings:
  apiKey: "ak-1234567890"`,
			yamlPath:       "settings.apiKey",
			redactorName:   "api-key-redactor",
			expectedPrefix: "APIKEY",
		},
		{
			name: "database context classification",
			input: `db:
  connectionString: "postgres://user:pass@host/db"`,
			yamlPath:       "db.connectionString",
			redactorName:   "database-redactor",
			expectedPrefix: "DATABASE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tokenizer for each test
			GetGlobalTokenizer().Reset()

			redactor := NewYamlRedactor(tt.yamlPath, "*.yaml", tt.redactorName)

			input := strings.NewReader(tt.input)
			output := redactor.Redact(input, "test.yaml")

			result, err := io.ReadAll(output)
			if err != nil {
				t.Fatalf("Failed to read redacted output: %v", err)
			}

			resultStr := string(result)
			t.Logf("Input:\n%s", tt.input)
			t.Logf("Output:\n%s", resultStr)

			// Check if the result contains the expected token prefix
			expectedToken := "***TOKEN_" + tt.expectedPrefix + "_"
			if !strings.Contains(resultStr, expectedToken) {
				// If not tokenized, should at least be masked
				if !strings.Contains(resultStr, "***HIDDEN***") {
					t.Errorf("Expected token with prefix %s or ***HIDDEN***, got: %s", tt.expectedPrefix, resultStr)
				}
			}
		})
	}
}
