package collect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCollectorResult_AddResult(t *testing.T) {
	r := CollectorResult{"a": []byte("a")}

	other := CollectorResult{"b": []byte("b")}
	r.AddResult(other)

	assert.Equal(t, 2, len(r))
	assert.Equal(t, []byte("a"), r["a"])
	assert.Equal(t, []byte("b"), r["b"])
}
