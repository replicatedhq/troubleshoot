package redact

import (
	"bytes"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewYamlRedactor(t *testing.T) {
	// Ensure tokenization is disabled for backward compatibility tests
	os.Unsetenv("TROUBLESHOOT_TOKENIZATION")
	ResetGlobalTokenizer()
	defer ResetRedactionList() // Clean up global redaction list
	tests := []struct {
		name           string
		path           []string
		inputString    string
		wantString     string
		wantRedactions RedactionList
	}{
		{
			name: "object paths",
			path: []string{"abc", "xyz"},
			inputString: `
abc:
  xyz:
  - a
  - b
xyz:
  hello: {}`,
			wantString: `abc:
  xyz: '***HIDDEN***'
xyz:
  hello: {}
`,
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"object paths": []Redaction{
						{
							RedactorName:      "object paths",
							CharactersRemoved: -3,
							Line:              0,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": []Redaction{
						{
							RedactorName:      "object paths",
							CharactersRemoved: -3,
							Line:              0,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name: "one index in array",
			path: []string{"abc", "xyz", "0"},
			inputString: `
abc:
  xyz:
  - a
  - b
xyz:
  hello: {}`,
			wantString: `abc:
  xyz:
  - '***HIDDEN***'
  - b
xyz:
  hello: {}
`,
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"one index in array": []Redaction{
						{
							RedactorName:      "one index in array",
							CharactersRemoved: -13,
							Line:              0,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": []Redaction{
						{
							RedactorName:      "one index in array",
							CharactersRemoved: -13,
							Line:              0,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name: "index after end of array",
			path: []string{"abc", "xyz", "10"},
			inputString: `
abc:
  xyz:
  - a
  - b
xyz:
  hello: {}`,
			wantString: `
abc:
  xyz:
  - a
  - b
xyz:
  hello: {}`,
			wantRedactions: RedactionList{ByRedactor: map[string][]Redaction{}, ByFile: map[string][]Redaction{}},
		},
		{
			name: "non-integer index",
			path: []string{"abc", "xyz", "non-int"},
			inputString: `
abc:
  xyz:
  - a
  - b
xyz:
  hello: {}`,
			wantString: `
abc:
  xyz:
  - a
  - b
xyz:
  hello: {}`,
			wantRedactions: RedactionList{ByRedactor: map[string][]Redaction{}, ByFile: map[string][]Redaction{}},
		},
		{
			name: "object paths, no matches",
			path: []string{"notexist", "xyz"},
			inputString: `
abc:
  xyz:
  - a
  - b
xyz:
  hello: {}`,
			wantString: `
abc:
  xyz:
  - a
  - b
xyz:
  hello: {}`,
			wantRedactions: RedactionList{ByRedactor: map[string][]Redaction{}, ByFile: map[string][]Redaction{}},
		},
		{
			name: "star index in array",
			path: []string{"abc", "xyz", "*"},
			inputString: `
abc:
  xyz:
  - a
  - b
xyz:
  hello: {}`,
			wantString: `abc:
  xyz:
  - '***HIDDEN***'
  - '***HIDDEN***'
xyz:
  hello: {}
`,
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"star index in array": []Redaction{
						{
							RedactorName:      "star index in array",
							CharactersRemoved: -26,
							Line:              0,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": []Redaction{
						{
							RedactorName:      "star index in array",
							CharactersRemoved: -26,
							Line:              0,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name: "objects within array index in array",
			path: []string{"abc", "xyz", "0", "a"},
			inputString: `
abc:
  xyz:
  - a: hello
  - b
xyz:
  hello: {}`,
			wantString: `abc:
  xyz:
  - a: '***HIDDEN***'
  - b
xyz:
  hello: {}
`,
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"objects within array index in array": []Redaction{
						{
							RedactorName:      "objects within array index in array",
							CharactersRemoved: -9,
							Line:              0,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": []Redaction{
						{
							RedactorName:      "objects within array index in array",
							CharactersRemoved: -9,
							Line:              0,
							File:              "testfile",
						},
					},
				},
			},
		},
		{
			name:           "non-yaml file",
			path:           []string{""},
			inputString:    `hello world, this is not valid yaml: {`,
			wantString:     `hello world, this is not valid yaml: {`,
			wantRedactions: RedactionList{ByRedactor: map[string][]Redaction{}, ByFile: map[string][]Redaction{}},
		},
		{
			name:           "no matches",
			path:           []string{"abc"},
			inputString:    `improperly-formatted: yaml`,
			wantString:     `improperly-formatted: yaml`,
			wantRedactions: RedactionList{ByRedactor: map[string][]Redaction{}, ByFile: map[string][]Redaction{}},
		},
		{
			name: "star index in map",
			path: []string{"abc", "xyz", "*"},
			inputString: `
abc:
  xyz:
    a: b
    c: d
    e: f
xyz:
  hello: {}`,
			wantString: `abc:
  xyz:
    a: '***HIDDEN***'
    c: '***HIDDEN***'
    e: '***HIDDEN***'
xyz:
  hello: {}
`,
			wantRedactions: RedactionList{
				ByRedactor: map[string][]Redaction{
					"star index in map": []Redaction{
						{
							RedactorName:      "star index in map",
							CharactersRemoved: -39,
							Line:              0,
							File:              "testfile",
						},
					},
				},
				ByFile: map[string][]Redaction{
					"testfile": []Redaction{
						{
							RedactorName:      "star index in map",
							CharactersRemoved: -39,
							Line:              0,
							File:              "testfile",
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ResetRedactionList() // Clean up between subtests
			req := require.New(t)
			yamlRunner := NewYamlRedactor(strings.Join(tt.path, "."), "testfile", tt.name)

			outReader := yamlRunner.Redact(bytes.NewReader([]byte(tt.inputString)), "testfile")
			gotBytes, err := ioutil.ReadAll(outReader)
			req.NoError(err)
			req.Equal(tt.wantString, string(gotBytes))

			actualRedactions := GetRedactionList()
			ResetRedactionList()
			req.Equal(tt.wantRedactions, actualRedactions)
		})
	}
}
