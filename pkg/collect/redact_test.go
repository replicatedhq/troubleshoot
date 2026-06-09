package collect

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_maxConcurrentRedactors(t *testing.T) {
	tests := []struct {
		name   string
		setEnv bool
		value  string
		want   int
	}{
		{
			name:   "env unset returns default",
			setEnv: false,
			want:   DefaultMaxConcurrentRedactors,
		},
		{
			name:   "empty value returns default",
			setEnv: true,
			value:  "",
			want:   DefaultMaxConcurrentRedactors,
		},
		{
			name:   "valid positive int overrides default",
			setEnv: true,
			value:  "50",
			want:   50,
		},
		{
			name:   "value equal to default is honored",
			setEnv: true,
			value:  "10",
			want:   DefaultMaxConcurrentRedactors,
		},
		{
			name:   "zero falls back to default",
			setEnv: true,
			value:  "0",
			want:   DefaultMaxConcurrentRedactors,
		},
		{
			name:   "negative value falls back to default",
			setEnv: true,
			value:  "-3",
			want:   DefaultMaxConcurrentRedactors,
		},
		{
			name:   "non-numeric value falls back to default",
			setEnv: true,
			value:  "potato",
			want:   DefaultMaxConcurrentRedactors,
		},
		{
			name:   "whitespace-padded value falls back to default",
			setEnv: true,
			value:  "  4  ",
			want:   DefaultMaxConcurrentRedactors,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Snapshot inherited state so each subtest is hermetic regardless
			// of the order Go picks. t.Setenv would mask "env unset" cases
			// with an empty value, so we manage the env directly here.
			prev, hadPrev := os.LookupEnv(MaxConcurrentRedactorsEnvVar)
			t.Cleanup(func() {
				if hadPrev {
					_ = os.Setenv(MaxConcurrentRedactorsEnvVar, prev)
				} else {
					_ = os.Unsetenv(MaxConcurrentRedactorsEnvVar)
				}
			})

			if tt.setEnv {
				if err := os.Setenv(MaxConcurrentRedactorsEnvVar, tt.value); err != nil {
					t.Fatalf("setenv: %v", err)
				}
			} else {
				if err := os.Unsetenv(MaxConcurrentRedactorsEnvVar); err != nil {
					t.Fatalf("unsetenv: %v", err)
				}
			}

			got := maxConcurrentRedactors()
			assert.Equal(t, tt.want, got)
		})
	}
}
