package redact

import (
	"io"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestLiteralRedactor_TokenizationIntegration(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList() // Clean up global redaction list

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	tests := []struct {
		name         string
		input        string
		literalValue string
		context      string
		expectToken  bool
	}{
		{
			name:         "redact literal password",
			input:        `password=mysecretpassword123 username=johndoe`,
			literalValue: "mysecretpassword123",
			context:      "password_literal",
			expectToken:  true,
		},
		{
			name:         "redact literal API key",
			input:        `API_KEY=sk-1234567890abcdef other=value`,
			literalValue: "sk-1234567890abcdef",
			context:      "api_key_literal",
			expectToken:  true,
		},
		{
			name: "redact literal token in multiline",
			input: `line1
Bearer abc123def456ghi789
line3`,
			literalValue: "Bearer abc123def456ghi789",
			context:      "bearer_token",
			expectToken:  true,
		},
		{
			name:         "redact literal secret in JSON",
			input:        `{"secret": "my-secret-value", "public": "not-secret"}`,
			literalValue: "my-secret-value",
			context:      "json_secret",
			expectToken:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tokenizer for each test
			GetGlobalTokenizer().Reset()

			redactor := literalString([]byte(tt.literalValue), "test-file", tt.context)

			input := strings.NewReader(tt.input)
			output := redactor.Redact(input, "test-file")

			result, err := io.ReadAll(output)
			if err != nil {
				t.Fatalf("Failed to read redacted output: %v", err)
			}

			resultStr := string(result)
			t.Logf("Input:  %s", tt.input)
			t.Logf("Literal: %s", tt.literalValue)
			t.Logf("Output: %s", resultStr)

			if tt.expectToken {
				// Should contain a token, not the original mask text
				if strings.Contains(resultStr, "***HIDDEN***") {
					t.Errorf("Expected tokenized output, but found original mask text")
				}

				// Should contain TOKEN_ prefix
				if !strings.Contains(resultStr, "***TOKEN_") {
					t.Errorf("Expected tokenized output to contain ***TOKEN_ prefix, got: %s", resultStr)
				}

				// Should not contain the original literal value
				if strings.Contains(resultStr, tt.literalValue) {
					t.Errorf("Result still contains original literal value '%s': %s", tt.literalValue, resultStr)
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
				t.Errorf("Expected deterministic tokenization, got different results:\n  First:  %s\n  Second: %s", string(result), string(result2))
			}
		})
	}
}

func TestLiteralRedactor_TokenizationDisabled(t *testing.T) {
	// Ensure tokenization is disabled
	os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList() // Clean up global redaction list

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	input := `password=mysecretpassword other=value`
	literalValue := "mysecretpassword"

	redactor := literalString([]byte(literalValue), "test-file", "password_literal")

	inputReader := strings.NewReader(input)
	output := redactor.Redact(inputReader, "test-file")

	result, err := io.ReadAll(output)
	if err != nil {
		t.Fatalf("Failed to read redacted output: %v", err)
	}

	resultStr := string(result)
	t.Logf("Input:  %s", input)
	t.Logf("Output: %s", resultStr)

	// Should contain the original mask text when tokenization is disabled
	if !strings.Contains(resultStr, "***HIDDEN***") {
		t.Errorf("Expected original mask text ***HIDDEN*** when tokenization disabled, got: %s", resultStr)
	}

	// Should not contain TOKEN_ prefix
	if strings.Contains(resultStr, "***TOKEN_") {
		t.Errorf("Should not contain tokenized output when tokenization disabled: %s", resultStr)
	}

	// Should not contain the original literal value
	if strings.Contains(resultStr, literalValue) {
		t.Errorf("Result still contains original literal value: %s", resultStr)
	}
}

func TestLiteralRedactor_ContextClassification(t *testing.T) {
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
		literalValue   string
		redactorName   string
		expectedPrefix string
	}{
		{
			name:           "password context classification",
			input:          `password=mypass123 other=value`,
			literalValue:   "mypass123",
			redactorName:   "password-redactor",
			expectedPrefix: "PASSWORD",
		},
		{
			name:           "api key context classification",
			input:          `api_key=ak-1234567890 other=value`,
			literalValue:   "ak-1234567890",
			redactorName:   "api-key-redactor",
			expectedPrefix: "APIKEY",
		},
		{
			name:           "database context classification",
			input:          `db_url=postgres://user:pass@host/db`,
			literalValue:   "postgres://user:pass@host/db",
			redactorName:   "database-redactor",
			expectedPrefix: "DATABASE",
		},
		{
			name:           "generic secret classification",
			input:          `secret=randomsecret123 other=value`,
			literalValue:   "randomsecret123",
			redactorName:   "secret-literal",
			expectedPrefix: "SECRET",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset tokenizer for each test
			GetGlobalTokenizer().Reset()

			redactor := literalString([]byte(tt.literalValue), "test-file", tt.redactorName)

			input := strings.NewReader(tt.input)
			output := redactor.Redact(input, "test-file")

			result, err := io.ReadAll(output)
			if err != nil {
				t.Fatalf("Failed to read redacted output: %v", err)
			}

			resultStr := string(result)
			t.Logf("Input:  %s", tt.input)
			t.Logf("Output: %s", resultStr)

			// Check if the result contains the expected token prefix
			expectedToken := "***TOKEN_" + tt.expectedPrefix + "_"
			if !strings.Contains(resultStr, expectedToken) {
				t.Errorf("Expected token with prefix %s, got: %s", tt.expectedPrefix, resultStr)
			}
		})
	}
}

func TestLiteralRedactor_MultipleOccurrences(t *testing.T) {
	// Enable tokenization
	os.Setenv("TROUBLESHOOT_TOKENIZATION", "true")
	defer os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	defer ResetRedactionList() // Clean up global redaction list

	// Reset the global tokenizer to ensure fresh state
	globalTokenizer = nil
	tokenizerOnce = sync.Once{}

	input := `password=secret123 confirm_password=secret123 other=value`
	literalValue := "secret123"

	// Reset tokenizer to ensure consistent results
	GetGlobalTokenizer().Reset()

	redactor := literalString([]byte(literalValue), "test-file", "password_literal")

	inputReader := strings.NewReader(input)
	output := redactor.Redact(inputReader, "test-file")

	result, err := io.ReadAll(output)
	if err != nil {
		t.Fatalf("Failed to read redacted output: %v", err)
	}

	resultStr := string(result)
	t.Logf("Input:  %s", input)
	t.Logf("Output: %s", resultStr)

	// Should contain TOKEN_ prefix
	if !strings.Contains(resultStr, "***TOKEN_") {
		t.Errorf("Expected tokenized output to contain ***TOKEN_ prefix, got: %s", resultStr)
	}

	// Should not contain the original literal value
	if strings.Contains(resultStr, literalValue) {
		t.Errorf("Result still contains original literal value: %s", resultStr)
	}

	// Count the number of token occurrences (should be 2)
	tokenCount := strings.Count(resultStr, "***TOKEN_")
	if tokenCount != 2 {
		t.Errorf("Expected 2 token occurrences, got %d in: %s", tokenCount, resultStr)
	}

	// Both occurrences should be the same token (deterministic)
	parts := strings.Split(resultStr, "***TOKEN_")
	if len(parts) >= 3 {
		// Extract the token part after the first occurrence
		firstToken := strings.Split(parts[1], "***")[0]
		secondToken := strings.Split(parts[2], "***")[0]

		if firstToken != secondToken {
			t.Errorf("Expected same token for multiple occurrences, got '%s' and '%s'", firstToken, secondToken)
		}
	}
}
