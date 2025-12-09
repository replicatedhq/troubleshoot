package redact

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewMultiLineRedactor(t *testing.T) {
	tests := []struct {
		name        string
		selector    LineRedactor
		scan        string
		redactor    string
		inputString string
		wantString  string
	}{
		{
			name: "Redact multiline with AWS secret access key",
			selector: LineRedactor{
				regex: `(?i)"name": *"[^\"]*SECRET_?ACCESS_?KEY[^\"]*"`,
			},
			redactor: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			inputString: `"name": "secret_access_key"
"value": "dfeadsfsdfe"`,
			wantString: `"name": "secret_access_key"
"value": "***HIDDEN***"`, // No trailing newline in input, so none in output
		},
		{
			name: "Redact multiline with AWS secret id",
			selector: LineRedactor{
				regex: `(?i)"name": *"[^\"]*ACCESS_?KEY_?ID[^\"]*"`,
			},
			redactor: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			inputString: `"name": "ACCESS_KEY_ID"
"value": "dfeadsfsdfe"`,
			wantString: `"name": "ACCESS_KEY_ID"
"value": "***HIDDEN***"`, // No trailing newline in input, so none in output
		},
		{
			name: "Redact multiline with OSD",
			selector: LineRedactor{
				regex: `(?i)"entity": *"(osd|client|mgr)\..*[^\"]*"`,
			},
			redactor: `(?i)("key": *")(?P<mask>.{38}==[^\"]*)(")`,
			inputString: `"entity": "osd.1abcdef"
"key": "Gjt8s0WkfPtxZUo7gI8a0awbQGHgzuprdaedfb=="`,
			wantString: `"entity": "osd.1abcdef"
"key": "***HIDDEN***"`, // No trailing newline in input, so none in output
		},
		{
			name: "Redact multiline with AWS secret access key and scan regex",
			selector: LineRedactor{
				regex: `(?i)"name": *"[^\"]*SECRET_?ACCESS_?KEY[^\"]*"`,
				scan:  `secret_?access_?key\"`,
			},
			redactor: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			inputString: `"name": "secret_access_key"
"value": "dfeadsfsdfe"`,
			wantString: `"name": "secret_access_key"
"value": "***HIDDEN***"`, // No trailing newline in input, so none in output
		},
		{
			name: "Redact multiline with AWS secret id and scan regex",
			selector: LineRedactor{
				regex: `(?i)"name": *"[^\"]*ACCESS_?KEY_?ID[^\"]*"`,
				scan:  `access_?key_?id\"`,
			},
			redactor: `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			inputString: `"name": "ACCESS_KEY_ID"
"value": "dfeadsfsdfe"`,
			wantString: `"name": "ACCESS_KEY_ID"
"value": "***HIDDEN***"`, // No trailing newline in input, so none in output
		},
		{
			name: "Redact multiline with OSD and scan regex",
			selector: LineRedactor{
				regex: `(?i)"entity": *"(osd|client|mgr)\..*[^\"]*"`,
				scan:  `(osd|client|mgr)`,
			},
			redactor: `(?i)("key": *")(?P<mask>.{38}==[^\"]*)(")`,
			inputString: `"entity": "osd.1abcdef"
"key": "Gjt8s0WkfPtxZUo7gI8a0awbQGHgzuprdaedfb=="`,
			wantString: `"entity": "osd.1abcdef"
"key": "***HIDDEN***"`, // No trailing newline in input, so none in output
		},
		{
			name: "Multiple newlines with no match",
			selector: LineRedactor{
				regex: `(?i)"name": *"[^\"]*SECRET_?ACCESS_?KEY[^\"]*"`,
				scan:  `secret_?access_?key`,
			},
			redactor:    `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			inputString: "no match\n\n no match \n\n",
			wantString:  "no match\n\n no match \n\n", // Input has trailing newline, should be preserved
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			reRunner, err := NewMultiLineRedactor(tt.selector, tt.redactor, MASK_TEXT, "testfile", tt.name, true)
			req.NoError(err)
			outReader := reRunner.Redact(bytes.NewReader([]byte(tt.inputString)), "")

			gotBytes, err := io.ReadAll(outReader)
			req.NoError(err)
			req.Equal(tt.wantString, string(gotBytes))
			GetRedactionList()
			ResetRedactionList()
		})
	}
}

func Test_writeBytes(t *testing.T) {
	tests := []struct {
		name       string
		inputBytes [][]byte
		want       string
	}{
		{
			name:       "No newline",
			inputBytes: [][]byte{[]byte("hello"), []byte("world")},
			want:       "helloworld",
		},
		{
			name:       "With newline",
			inputBytes: [][]byte{[]byte("hello"), NEW_LINE, []byte("world"), NEW_LINE},
			want:       "hello\nworld\n",
		},
		{
			name:       "Empty line",
			inputBytes: [][]byte{NEW_LINE},
			want:       "\n",
		},
		{
			name: "Nothing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var w strings.Builder
			err := writeBytes(&w, tt.inputBytes...)
			require.NoError(t, err)

			assert.Equal(t, tt.want, w.String())
		})
	}
}

// Test 3.16: Binary file (no newlines) → unchanged
func TestMultiLineRedactor_BinaryFile(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()
	
	// Binary content with no newlines - the bug that caused 2 extra bytes
	binaryData := []byte{0x01, 0x02, 0x03, 0x04, 0x00, 0xFF, 0xFE, 0xAB, 0xCD}

	redactor, err := NewMultiLineRedactor(
		LineRedactor{regex: `"name":`},
		`"value":`,
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader(binaryData), "test.bin")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, binaryData, result, "Binary file should be unchanged (no extra newlines)")
}

// Test 3.17: Single line with \n → unchanged
func TestMultiLineRedactor_SingleLineWithNewline(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()
	
	input := "single line\n"

	redactor, err := NewMultiLineRedactor(
		LineRedactor{regex: `"name":`},
		`"value":`,
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "single line\n", string(result))
}

// Test 3.18: Single line without \n → unchanged
func TestMultiLineRedactor_SingleLineWithoutNewline(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()
	
	input := "single line"

	redactor, err := NewMultiLineRedactor(
		LineRedactor{regex: `"name":`},
		`"value":`,
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "single line", string(result), "No newline should be added")
}

// Test 3.19: Empty file → unchanged
func TestMultiLineRedactor_EmptyFile(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()
	
	input := ""

	redactor, err := NewMultiLineRedactor(
		LineRedactor{regex: `"name":`},
		`"value":`,
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "", string(result), "Empty file should remain empty")
}

// Test 3.20: Two lines, matches selector → line2 redacted
func TestMultiLineRedactor_TwoLinesMatch(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()
	
	input := `"name": "PASSWORD"
"value": "secret123"`

	redactor, err := NewMultiLineRedactor(
		LineRedactor{regex: `(?i)"name": *"PASSWORD"`},
		`(?i)("value": *")(?P<mask>[^"]*)(")`,
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	expected := `"name": "PASSWORD"
"value": "***HIDDEN***"`
	require.Equal(t, expected, string(result))
}

// Test 3.21: Two lines, no selector match → unchanged
func TestMultiLineRedactor_TwoLinesNoMatch(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()
	
	input := `"name": "USERNAME"
"value": "admin"`

	redactor, err := NewMultiLineRedactor(
		LineRedactor{regex: `(?i)"name": *"PASSWORD"`},
		`(?i)("value": *")(?P<mask>[^"]*)(")`,
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	expected := `"name": "USERNAME"
"value": "admin"`
	require.Equal(t, expected, string(result))
}

// Test 3.22: Multiple line pairs → correct redactions
func TestMultiLineRedactor_MultiplePairs(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()
	
	input := `"name": "PASSWORD"
"value": "secret1"
"name": "TOKEN"
"value": "secret2"
"name": "USERNAME"
"value": "admin"
`

	redactor, err := NewMultiLineRedactor(
		LineRedactor{regex: `(?i)"name": *"(PASSWORD|TOKEN)"`},
		`(?i)("value": *")(?P<mask>[^"]*)(")`,
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	expected := `"name": "PASSWORD"
"value": "***HIDDEN***"
"name": "TOKEN"
"value": "***HIDDEN***"
"name": "USERNAME"
"value": "admin"
`
	require.Equal(t, expected, string(result))
}

// Test 3.23: Three lines (pair + unpaired)
func TestMultiLineRedactor_ThreeLines(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()
	
	input := `"name": "PASSWORD"
"value": "secret"
unpaired line`

	redactor, err := NewMultiLineRedactor(
		LineRedactor{regex: `(?i)"name": *"PASSWORD"`},
		`(?i)("value": *")(?P<mask>[^"]*)(")`,
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	expected := `"name": "PASSWORD"
"value": "***HIDDEN***"
unpaired line`
	require.Equal(t, expected, string(result))
}

// Test 3.24: Large file with selector matches
func TestMultiLineRedactor_LargeFile(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()
	
	var input strings.Builder
	for i := 0; i < 1000; i++ {
		input.WriteString(`"name": "PASSWORD"` + "\n")
		input.WriteString(`"value": "secret"` + "\n")
	}

	redactor, err := NewMultiLineRedactor(
		LineRedactor{regex: `(?i)"name": *"PASSWORD"`},
		`(?i)("value": *")(?P<mask>[^"]*)(")`,
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(strings.NewReader(input.String()), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	
	// Verify all secrets were redacted
	require.NotContains(t, string(result), `"value": "secret"`)
	require.Contains(t, string(result), `"value": "***HIDDEN***"`)
}
