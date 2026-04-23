package collect

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectHostRegistryImagesTitle(t *testing.T) {
	tests := []struct {
		name     string
		meta     troubleshootv1beta2.HostCollectorMeta
		expected string
	}{
		{
			name:     "default title",
			meta:     troubleshootv1beta2.HostCollectorMeta{},
			expected: "Registry Images",
		},
		{
			name: "custom title",
			meta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: "My Registry",
			},
			expected: "My Registry",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := &CollectHostRegistryImages{
				hostCollector: &troubleshootv1beta2.HostRegistryImages{
					HostCollectorMeta: test.meta,
				},
			}
			assert.Equal(t, test.expected, c.Title())
		})
	}
}

func TestCollectHostRegistryImagesResolveAuth(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		expected *registryAuthConfig
	}{
		{
			name:     "nil when no credentials",
			username: "",
			password: "",
			expected: nil,
		},
		{
			name:     "returns auth with credentials",
			username: "user",
			password: "pass",
			expected: &registryAuthConfig{
				username: "user",
				password: "pass",
			},
		},
		{
			name:     "returns auth with username only",
			username: "user",
			password: "",
			expected: &registryAuthConfig{
				username: "user",
				password: "",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := &CollectHostRegistryImages{
				hostCollector: &troubleshootv1beta2.HostRegistryImages{
					Username: test.username,
					Password: test.password,
				},
			}
			result := c.resolveAuth()
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestCollectHostRegistryImagesRemoteCollect(t *testing.T) {
	c := &CollectHostRegistryImages{
		hostCollector: &troubleshootv1beta2.HostRegistryImages{},
	}
	result, err := c.RemoteCollect(nil)
	require.ErrorIs(t, err, ErrRemoteCollectorNotImplemented)
	assert.Nil(t, result)
}
