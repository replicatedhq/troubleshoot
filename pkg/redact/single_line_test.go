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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			ResetRedactionList()
			reRunner, err := NewSingleLineRedactor(tt.re, MASK_TEXT, "testfile", tt.name, false)
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
