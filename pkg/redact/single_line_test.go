package redact

import (
	"bytes"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/require"
	"go.undefinedlabs.com/scopeagent"
)

func TestNewSingleLineRedactor(t *testing.T) {
	tests := []struct {
		name        string
		re          string
		inputString string
		wantString  string
	}{
		{
			name:        "copied from default redactors",
			re:          `(?i)(Pwd *= *)(?P<mask>[^\;]+)(;)`,
			inputString: `pwd = abcdef;`,
			wantString:  "pwd = ***HIDDEN***;\n",
		},
		{
			name:        "no leading matching group", // this is not the ideal behavior - why are we dropping ungrouped match components?
			re:          `(?i)Pwd *= *(?P<mask>[^\;]+)(;)`,
			inputString: `pwd = abcdef;`,
			wantString:  "***HIDDEN***;\n",
		},
		{
			name:        "multiple matching literals",
			re:          `(?i)(Pwd *= *)(?P<mask>[^\;]+)(;)`,
			inputString: `pwd = abcdef;abcdef`,
			wantString:  "pwd = ***HIDDEN***;abcdef\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopetest := scopeagent.StartTest(t)
			defer scopetest.End()

			req := require.New(t)
			reRunner, err := NewSingleLineRedactor(tt.re, MASK_TEXT, "testfile", tt.name)
			req.NoError(err)

			outReader := reRunner.Redact(bytes.NewReader([]byte(tt.inputString)))
			gotBytes, err := ioutil.ReadAll(outReader)
			req.NoError(err)
			req.Equal(tt.wantString, string(gotBytes))
		})
	}
}
