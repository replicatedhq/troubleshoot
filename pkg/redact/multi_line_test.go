package redact

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewMultiLineRedactor(t *testing.T) {
	// Ensure tokenization is disabled for backward compatibility tests
	os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	ResetGlobalTokenizer()
	defer ResetRedactionList() // Clean up global redaction list
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
"value": "***HIDDEN***"
`,
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
"value": "***HIDDEN***"
`,
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
"key": "***HIDDEN***"
`,
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
"value": "***HIDDEN***"
`,
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
"value": "***HIDDEN***"
`,
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
"key": "***HIDDEN***"
`,
		},
		{
			name: "Multiple newlines with no match",
			selector: LineRedactor{
				regex: `(?i)"name": *"[^\"]*SECRET_?ACCESS_?KEY[^\"]*"`,
				scan:  `secret_?access_?key`,
			},
			redactor:    `(?i)("value": *")(?P<mask>.*[^\"]*)(")`,
			inputString: "no match\n\n no match \n\n",
			wantString:  "no match\n\n no match \n\n",
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
