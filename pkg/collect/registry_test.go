package collect

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"go.podman.io/image/v5/transports/alltransports"
	"k8s.io/client-go/rest"
)

// fakeRegistry is a minimal Docker Registry v2 stand-in for unit testing
// imageExists. The handler returns whatever the caller puts in `manifest`
// for /v2/{name}/manifests/{ref}. /v2/ is always a 200.
type fakeRegistry struct {
	server   *httptest.Server
	manifest http.HandlerFunc
}

func newFakeRegistry(t *testing.T, manifest http.HandlerFunc) *fakeRegistry {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v2/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" || r.URL.Path == "/v2" {
			w.Header().Set("Docker-Distribution-Api-Version", "registry/2.0")
			w.WriteHeader(http.StatusOK)
			return
		}
		manifest(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &fakeRegistry{server: srv, manifest: manifest}
}

// hostPort strips "http://" from the test server URL, leaving "127.0.0.1:NNNN"
// suitable for use as the registry portion of an image reference.
func (f *fakeRegistry) hostPort() string {
	return strings.TrimPrefix(f.server.URL, "http://")
}

func TestImageExists_Found(t *testing.T) {
	fr := newFakeRegistry(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		w.Header().Set("Docker-Content-Digest", "sha256:1111111111111111111111111111111111111111111111111111111111111111")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","size":1,"digest":"sha256:2222222222222222222222222222222222222222222222222222222222222222"},"layers":[]}`))
	})

	collector := &v1beta2.RegistryImages{
		Images: []string{fmt.Sprintf("%s/test:latest", fr.hostPort())},
	}
	exists, err := imageExists("default", &rest.Config{}, collector, fmt.Sprintf("%s/test:latest", fr.hostPort()), 5*time.Second)

	assert.NoError(t, err)
	assert.True(t, exists)
}

func TestImageExists_NotFound(t *testing.T) {
	fr := newFakeRegistry(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":[{"code":"MANIFEST_UNKNOWN","message":"manifest unknown"}]}`))
	})

	collector := &v1beta2.RegistryImages{
		Images: []string{fmt.Sprintf("%s/test:latest", fr.hostPort())},
	}
	exists, err := imageExists("default", &rest.Config{}, collector, fmt.Sprintf("%s/test:latest", fr.hostPort()), 5*time.Second)

	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestImageExists_Unauthorized(t *testing.T) {
	fr := newFakeRegistry(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Www-Authenticate", `Basic realm="registry"`)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"errors":[{"code":"UNAUTHORIZED","message":"authentication required"}]}`))
	})

	collector := &v1beta2.RegistryImages{
		Images: []string{fmt.Sprintf("%s/test:latest", fr.hostPort())},
	}
	exists, err := imageExists("default", &rest.Config{}, collector, fmt.Sprintf("%s/test:latest", fr.hostPort()), 5*time.Second)

	assert.Error(t, err)
	assert.False(t, exists)
}

func TestImageExists_RetriesOnEOF(t *testing.T) {
	var attempts int32
	fr := newFakeRegistry(t, func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			// hijack the connection and slam it shut to cause an EOF on the client
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatalf("response writer does not support hijacking")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatalf("hijack: %v", err)
			}
			_ = conn.Close()
			return
		}
		w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		w.Header().Set("Docker-Content-Digest", "sha256:1111111111111111111111111111111111111111111111111111111111111111")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"schemaVersion":2,"mediaType":"application/vnd.docker.distribution.manifest.v2+json","config":{"mediaType":"application/vnd.docker.container.image.v1+json","size":1,"digest":"sha256:2222222222222222222222222222222222222222222222222222222222222222"},"layers":[]}`))
	})

	collector := &v1beta2.RegistryImages{
		Images: []string{fmt.Sprintf("%s/test:latest", fr.hostPort())},
	}
	exists, err := imageExists("default", &rest.Config{}, collector, fmt.Sprintf("%s/test:latest", fr.hostPort()), 5*time.Second)

	assert.NoError(t, err)
	assert.True(t, exists)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&attempts), int32(3))
}

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
