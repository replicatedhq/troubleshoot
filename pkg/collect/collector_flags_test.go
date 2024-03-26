package collect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectorModes(t *testing.T) {
	tests := []struct {
		name string
		fn   func() bool
		want bool
	}{
		{
			name: "requires root",
			fn: func() bool {
				return RequireRoot.RequiresRoot()
			},
			want: true,
		},
		{
			name: "does not require root",
			fn: func() bool {
				cm := CollectorFlags(0)
				return cm.RequiresRoot()
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.fn())
		})
	}
}
