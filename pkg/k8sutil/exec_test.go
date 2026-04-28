package k8sutil

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	restclient "k8s.io/client-go/rest"
)

func TestNewFallbackExecutor(t *testing.T) {
	config := &restclient.Config{Host: "http://localhost:8080"}
	u, err := url.Parse("http://localhost:8080/api/v1/namespaces/default/pods/foo/exec")
	require.NoError(t, err)

	exec, err := NewFallbackExecutor(config, "POST", u)
	require.NoError(t, err)
	require.NotNil(t, exec)
}
