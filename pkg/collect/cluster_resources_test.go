package collect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_SelectCRDVersionByPriority(t *testing.T) {
	assert.Equal(t, "v1alpha3", selectCRDVersionByPriority([]string{"v1alpha2", "v1alpha3"}))
	assert.Equal(t, "v1alpha3", selectCRDVersionByPriority([]string{"v1alpha3", "v1alpha2"}))
	assert.Equal(t, "v1", selectCRDVersionByPriority([]string{"v1alpha2", "v1alpha3", "v1"}))
	assert.Equal(t, "v1", selectCRDVersionByPriority([]string{"v1", "v1alpha2", "v1alpha3"}))
}
