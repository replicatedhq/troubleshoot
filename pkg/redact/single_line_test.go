package redact

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewSingleLineRedactor(t *testing.T) {
	// Ensure tokenization is disabled for backward compatibility tests
	os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	ResetGlobalTokenizer()
	defer ResetRedactionList() // Clean up global redaction list
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
			wantString:  "pwd = ***HIDDEN***;\n",
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
			wantString:  "***HIDDEN***;\n",
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
			wantString:  "pwd = ***HIDDEN***;abcdef\n",
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
			wantString: `{"name":"SECRET_ACCESS_KEY","value":"***HIDDEN***"}
`,
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
			wantString:  "http://***HIDDEN***:***HIDDEN***@host:8888\n",
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
			wantString: `{"name":"SECRET_ACCESS_KEY","value":"***HIDDEN***"}
`,
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
			wantString: `{"name":"ACCESS_KEY_ID","value":"***HIDDEN***"}
`,
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
			wantString: `{"name":"OWNER_ACCOUNT","value":"***HIDDEN***"}
`,
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
			wantString: `Data Source = ***HIDDEN***;
`,
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
			wantString: `http://***HIDDEN***:***HIDDEN***@host:8888;
`,
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
