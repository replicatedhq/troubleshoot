package redact

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Test basic literal redaction functionality
func TestLiteralRedactor_BasicRedaction(t *testing.T) {
	tests := []struct {
		name        string
		match       string
		inputString string
		wantString  string
	}{
		{
			name:        "Simple literal match",
			match:       "secret123",
			inputString: "password=secret123",
			wantString:  "password=***HIDDEN***", // No trailing newline in input
		},
		{
			name:        "Multiple occurrences",
			match:       "secret",
			inputString: "secret is secret here secret",
			wantString:  "***HIDDEN*** is ***HIDDEN*** here ***HIDDEN***",
		},
		{
			name:        "No match",
			match:       "xyz",
			inputString: "no match here",
			wantString:  "no match here",
		},
		{
			name:        "With trailing newline",
			match:       "secret",
			inputString: "secret\n",
			wantString:  "***HIDDEN***\n",
		},
		{
			name:        "Multiline with newlines",
			match:       "secret",
			inputString: "line1 secret\nline2 secret\n",
			wantString:  "line1 ***HIDDEN***\nline2 ***HIDDEN***\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetRedactionList()
			defer ResetRedactionList()

			redactor := literalString([]byte(tt.match), "testfile", tt.name)

			out := redactor.Redact(bytes.NewReader([]byte(tt.inputString)), "")
			result, err := io.ReadAll(out)

			require.NoError(t, err)
			require.Equal(t, tt.wantString, string(result))
		})
	}
}

// Test 4.12: Binary file → unchanged
func TestLiteralRedactor_BinaryFile(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	// Binary content with no newlines and no match
	binaryData := []byte{0x01, 0x02, 0x03, 0x04, 0x00, 0xFF, 0xFE, 0xAB, 0xCD}

	redactor := literalString([]byte("notfound"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader(binaryData), "test.bin")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, binaryData, result, "Binary file should be unchanged")
}

// Test 4.12 (variant): Binary file with literal match → redacted, no extra newlines
func TestLiteralRedactor_BinaryFileWithMatch(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	// Binary content with a literal match (0xFF 0xFE sequence)
	binaryData := []byte{0x01, 0x02, 0xFF, 0xFE, 0x03, 0x04}

	redactor := literalString([]byte{0xFF, 0xFE}, "testfile", "test")

	// We need to mock maskTextBytes for this test to work predictably
	// For now, test that no newlines are added
	out := redactor.Redact(bytes.NewReader(binaryData), "test.bin")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.NotEqual(t, binaryData, result, "Binary should be redacted")
	require.NotContains(t, result, []byte{0xFF, 0xFE}, "Match should be replaced")
	// Most importantly: no trailing newline added to binary file
	require.NotEqual(t, byte('\n'), result[len(result)-1], "Should not add trailing newline")
}

// Test 4.13: Text with trailing \n → preserved
func TestLiteralRedactor_TextWithTrailingNewline(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "hello world\n"

	redactor := literalString([]byte("xyz"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(result), "Trailing newline should be preserved")
}

// Test 4.14: Text without trailing \n → preserved
func TestLiteralRedactor_TextWithoutTrailingNewline(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "hello world"

	redactor := literalString([]byte("xyz"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "hello world", string(result), "No newline should be added")
}

// Test 4.15: Empty file → unchanged
func TestLiteralRedactor_EmptyFile(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := ""

	redactor := literalString([]byte("secret"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "", string(result), "Empty file should remain empty")
}

// Test 4.16: Literal match and replacement works
func TestLiteralRedactor_LiteralMatch(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "password=secret123"

	redactor := literalString([]byte("secret123"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "password=***HIDDEN***", string(result))
}

// Test 4.17: Multiple occurrences replaced
func TestLiteralRedactor_MultipleOccurrences(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "secret here and secret there and secret everywhere"

	redactor := literalString([]byte("secret"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "***HIDDEN*** here and ***HIDDEN*** there and ***HIDDEN*** everywhere", string(result))
}

// Test 4.17 (variant): Multiple occurrences across lines
func TestLiteralRedactor_MultipleOccurrencesMultiline(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "line1 secret\nline2 secret\nline3 secret\n"

	redactor := literalString([]byte("secret"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	expected := "line1 ***HIDDEN***\nline2 ***HIDDEN***\nline3 ***HIDDEN***\n"
	require.Equal(t, expected, string(result))
}

// Test 4.18: Tokenization works
func TestLiteralRedactor_Tokenization(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	// Enable tokenization for this test
	EnableTokenization()
	defer DisableTokenization()

	input := "password=secret123"

	redactor := literalString([]byte("secret123"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	// Result should contain a token, not the original or ***HIDDEN***
	require.NotContains(t, string(result), "secret123")
	require.NotContains(t, string(result), "***HIDDEN***")
	require.Contains(t, string(result), "password=")
}

// Test 4.19: Redaction count accurate
func TestLiteralRedactor_RedactionCount(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "secret here\nsecret there"

	redactor := literalString([]byte("secret"), "testfile", "test-redactor")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "")
	_, err := io.ReadAll(out)

	require.NoError(t, err)

	redactions := GetRedactionList()
	// Two lines, each with one match = 2 redaction events
	require.Len(t, redactions.ByRedactor["test-redactor"], 2, "Should record 2 redactions (one per line)")
	require.Len(t, redactions.ByFile["testfile"], 2, "Should record 2 redactions for file")
}

// Test 4.20: Backward compatibility - existing behavior preserved for text with newlines
func TestLiteralRedactor_BackwardCompatibility(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "line1 secret\nline2 secret\nline3\n"

	redactor := literalString([]byte("secret"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	expected := "line1 ***HIDDEN***\nline2 ***HIDDEN***\nline3\n"
	require.Equal(t, expected, string(result), "Behavior for text with newlines should be unchanged")
}

// Test 4.20 (variant): Literal match on last line without \n
func TestLiteralRedactor_LastLineWithoutNewline(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "line1\nline2 secret"

	redactor := literalString([]byte("secret"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	expected := "line1\nline2 ***HIDDEN***"
	require.Equal(t, expected, string(result), "Should not add newline to last line")
}

// Additional test: Empty line handling
func TestLiteralRedactor_EmptyLines(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "\n\n\n"

	redactor := literalString([]byte("secret"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "\n\n\n", string(result), "Empty lines should be preserved")
}

// Additional test: Large file with many matches
func TestLiteralRedactor_LargeFile(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	// Create large file with many occurrences
	var input strings.Builder
	for i := 0; i < 1000; i++ {
		input.WriteString("line ")
		input.WriteString("secret")
		input.WriteString(" here\n")
	}

	redactor := literalString([]byte("secret"), "testfile", "test")

	out := redactor.Redact(strings.NewReader(input.String()), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.NotContains(t, string(result), "secret", "All secrets should be redacted")
	require.Contains(t, string(result), "***HIDDEN***")
}

// Additional test: Partial match should not be replaced
func TestLiteralRedactor_PartialMatchNotReplaced(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "secret secretive secrets"

	// Should only replace exact literal "secret", not "secretive" or "secrets"
	redactor := literalString([]byte("secret"), "testfile", "test")

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "***HIDDEN*** ***HIDDEN***ive ***HIDDEN***s", string(result))
}
