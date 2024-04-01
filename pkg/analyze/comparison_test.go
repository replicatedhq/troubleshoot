package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseComparisonOperator(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ComparisonOperator
		wantErr bool
	}{
		{
			name:  "equal",
			input: "=",
			want:  Equal,
		},
		{
			name:  "equal",
			input: "==",
			want:  Equal,
		},
		{
			name:  "equal",
			input: "===",
			want:  Equal,
		},
		{
			name:  "not equal",
			input: "!=",
			want:  NotEqual,
		},
		{
			name:  "not equal",
			input: "!==",
			want:  NotEqual,
		},
		{
			name:  "less than",
			input: "<",
			want:  LessThan,
		},
		{
			name:  "greater than",
			input: ">",
			want:  GreaterThan,
		},
		{
			name:  "less than or equal",
			input: "<=",
			want:  LessThanOrEqual,
		},
		{
			name:  "greater than or equal",
			input: ">=",
			want:  GreaterThanOrEqual,
		},
		{
			name:    "invalid operator 1",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid operator 2",
			input:   "gibberish",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseComparisonOperator(tt.input)
			assert.Equal(t, tt.want, got, "ParseOperator() = %v, want %v", got, tt.want)
			assert.Equalf(t, tt.wantErr, err != nil, "ParseOperator() error = %v, wantErr %v", err, tt.wantErr)
		})
	}
}
