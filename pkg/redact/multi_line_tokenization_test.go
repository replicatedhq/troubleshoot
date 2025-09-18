package redact

import (
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestMultiLineRedactor_TokenizationIntegration(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList() // Clean up global redaction list

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tests := []struct {
		name          string
		input         string
		selectorRegex string
		redactorRegex string
		context       string
		expectToken   bool
	}{
		{
			name: "AWS secret key multiline redaction",
			input: `"name":"AWS_SECRET_ACCESS_KEY"
"value":"AKIAIOSFODNN7EXAMPLE"`,
			selectorRegex: `"name":"AWS_SECRET_ACCESS_KEY"`,
			redactorRegex: `"value":"(?P<mask>.*)"`,
			context:       "aws_secret_key",
			expectToken:   true,
		},
		{
			name: "Database password multiline redaction",
			input: `"name":"DATABASE_PASSWORD"
"value":"mysecretdbpassword"`,
			selectorRegex: `"name":"DATABASE_PASSWORD"`,
			redactorRegex: `"value":"(?P<mask>.*)"`,
			context:       "database_password",
			expectToken:   true,
		},
		{
			name: "API token multiline redaction",
			input: `"name":"API_TOKEN"
"value":"Bearer abc123def456ghi789"`,
			selectorRegex: `"name":"API_TOKEN"`,
			redactorRegex: `"value":"(?P<mask>.*)"`,
			context:       "api_token",
			expectToken:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tokenizer for each test
			GetGlobalTokenizer().Reset()

			redactor, err := NewMultiLineRedactor(LineRedactor{
				regex: tt.selectorRegex,
			}, tt.redactorRegex, MASK_TEXT, "test-file", tt.context, false)
			if err != nil {
				t.Fatalf("Failed to create redactor: %v", err)
			}

			input := strings.NewReader(tt.input)
			output := redactor.Redact(input, "test-file")

			result, err := io.ReadAll(output)
			if err != nil {
				t.Fatalf("Failed to read redacted output: %v", err)
			}

			resultStr := string(result)
			t.Logf("Input:\n%s", tt.input)
			t.Logf("Output:\n%s", resultStr)

			if tt.expectToken {
				// Should contain a token, not the original mask text
				if strings.Contains(resultStr, "***HIDDEN***") {
					t.Errorf("Expected tokenized output, but found original mask text")
				}

				// Should contain TOKEN_ prefix
				if !strings.Contains(resultStr, "***TOKEN_") {
					t.Errorf("Expected tokenized output to contain ***TOKEN_ prefix, got: %s", resultStr)
				}

				// Should not contain the original secret
				if strings.Contains(resultStr, "AKIAIOSFODNN7EXAMPLE") ||
					strings.Contains(resultStr, "mysecretdbpassword") ||
					strings.Contains(resultStr, "Bearer abc123def456ghi789") {
					t.Errorf("Result still contains original secret value: %s", resultStr)
				}
			}

			// Test determinism - same input should produce same token
			input2 := strings.NewReader(tt.input)
			output2 := redactor.Redact(input2, "test-file")
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

func TestMultiLineRedactor_TokenizationDisabled(t *testing.T) {
	// Ensure tokenization is disabled
	os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList() // Clean up global redaction list

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	input := `"name":"DATABASE_PASSWORD"
"value":"mysecretpassword"`

	redactor, err := NewMultiLineRedactor(LineRedactor{
		regex: `"name":"DATABASE_PASSWORD"`,
	}, `"value":"(?P<mask>.*)"`, MASK_TEXT, "test-file", "database_password", false)
	if err != nil {
		t.Fatalf("Failed to create redactor: %v", err)
	}

	inputReader := strings.NewReader(input)
	output := redactor.Redact(inputReader, "test-file")

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
}

func TestMultiLineRedactor_ContextClassification(t *testing.T) {
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
		redactorName   string
		expectedPrefix string
	}{
		{
			name: "password context classification",
			input: `"name":"USER_PASSWORD"
"value":"mypassword123"`,
			redactorName:   "password-redactor",
			expectedPrefix: "PASSWORD",
		},
		{
			name: "api key context classification",
			input: `"name":"API_KEY"
"value":"ak-1234567890"`,
			redactorName:   "api-key-redactor",
			expectedPrefix: "APIKEY",
		},
		{
			name: "database context classification",
			input: `"name":"DATABASE_URL"
"value":"postgres://user:pass@host/db"`,
			redactorName:   "database-redactor",
			expectedPrefix: "DATABASE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tokenizer for each test
			GetGlobalTokenizer().Reset()

			redactor, err := NewMultiLineRedactor(LineRedactor{
				regex: `"name":"[A-Z_]+"`,
			}, `"value":"(?P<mask>.*)"`, MASK_TEXT, "test-file", tt.redactorName, false)
			if err != nil {
				t.Fatalf("Failed to create redactor: %v", err)
			}

			input := strings.NewReader(tt.input)
			output := redactor.Redact(input, "test-file")

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
				t.Errorf("Expected token with prefix %s, got: %s", tt.expectedPrefix, resultStr)
			}
		})
	}
}
