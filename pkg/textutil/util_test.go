package textutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_stripSpaces(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{
			name:   "spaces",
			input:  "  some word here   ",
			expect: "somewordhere",
		},
		{
			name:   "tabs",
			input:  "\t\tsome\tword\there\t",
			expect: "somewordhere",
		},
		{
			name:   "spaces and tabs",
			input:  "\t\t     some    \tword    \there    \t",
			expect: "somewordhere",
		},
		{
			name:   "new lines",
			input:  "\nsome\nword\nhere\n\n\n",
			expect: "somewordhere",
		},
		{
			name:   "form feed",
			input:  "\fsome\fword\fhere\f\f\f",
			expect: "somewordhere",
		},
		{
			name:   "carriage return",
			input:  "\rsome\rword\rhere\r\r\r",
			expect: "somewordhere",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := StripSpaces(test.input)
			assert.Equal(t, test.expect, actual)
		})
	}
}
