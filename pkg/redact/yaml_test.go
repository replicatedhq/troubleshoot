package redact

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.undefinedlabs.com/scopeagent"
)

func TestNewYamlRedactor(t *testing.T) {
	tests := []struct {
		name        string
		path        []string
		inputString string
		wantString  string
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
		},
		{
			name:        "non-yaml file",
			path:        []string{""},
			inputString: `hello world, this is not valid yaml: {`,
			wantString:  `hello world, this is not valid yaml: {`,
		},
		{
			name:        "no matches",
			path:        []string{"abc"},
			inputString: `improperly-formatted: yaml`,
			wantString:  `improperly-formatted: yaml`,
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
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopetest := scopeagent.StartTest(t)
			defer scopetest.End()

			req := require.New(t)
			yamlRunner := YamlRedactor{maskPath: tt.path}

			outReader := yamlRunner.Redact(bytes.NewReader([]byte(tt.inputString)))
			gotBytes, err := ioutil.ReadAll(outReader)
			req.NoError(err)
			req.Equal(tt.wantString, string(gotBytes))
		})
	}
}
