package redact

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewSingleLineRedactor(t *testing.T) {
	tests := []struct {
		name           string
		re             string
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ResetRedactionList()
			reRunner, err := NewSingleLineRedactor(lineRedactor{
				regex: tt.re,
			}, MASK_TEXT, "testfile", tt.name, false)
			req.NoError(err)

			outReader := reRunner.Redact(bytes.NewReader([]byte(tt.inputString)), "")
			gotBytes, err := ioutil.ReadAll(outReader)
			req.NoError(err)
			req.Equal(tt.wantString, string(gotBytes))

			actualRedactions := GetRedactionList()
			ResetRedactionList()
			req.Equal(tt.wantRedactions, actualRedactions)
		})
	}
}
