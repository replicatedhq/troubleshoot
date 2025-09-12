package collect

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/containers/image/v5/transports/alltransports"
	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
)

func TestGetImageAuthConfigFromData(t *testing.T) {
	tests := []struct {
		name             string
		imageName        string
		dockerConfigJSON string
		expectedUsername string
		expectedPassword string
		expectedError    bool
	}{
		{
			name:             "docker.io auth",
			imageName:        "docker.io/myimage",
			dockerConfigJSON: `{"auths":{"docker.io":{"auth":"username:password"}}}`,
			expectedUsername: "username",
			expectedPassword: "password",
			expectedError:    false,
		},
		{
			name:             "docker.io auth multi colon",
			imageName:        "docker.io/myimage",
			dockerConfigJSON: `{"auths":{"docker.io":{"auth":"user:name:pass:word"}}}`,
			expectedError:    true,
		},
		{
			name:             "gcr.io auth",
			imageName:        "gcr.io/myimage",
			dockerConfigJSON: `{"auths":{"gcr.io":{"username":"_json_key","password":"sa-key"}}}`,
			expectedUsername: "_json_key",
			expectedPassword: "sa-key",
			expectedError:    false,
		},
		{
			name:             "proxy.replicated.com auth base64 encoded",
			imageName:        "proxy.replicated.com/app-slug/myimage",
			dockerConfigJSON: `{"auths":{"proxy.replicated.com":{"auth":"bGljZW5zZV9pZF8xOmxpY2Vuc2VfaWRfMQ=="}}}`,
			expectedUsername: "license_id_1",
			expectedPassword: "license_id_1",
			expectedError:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			imageRef, err := alltransports.ParseImageName(fmt.Sprintf("docker://%s", test.imageName))
			assert.NoError(t, err)

			pullSecrets := &v1beta2.ImagePullSecrets{
				SecretType: "kubernetes.io/dockerconfigjson",
				Data: map[string]string{
					".dockerconfigjson": base64.StdEncoding.EncodeToString([]byte(test.dockerConfigJSON)),
				},
			}

			authConfig, err := getImageAuthConfigFromData(imageRef, pullSecrets)
			if test.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, authConfig)
			assert.Equal(t, test.expectedUsername, authConfig.username)
			assert.Equal(t, test.expectedPassword, authConfig.password)
		})
	}
}

func TestImageExists_ContextDeadlineExceeded(t *testing.T) {
	// Start a server that accepts connections but doesn't respond
	// This will cause the context deadline exceeded error
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port

	// Start a goroutine that accepts connections but doesn't respond
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Keep the connection open but don't respond
			// This will cause the client to timeout
			time.Sleep(2 * time.Second)
			conn.Close()
		}
	}()

	// Create a test registry collector pointing to our test server
	registryCollector := &v1beta2.RegistryImages{
		Images: []string{fmt.Sprintf("127.0.0.1:%d/test-image:latest", port)},
	}

	// Create a minimal client config
	clientConfig := &rest.Config{}

	// Test the imageExists function - this should trigger context deadline exceeded
	exists, err := imageExists("default", clientConfig, registryCollector, fmt.Sprintf("127.0.0.1:%d/test-image:latest", port), 100*time.Millisecond)

	// Verify the result - should return false without error due to context deadline exceeded handling
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.False(t, exists)
}
