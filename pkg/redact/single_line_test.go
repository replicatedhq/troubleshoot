package redact

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewSingleLineRedactor(t *testing.T) {
	tests := []struct {
		name           string
		re             string
		scan           string
		inputString    string
		wantString     string
		wantRedactions RedactionList
	}{
		{
			name:        "copied from default redactors",
			re:          `(?i)(Pwd *= *)(?P<mask>[^\;]+)(;)`,
			inputString: `pwd = abcdef;`,
			wantString:  "pwd = ***HIDDEN***;", // No trailing newline in input, so none in output
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"copied from default redactors": []Redaction{
						{
							RedactorName:      "copied from default redactors",
							CharactersRemoved: -6,
							Line:              1,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": []Redaction{
						{
							RedactorName:      "copied from default redactors",
							CharactersRemoved: -6,
							Line:              1,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name:        "no leading matching group", // this is not the ideal behavior - why are we dropping ungrouped match components?
			re:          `(?i)Pwd *= *(?P<mask>[^\;]+)(;)`,
			inputString: `pwd = abcdef;`,
			wantString:  "***HIDDEN***;", // No trailing newline in input, so none in output
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"no leading matching group": []Redaction{
						{
							RedactorName:      "no leading matching group",
							CharactersRemoved: 0,
							Line:              1,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": []Redaction{
						{
							RedactorName:      "no leading matching group",
							CharactersRemoved: 0,
							Line:              1,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name:        "multiple matching literals",
			re:          `(?i)(Pwd *= *)(?P<mask>[^\;]+)(;)`,
			inputString: `pwd = abcdef;abcdef`,
			wantString:  "pwd = ***HIDDEN***;abcdef", // No trailing newline in input, so none in output
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"multiple matching literals": []Redaction{
						{
							RedactorName:      "multiple matching literals",
							CharactersRemoved: -6,
							Line:              1,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": []Redaction{
						{
							RedactorName:      "multiple matching literals",
							CharactersRemoved: -6,
							Line:              1,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name:        "Redact values for environment variables that look like AWS Secret Access Keys",
			re:          `(?i)("name":"[^\"]*SECRET_?ACCESS_?KEY","value":")(?P<mask>[^\"]*)(")`,
			inputString: `{"name":"SECRET_ACCESS_KEY","value":"123"}`,
			wantString:  `{"name":"SECRET_ACCESS_KEY","value":"***HIDDEN***"}`, // No trailing newline in input, so none in output
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"Redact values for environment variables that look like AWS Secret Access Keys": []Redaction{
						{
							RedactorName:      "Redact values for environment variables that look like AWS Secret Access Keys",
							CharactersRemoved: -9,
							Line:              1,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": []Redaction{
						{
							RedactorName:      "Redact values for environment variables that look like AWS Secret Access Keys",
							CharactersRemoved: -9,
							Line:              1,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name:        "Redact connection strings with username and password",
			re:          `(?i)(https?|ftp)(:\/\/)(?P<mask>[^:\"\/]+){1}(:)(?P<mask>[^@\"\/]+){1}(?P<host>@[^:\/\s\"]+){1}(?P<port>:[\d]+)?`,
			inputString: `http://user:password@host:8888`,
			wantString:  "http://***HIDDEN***:***HIDDEN***@host:8888", // No trailing newline in input, so none in output
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"Redact connection strings with username and password": []Redaction{
						{
							RedactorName:      "Redact connection strings with username and password",
							CharactersRemoved: -12,
							Line:              1,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": []Redaction{
						{
							RedactorName:      "Redact connection strings with username and password",
							CharactersRemoved: -12,
							Line:              1,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name:        "Redact values for environment variables that look like AWS Secret Access Keys With Scan",
			re:          `(?i)("name":"[^\"]*SECRET_?ACCESS_?KEY","value":")(?P<mask>[^\"]*)(")`,
			scan:        `secret_?access_?key`,
			inputString: `{"name":"SECRET_ACCESS_KEY","value":"123"}`,
			wantString:  `{"name":"SECRET_ACCESS_KEY","value":"***HIDDEN***"}`, // No trailing newline in input, so none in output
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"Redact values for environment variables that look like AWS Secret Access Keys With Scan": {
						{
							RedactorName:      "Redact values for environment variables that look like AWS Secret Access Keys With Scan",
							CharactersRemoved: -9,
							Line:              1,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": {
						{
							RedactorName:      "Redact values for environment variables that look like AWS Secret Access Keys With Scan",
							CharactersRemoved: -9,
							Line:              1,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name:        "Redact values for environment variables that look like Access Keys ID With Scan",
			re:          `(?i)("name":"[^\"]*ACCESS_?KEY_?ID","value":")(?P<mask>[^\"]*)(")`,
			scan:        `access_?key_?id`,
			inputString: `{"name":"ACCESS_KEY_ID","value":"123"}`,
			wantString:  `{"name":"ACCESS_KEY_ID","value":"***HIDDEN***"}`, // No trailing newline in input, so none in output
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"Redact values for environment variables that look like Access Keys ID With Scan": {
						{
							RedactorName:      "Redact values for environment variables that look like Access Keys ID With Scan",
							CharactersRemoved: -9,
							Line:              1,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": {
						{
							RedactorName:      "Redact values for environment variables that look like Access Keys ID With Scan",
							CharactersRemoved: -9,
							Line:              1,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name:        "Redact values for environment variables that look like Owner Account With Scan",
			re:          `(?i)("name":"[^\"]*OWNER_?ACCOUNT","value":")(?P<mask>[^\"]*)(")`,
			scan:        `owner_?account`,
			inputString: `{"name":"OWNER_ACCOUNT","value":"123"}`,
			wantString:  `{"name":"OWNER_ACCOUNT","value":"***HIDDEN***"}`, // No trailing newline in input, so none in output
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"Redact values for environment variables that look like Owner Account With Scan": {
						{
							RedactorName:      "Redact values for environment variables that look like Owner Account With Scan",
							CharactersRemoved: -9,
							Line:              1,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": {
						{
							RedactorName:      "Redact values for environment variables that look like Owner Account With Scan",
							CharactersRemoved: -9,
							Line:              1,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name:        "Redact 'Data Source' values With Scan",
			re:          `(?i)(Data Source *= *)(?P<mask>[^\;]+)(;)`,
			scan:        `data source`,
			inputString: `Data Source = abcdef;`,
			wantString:  `Data Source = ***HIDDEN***;`, // No trailing newline in input, so none in output
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"Redact 'Data Source' values With Scan": {
						{
							RedactorName:      "Redact 'Data Source' values With Scan",
							CharactersRemoved: -6,
							Line:              1,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": {
						{
							RedactorName:      "Redact 'Data Source' values With Scan",
							CharactersRemoved: -6,
							Line:              1,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name:        "Redact connection strings With Scan",
			re:          `(?i)(https?|ftp)(:\/\/)(?P<mask>[^:\"\/]+){1}(:)(?P<mask>[^@\"\/]+){1}(?P<host>@[^:\/\s\"]+){1}(?P<port>:[\d]+)?`,
			scan:        `https?|ftp`,
			inputString: `http://user:password@host:8888;`,
			wantString:  `http://***HIDDEN***:***HIDDEN***@host:8888;`, // No trailing newline in input, so none in output
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"Redact connection strings With Scan": {
						{
							RedactorName:      "Redact connection strings With Scan",
							CharactersRemoved: -12,
							Line:              1,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": {
						{
							RedactorName:      "Redact connection strings With Scan",
							CharactersRemoved: -12,
							Line:              1,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name:        "Multiple newlines with no match",
			re:          `abcd`,
			inputString: "no match\n\n no match \n\n",
			wantString:  "no match\n\n no match \n\n",
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{},
				ByFile:     map[string][]Redaction{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ResetRedactionList()
			reRunner, err := NewSingleLineRedactor(LineRedactor{
				regex: tt.re,
				scan:  tt.scan,
			}, MASK_TEXT, "testfile", tt.name, false)
			req.NoError(err)

			outReader := reRunner.Redact(bytes.NewReader([]byte(tt.inputString)), "")
			gotBytes, err := io.ReadAll(outReader)
			req.NoError(err)
			req.Equal(tt.wantString, string(gotBytes))

			actualRedactions := GetRedactionList()
			ResetRedactionList()
			req.Equal(tt.wantRedactions, actualRedactions)
		})
	}
}

// Test 2.15: Binary file (no newlines) → unchanged
func TestSingleLineRedactor_BinaryFile(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	// Binary content with no newlines
	binaryData := []byte{0x01, 0x02, 0x03, 0x04, 0x00, 0xFF, 0xFE, 0xAB, 0xCD}

	redactor, err := NewSingleLineRedactor(
		LineRedactor{regex: "password"}, // Pattern that won't match binary
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader(binaryData), "test.bin")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, binaryData, result, "Binary file should be unchanged")
}

// Test 2.16: Text file with \n → preserved
func TestSingleLineRedactor_TextFileWithNewline(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "hello world\n"

	redactor, err := NewSingleLineRedactor(
		LineRedactor{regex: "xyz"}, // No match
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(result), "Trailing newline should be preserved")
}

// Test 2.17: Text file without \n → preserved
func TestSingleLineRedactor_TextFileWithoutNewline(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "hello world"

	redactor, err := NewSingleLineRedactor(
		LineRedactor{regex: "xyz"}, // No match
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "hello world", string(result), "No newline should be added")
}

// Test 2.18: Empty file → unchanged
func TestSingleLineRedactor_EmptyFile(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := ""

	redactor, err := NewSingleLineRedactor(
		LineRedactor{regex: "password"},
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "", string(result), "Empty file should remain empty")
}

// Test 2.19: Single line with secret → redacted correctly
func TestSingleLineRedactor_SingleLineWithSecret(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "password=secret123"

	redactor, err := NewSingleLineRedactor(
		LineRedactor{regex: `(?i)(password=)(?P<mask>.*)`},
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "password=***HIDDEN***", string(result))
}

// Test 2.20: Multiple lines with secrets → all redacted
func TestSingleLineRedactor_MultipleLinesWithSecrets(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "password=secret1\npassword=secret2\npassword=secret3\n"

	redactor, err := NewSingleLineRedactor(
		LineRedactor{regex: `(?i)(password=)(?P<mask>.*)`},
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	expected := "password=***HIDDEN***\npassword=***HIDDEN***\npassword=***HIDDEN***\n"
	require.Equal(t, expected, string(result))
}

// Test 2.21: Scan pattern filters correctly
func TestSingleLineRedactor_ScanPatternFilters(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "password=secret\nusername=admin\n"

	redactor, err := NewSingleLineRedactor(
		LineRedactor{
			regex: `(?i)(password=)(?P<mask>.*)`,
			scan:  `password`, // Only process lines containing "password"
		},
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	expected := "password=***HIDDEN***\nusername=admin\n"
	require.Equal(t, expected, string(result))
}

// Test 2.22: File with only one newline \n → one newline out
func TestSingleLineRedactor_OnlyNewline(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	input := "\n"

	redactor, err := NewSingleLineRedactor(
		LineRedactor{regex: "password"},
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader([]byte(input)), "test.txt")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, "\n", string(result))
}

// Test 2.23: Mixed binary/text content
func TestSingleLineRedactor_MixedContent(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	// Binary data with embedded newline
	input := []byte{0x01, 0x02, '\n', 0x03, 0x04}

	redactor, err := NewSingleLineRedactor(
		LineRedactor{regex: "password"},
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader(input), "test.bin")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, input, result, "Mixed content should be preserved")
}

// Test 2.24: Large binary file (1MB, no newlines) → preserved
func TestSingleLineRedactor_LargeBinaryFile(t *testing.T) {
	ResetRedactionList()
	defer ResetRedactionList()

	// Create 1MB of binary data
	largeData := make([]byte, 1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	redactor, err := NewSingleLineRedactor(
		LineRedactor{regex: "password"},
		MASK_TEXT, "testfile", "test", false,
	)
	require.NoError(t, err)

	out := redactor.Redact(bytes.NewReader(largeData), "large.bin")
	result, err := io.ReadAll(out)

	require.NoError(t, err)
	require.Equal(t, largeData, result, "Large binary file should be unchanged")
}
