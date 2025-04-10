package redact

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLiteralRedactor(t *testing.T) {
	tests := []struct {
		name           string
		match         string
		inputString    string
		wantString     string
		wantRedactions RedactionList
	}{
		{
			name:        "Multiple newlines with no match",
			match:       "secret",
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

			redactor := literalString([]byte(tt.match), "testfile", tt.name)
			outReader := redactor.Redact(bytes.NewReader([]byte(tt.inputString)), "")
			
			gotBytes, err := io.ReadAll(outReader)
			req.NoError(err)
			req.Equal(tt.wantString, string(gotBytes))

			actualRedactions := GetRedactionList()
			ResetRedactionList()
			req.Equal(tt.wantRedactions, actualRedactions)
		})
	}
} 