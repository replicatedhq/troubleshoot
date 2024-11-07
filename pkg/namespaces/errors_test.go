package namespaces

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWrapIfFail(t *testing.T) {
	for _, tt := range []struct {
		name        string
		msg         string
		originalerr error
		fn          func() error
		expectederr string
	}{
		{
			name:        "function succeeds, no original error",
			msg:         "test message",
			originalerr: nil,
			fn:          func() error { return nil },
			expectederr: "",
		},
		{
			name:        "no original error, function fails",
			msg:         "test message",
			originalerr: nil,
			fn:          func() error { return fmt.Errorf("test error") },
			expectederr: "test error",
		},
		{
			name:        "original error, function succeeds",
			msg:         "test message",
			originalerr: fmt.Errorf("original error"),
			fn:          func() error { return nil },
			expectederr: "original error",
		},
		{
			name:        "original error, and function fails",
			msg:         "test message",
			originalerr: fmt.Errorf("original error"),
			fn:          func() error { return fmt.Errorf("func error") },
			expectederr: "test message: func error: original error",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := WrapIfFail(tt.msg, tt.originalerr, tt.fn)
			if tt.expectederr == "" {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectederr)
		})
	}
}
