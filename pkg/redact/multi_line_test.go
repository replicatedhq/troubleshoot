package redact

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_NewMultiLineRedactorr(t *testing.T) {
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			reRunner, err := NewMultiLineRedactor(tt.selector, tt.redactor, MASK_TEXT, "testfile", tt.name, true)
			req.NoError(err)
			outReader := reRunner.Redact(bytes.NewReader([]byte(tt.inputString)), "")

			gotBytes, err := ioutil.ReadAll(outReader)
			req.NoError(err)
			req.Equal(tt.wantString, string(gotBytes))
			ResetRedactionList()
		})
	}
}
