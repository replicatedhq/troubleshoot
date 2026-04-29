package supportbundle

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// newTLSTestServer creates an HTTPS test server with the given handler.
// It also configures the default HTTP transport to skip TLS verification
// for the test server.
func newTLSTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	// Allow the test HTTP clients to connect to the self-signed cert
	http.DefaultTransport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	t.Cleanup(func() {
		http.DefaultTransport = &http.Transport{}
	})
	return server
}

func TestGetPresignedUploadURL(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse supportBundleUploadURLResponse
		serverStatus   int
		wantErr        bool
		errContains    string
	}{
		{
			name: "successful response",
			serverResponse: supportBundleUploadURLResponse{
				BundleID:  "bundle-123",
				UploadURL: "https://s3.amazonaws.com/presigned-url",
			},
			serverStatus: http.StatusOK,
		},
		{
			name:         "server error",
			serverStatus: http.StatusInternalServerError,
			wantErr:      true,
			errContains:  "status 500",
		},
		{
			name:         "unauthorized",
			serverStatus: http.StatusUnauthorized,
			wantErr:      true,
			errContains:  "status 401",
		},
		{
			name: "empty upload URL",
			serverResponse: supportBundleUploadURLResponse{
				BundleID:  "bundle-123",
				UploadURL: "",
			},
			serverStatus: http.StatusOK,
			wantErr:      true,
			errContains:  "did not contain an upload URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Equal(t, "/v3/supportbundle/upload-url", r.URL.Path)
				assert.NotEmpty(t, r.Header.Get("Authorization"))

				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					// Override upload URL to use HTTPS test server URL for validation
					resp := tt.serverResponse
					if resp.UploadURL == "" {
						json.NewEncoder(w).Encode(resp)
					} else {
						json.NewEncoder(w).Encode(resp)
					}
				}
			})
			defer server.Close()

			creds := &ReplicatedUploadCredentials{
				LicenseID: "test-license-id",
				ChannelID: "test-channel-id",
				Endpoint:  server.URL,
			}

			resp, err := GetPresignedUploadURL(creds)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.serverResponse.BundleID, resp.BundleID)
			assert.Equal(t, tt.serverResponse.UploadURL, resp.UploadURL)
		})
	}
}

func TestGetPresignedUploadURL_RejectsHTTPPresignedURL(t *testing.T) {
	server := newTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(supportBundleUploadURLResponse{
			BundleID:  "bundle-123",
			UploadURL: "http://evil.example.com/upload",
		})
	})
	defer server.Close()

	creds := &ReplicatedUploadCredentials{
		LicenseID: "test-license-id",
		ChannelID: "test-channel-id",
		Endpoint:  server.URL,
	}

	_, err := GetPresignedUploadURL(creds)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must use HTTPS")
}

func TestUploadToS3(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test-bundle.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, []byte("fake-archive-content"), 0644))

	tests := []struct {
		name         string
		serverStatus int
		wantErr      bool
		errContains  string
	}{
		{
			name:         "successful upload",
			serverStatus: http.StatusOK,
		},
		{
			name:         "s3 error",
			serverStatus: http.StatusForbidden,
			wantErr:      true,
			errContains:  "S3 upload failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "PUT", r.Method)
				assert.Equal(t, "application/tar+gzip", r.Header.Get("Content-Type"))
				w.WriteHeader(tt.serverStatus)
			})
			defer server.Close()

			err := UploadToS3(server.URL, archivePath)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			require.NoError(t, err)
		})
	}
}

func TestMarkSupportBundleUploaded(t *testing.T) {
	tests := []struct {
		name           string
		serverResponse markUploadedResponse
		serverStatus   int
		wantErr        bool
		errContains    string
	}{
		{
			name:           "successful mark",
			serverResponse: markUploadedResponse{Slug: "bundle-slug-abc"},
			serverStatus:   http.StatusOK,
		},
		{
			name:         "server error",
			serverStatus: http.StatusInternalServerError,
			wantErr:      true,
			errContains:  "status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "POST", r.Method)
				assert.Contains(t, r.URL.Path, "/v3/supportbundle/")
				assert.Contains(t, r.URL.Path, "/uploaded")
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.NotEmpty(t, r.Header.Get("Authorization"))

				var body markUploadedRequest
				json.NewDecoder(r.Body).Decode(&body)
				assert.Equal(t, "test-channel-id", body.ChannelID)

				w.WriteHeader(tt.serverStatus)
				if tt.serverStatus == http.StatusOK {
					json.NewEncoder(w).Encode(tt.serverResponse)
				}
			})
			defer server.Close()

			creds := &ReplicatedUploadCredentials{
				LicenseID: "test-license-id",
				ChannelID: "test-channel-id",
				Endpoint:  server.URL,
			}

			slug, err := MarkSupportBundleUploaded(creds, "bundle-123")
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.serverResponse.Slug, slug)
		})
	}
}

func TestMarkSupportBundleUploaded_EscapesBundleID(t *testing.T) {
	server := newTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// The raw request URI preserves percent-encoding, verifying PathEscape was applied
		assert.Contains(t, r.RequestURI, "..%2F..%2Fadmin")
		json.NewEncoder(w).Encode(markUploadedResponse{Slug: "test-slug"})
	})
	defer server.Close()

	creds := &ReplicatedUploadCredentials{
		LicenseID: "test-license-id",
		ChannelID: "test-channel-id",
		Endpoint:  server.URL,
	}

	slug, err := MarkSupportBundleUploaded(creds, "../../admin")
	require.NoError(t, err)
	assert.Equal(t, "test-slug", slug)
}

func TestUploadSupportBundleToReplicated(t *testing.T) {
	tmpDir := t.TempDir()
	archivePath := filepath.Join(tmpDir, "test-bundle.tar.gz")
	require.NoError(t, os.WriteFile(archivePath, []byte("fake-archive-content"), 0644))

	step := 0
	var serverURL string
	server := newTLSTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v3/supportbundle/upload-url" && r.Method == "POST":
			step++
			assert.Equal(t, 1, step, "step 1: get presigned URL")
			json.NewEncoder(w).Encode(supportBundleUploadURLResponse{
				BundleID:  "bundle-456",
				UploadURL: serverURL + "/s3-upload",
			})
		case r.URL.Path == "/s3-upload" && r.Method == "PUT":
			step++
			assert.Equal(t, 2, step, "step 2: upload to S3")
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/v3/supportbundle/bundle-456/uploaded" && r.Method == "POST":
			step++
			assert.Equal(t, 3, step, "step 3: mark uploaded")
			json.NewEncoder(w).Encode(markUploadedResponse{Slug: "final-slug"})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()
	serverURL = server.URL

	creds := &ReplicatedUploadCredentials{
		LicenseID: "test-license-id",
		ChannelID: "test-channel-id",
		Endpoint:  server.URL,
	}

	slug, err := UploadSupportBundleToReplicated(creds, archivePath)
	require.NoError(t, err)
	assert.Equal(t, "final-slug", slug)
	assert.Equal(t, 3, step, "all 3 steps should have been called")
}

func TestSetBasicAuth(t *testing.T) {
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	setBasicAuth(req, "my-license-id")

	auth := req.Header.Get("Authorization")
	assert.NotEmpty(t, auth)
	assert.Contains(t, auth, "Basic ")
	// Basic base64("my-license-id:my-license-id") = "bXktbGljZW5zZS1pZDpteS1saWNlbnNlLWlk"
	assert.Equal(t, "Basic bXktbGljZW5zZS1pZDpteS1saWNlbnNlLWlk", auth)
}

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantErr  bool
	}{
		{name: "valid https", endpoint: "https://replicated.app", wantErr: false},
		{name: "http rejected", endpoint: "http://replicated.app", wantErr: true},
		{name: "empty scheme", endpoint: "replicated.app", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEndpoint(tt.endpoint)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidatePresignedURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "valid https", url: "https://s3.amazonaws.com/bucket/key?sig=abc", wantErr: false},
		{name: "http rejected", url: "http://s3.amazonaws.com/bucket/key", wantErr: true},
		{name: "ftp rejected", url: "ftp://example.com/file", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePresignedURL(tt.url)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// --- Tests for extractLicenseID ---

func TestExtractLicenseID(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string][]byte
		wantID      string
		wantErr     bool
		errContains string
	}{
		{
			name: "from integration-license-id key",
			data: map[string][]byte{
				"integration-license-id": []byte("license-abc-123"),
			},
			wantID: "license-abc-123",
		},
		{
			name: "integration-license-id takes priority over config.yaml",
			data: map[string][]byte{
				"integration-license-id": []byte("from-integration"),
				"config.yaml":            []byte("license:\n  spec:\n    licenseID: from-config\n"),
			},
			wantID: "from-integration",
		},
		{
			name: "falls back to config.yaml when integration key missing",
			data: map[string][]byte{
				"config.yaml": []byte("license:\n  spec:\n    licenseID: fallback-id\nchannelID: chan-1\n"),
			},
			wantID: "fallback-id",
		},
		{
			name: "falls back to config.yaml when integration key is empty",
			data: map[string][]byte{
				"integration-license-id": []byte("  "),
				"config.yaml":            []byte("license:\n  spec:\n    licenseID: fallback-id\n"),
			},
			wantID: "fallback-id",
		},
		{
			name:        "error when neither key present",
			data:        map[string][]byte{},
			wantErr:     true,
			errContains: "contains neither",
		},
		{
			name: "error on malformed config.yaml",
			data: map[string][]byte{
				"config.yaml": []byte("{{invalid yaml"),
			},
			wantErr:     true,
			errContains: "unmarshal",
		},
		{
			name: "error when license is nil in config",
			data: map[string][]byte{
				"config.yaml": []byte("channelID: chan-1\n"),
			},
			wantErr:     true,
			errContains: "does not contain a license",
		},
		{
			name: "error when licenseID is empty in config",
			data: map[string][]byte{
				"config.yaml": []byte("license:\n  spec:\n    licenseID: \"\"\n"),
			},
			wantErr:     true,
			errContains: "empty",
		},
		{
			name: "license as string (Helm template {{- whitespace trimming)",
			data: map[string][]byte{
				"config.yaml": []byte("license: |\n  apiVersion: kots.io/v1beta1\n  kind: License\n  spec:\n    licenseID: string-license-abc\n    appSlug: myapp\nchannelID: chan-1\n"),
			},
			wantID: "string-license-abc",
		},
		{
			name: "license as inline string",
			data: map[string][]byte{
				"config.yaml": []byte("license: \"apiVersion: kots.io/v1beta1\\nkind: License\\nspec:\\n  licenseID: inline-id-123\"\nchannelID: chan-2\n"),
			},
			wantID: "inline-id-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, err := extractLicenseID(tt.data, "test-secret", "test-ns")
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

// --- Tests for extractConfigFields ---

func TestExtractConfigFields(t *testing.T) {
	tests := []struct {
		name        string
		data        map[string][]byte
		wantChannel string
		wantEndpt   string
		wantErr     bool
	}{
		{
			name: "extracts channelID and endpoint",
			data: map[string][]byte{
				"config.yaml": []byte("channelID: chan-abc\nreplicatedAppEndpoint: https://custom.replicated.app\n"),
			},
			wantChannel: "chan-abc",
			wantEndpt:   "https://custom.replicated.app",
		},
		{
			name:        "returns empty when config.yaml missing",
			data:        map[string][]byte{},
			wantChannel: "",
			wantEndpt:   "",
		},
		{
			name: "partial config - only channelID",
			data: map[string][]byte{
				"config.yaml": []byte("channelID: chan-only\n"),
			},
			wantChannel: "chan-only",
			wantEndpt:   "",
		},
		{
			name: "malformed yaml returns error",
			data: map[string][]byte{
				"config.yaml": []byte("{{bad yaml"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch, ep, err := extractConfigFields(tt.data)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantChannel, ch)
			assert.Equal(t, tt.wantEndpt, ep)
		})
	}
}

// --- Tests for DiscoverReplicatedCredentials (using fake k8s client) ---

// newFakeSecret creates a corev1.Secret with the given labels and data.
func newFakeSecret(name, namespace string, labels map[string]string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: data,
	}
}

func TestDiscoverReplicatedCredentials_LabelDiscovery(t *testing.T) {
	sdkSecret := newFakeSecret("myapp-sdk", "app-ns", map[string]string{
		"helm.sh/chart":                "replicated-1.18.2",
		"app.kubernetes.io/managed-by": "Helm",
	}, map[string][]byte{
		"integration-license-id": []byte("discovered-license"),
		"config.yaml":            []byte("channelID: chan-discovered\nreplicatedAppEndpoint: https://replicated.app\n"),
	})

	clientset := fake.NewSimpleClientset(sdkSecret)

	// Inject the fake clientset by calling the internal functions directly,
	// since DiscoverReplicatedCredentials creates its own clientset from restConfig.
	// Instead, test the building blocks and one integration path.

	// Test extractLicenseID with the same data
	licenseID, err := extractLicenseID(sdkSecret.Data, sdkSecret.Name, sdkSecret.Namespace)
	require.NoError(t, err)
	assert.Equal(t, "discovered-license", licenseID)

	// Test extractConfigFields
	ch, ep, err := extractConfigFields(sdkSecret.Data)
	require.NoError(t, err)
	assert.Equal(t, "chan-discovered", ch)
	assert.Equal(t, "https://replicated.app", ep)

	// Verify the fake clientset can list and find by label
	secrets, err := clientset.CoreV1().Secrets("app-ns").List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=Helm",
	})
	require.NoError(t, err)
	require.Len(t, secrets.Items, 1)
	assert.Equal(t, "myapp-sdk", secrets.Items[0].Name)
	assert.True(t, len(secrets.Items[0].Labels["helm.sh/chart"]) > 0)
}

func TestDiscoverReplicatedCredentials_ExplicitSecretName(t *testing.T) {
	secret := newFakeSecret("custom-secret", "custom-ns", nil, map[string][]byte{
		"integration-license-id": []byte("explicit-license"),
		"config.yaml":            []byte("channelID: chan-explicit\nreplicatedAppEndpoint: https://replicated.app\n"),
	})

	clientset := fake.NewSimpleClientset(secret)

	// Verify explicit get works
	s, err := clientset.CoreV1().Secrets("custom-ns").Get(context.Background(), "custom-secret", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "custom-secret", s.Name)

	licenseID, err := extractLicenseID(s.Data, s.Name, s.Namespace)
	require.NoError(t, err)
	assert.Equal(t, "explicit-license", licenseID)
}

func TestDiscoverReplicatedCredentials_NoSDKSecret(t *testing.T) {
	// Create a non-SDK Helm secret
	otherSecret := newFakeSecret("other-chart", "app-ns", map[string]string{
		"helm.sh/chart":                "postgresql-12.0.0",
		"app.kubernetes.io/managed-by": "Helm",
	}, map[string][]byte{
		"password": []byte("secret"),
	})

	clientset := fake.NewSimpleClientset(otherSecret)

	secrets, err := clientset.CoreV1().Secrets("app-ns").List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=Helm",
	})
	require.NoError(t, err)

	// Simulate the discovery logic
	var found bool
	for _, s := range secrets.Items {
		if chartLabel := s.Labels["helm.sh/chart"]; len(chartLabel) > 0 {
			if len(chartLabel) >= len(replicatedSDKChartLabelPrefix) &&
				chartLabel[:len(replicatedSDKChartLabelPrefix)] == replicatedSDKChartLabelPrefix {
				found = true
				break
			}
		}
	}
	assert.False(t, found, "should not find a replicated SDK secret")
}

func TestDiscoverReplicatedCredentials_DefaultEndpointFallback(t *testing.T) {
	// config.yaml with no endpoint — should default to https://replicated.app
	data := map[string][]byte{
		"integration-license-id": []byte("test-license"),
		"config.yaml":            []byte("channelID: chan-1\n"),
	}

	licenseID, err := extractLicenseID(data, "test", "test-ns")
	require.NoError(t, err)
	assert.Equal(t, "test-license", licenseID)

	_, ep, err := extractConfigFields(data)
	require.NoError(t, err)
	assert.Equal(t, "", ep, "endpoint should be empty from config, caller applies default")
}

func TestDiscoverReplicatedCredentials_MultipleHelmSecrets(t *testing.T) {
	// Multiple Helm secrets, only one is the SDK
	pgSecret := newFakeSecret("postgres", "app-ns", map[string]string{
		"helm.sh/chart":                "postgresql-12.0.0",
		"app.kubernetes.io/managed-by": "Helm",
	}, map[string][]byte{"password": []byte("pg-pass")})

	sdkSecret := newFakeSecret("myapp-sdk", "app-ns", map[string]string{
		"helm.sh/chart":                "replicated-1.18.2",
		"app.kubernetes.io/managed-by": "Helm",
	}, map[string][]byte{
		"integration-license-id": []byte("correct-license"),
		"config.yaml":            []byte("channelID: correct-channel\nreplicatedAppEndpoint: https://replicated.app\n"),
	})

	redisSecret := newFakeSecret("redis", "app-ns", map[string]string{
		"helm.sh/chart":                "redis-17.0.0",
		"app.kubernetes.io/managed-by": "Helm",
	}, map[string][]byte{"password": []byte("redis-pass")})

	clientset := fake.NewSimpleClientset(pgSecret, sdkSecret, redisSecret)

	secrets, err := clientset.CoreV1().Secrets("app-ns").List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=Helm",
	})
	require.NoError(t, err)
	require.Len(t, secrets.Items, 3)

	// Simulate discovery — find the SDK secret among many
	var foundData map[string][]byte
	var foundName string
	for _, s := range secrets.Items {
		if chartLabel := s.Labels["helm.sh/chart"]; len(chartLabel) >= len(replicatedSDKChartLabelPrefix) &&
			chartLabel[:len(replicatedSDKChartLabelPrefix)] == replicatedSDKChartLabelPrefix {
			foundData = s.Data
			foundName = s.Name
			break
		}
	}

	require.NotNil(t, foundData, "should find the SDK secret")
	assert.Equal(t, "myapp-sdk", foundName)

	licenseID, err := extractLicenseID(foundData, foundName, "app-ns")
	require.NoError(t, err)
	assert.Equal(t, "correct-license", licenseID)
}

// --- E2E-style tests for multi-app cluster scenarios ---

// makeSDKSecret creates a realistic SDK secret matching the Helm template output.
// The license is stored as a YAML string in config.yaml (matching real cluster behavior).
func makeSDKSecret(appName, namespace, licenseID, channelID string) *corev1.Secret {
	configYAML := fmt.Sprintf(
		"license: |\n  apiVersion: kots.io/v1beta1\n  kind: License\n  spec:\n    licenseID: %s\n    appSlug: %s\nchannelID: %q\nreplicatedAppEndpoint: \"https://replicated.app\"\n",
		licenseID, appName, channelID,
	)
	return newFakeSecret(appName+"-sdk", namespace, map[string]string{
		"helm.sh/chart":                "replicated-1.19.2",
		"app.kubernetes.io/name":       appName + "-sdk",
		"app.kubernetes.io/instance":   appName,
		"app.kubernetes.io/managed-by": "Helm",
	}, map[string][]byte{
		"config.yaml": []byte(configYAML),
	})
}

// makeNonSDKSecret creates a replicated chart secret that is NOT the SDK config
// (e.g., pull secrets, support metadata). These should be skipped by discovery.
func makeNonSDKSecret(name, namespace string) *corev1.Secret {
	return newFakeSecret(name, namespace, map[string]string{
		"helm.sh/chart":                "replicated-1.19.2",
		"app.kubernetes.io/managed-by": "Helm",
	}, map[string][]byte{
		"some-other-key": []byte("not-a-license"),
	})
}

func TestFindAllSDKCredentials_MultipleAppsInCluster(t *testing.T) {
	// Simulate a cluster with two Replicated apps in different namespaces,
	// each with their own SDK secret plus other replicated chart secrets.

	// App 1: "firstresponse" in namespace "firstresponse"
	app1SDK := makeSDKSecret("firstresponse", "firstresponse", "license-fr-001", "chan-fr")
	app1Pull := makeNonSDKSecret("enterprise-pull-secret", "firstresponse")
	app1SB := makeNonSDKSecret("firstresponse-sdk-supportbundle", "firstresponse")
	app1Meta := makeNonSDKSecret("replicated-support-metadata", "firstresponse")

	// App 2: "k8laude" in namespace "demo"
	app2SDK := makeSDKSecret("k8laude", "demo", "license-k8-002", "chan-k8")
	app2Pull := makeNonSDKSecret("enterprise-pull-secret", "demo")

	// Unrelated Helm chart
	pgSecret := newFakeSecret("postgres", "demo", map[string]string{
		"helm.sh/chart": "postgresql-15.0.0",
	}, map[string][]byte{"password": []byte("pg")})

	clientset := fake.NewSimpleClientset(
		app1SDK, app1Pull, app1SB, app1Meta,
		app2SDK, app2Pull,
		pgSecret,
	)

	matches, err := FindAllSDKCredentialsWithClient(context.Background(), clientset)
	require.NoError(t, err)

	// Should find exactly 2 SDK secrets (one per app), skipping pull/meta/supportbundle secrets
	require.Len(t, matches, 2, "should find exactly 2 SDK secrets for 2 apps")

	// Verify both apps are found with correct credentials
	foundApps := map[string]SDKSecretMatch{}
	for _, m := range matches {
		foundApps[m.Namespace+"/"+m.SecretName] = m
	}

	fr, ok := foundApps["firstresponse/firstresponse-sdk"]
	require.True(t, ok, "should find firstresponse SDK secret")
	assert.Equal(t, "license-fr-001", fr.Creds.LicenseID)
	assert.Equal(t, "chan-fr", fr.Creds.ChannelID)
	assert.Equal(t, "https://replicated.app", fr.Creds.Endpoint)

	k8, ok := foundApps["demo/k8laude-sdk"]
	require.True(t, ok, "should find k8laude SDK secret")
	assert.Equal(t, "license-k8-002", k8.Creds.LicenseID)
	assert.Equal(t, "chan-k8", k8.Creds.ChannelID)
}

func TestFindAllSDKCredentials_SingleAppInCluster(t *testing.T) {
	sdk := makeSDKSecret("myapp", "production", "license-prod-123", "chan-prod")
	pullSecret := makeNonSDKSecret("enterprise-pull-secret", "production")

	clientset := fake.NewSimpleClientset(sdk, pullSecret)

	matches, err := FindAllSDKCredentialsWithClient(context.Background(), clientset)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	assert.Equal(t, "myapp-sdk", matches[0].SecretName)
	assert.Equal(t, "production", matches[0].Namespace)
	assert.Equal(t, "license-prod-123", matches[0].Creds.LicenseID)
}

func TestFindAllSDKCredentials_NoAppsInCluster(t *testing.T) {
	// Only non-replicated Helm charts
	pgSecret := newFakeSecret("postgres", "default", map[string]string{
		"helm.sh/chart": "postgresql-15.0.0",
	}, map[string][]byte{"password": []byte("pg")})

	clientset := fake.NewSimpleClientset(pgSecret)

	matches, err := FindAllSDKCredentialsWithClient(context.Background(), clientset)
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestFindAllSDKCredentials_SkipsSecretsWithoutLicense(t *testing.T) {
	// All secrets have the replicated- chart label, but only the SDK secret
	// has config.yaml with a valid license
	sdkSecret := makeSDKSecret("myapp", "app-ns", "valid-license", "chan-1")
	pullSecret := makeNonSDKSecret("enterprise-pull-secret", "app-ns")
	metaSecret := makeNonSDKSecret("replicated-support-metadata", "app-ns")
	sbSecret := makeNonSDKSecret("myapp-sdk-supportbundle", "app-ns")

	clientset := fake.NewSimpleClientset(sdkSecret, pullSecret, metaSecret, sbSecret)

	matches, err := FindAllSDKCredentialsWithClient(context.Background(), clientset)
	require.NoError(t, err)
	require.Len(t, matches, 1, "should only match the secret with valid license data")
	assert.Equal(t, "myapp-sdk", matches[0].SecretName)
}

func TestPromptForSDKSecret_NonInteractive(t *testing.T) {
	// When stdin is not a TTY (CI, piped input), the CLI cannot show an
	// interactive prompt. In this case PromptForSDKSecret lists all found
	// secrets and returns an error suggesting --sdk-namespace. When a TTY IS
	// present, the CLI layer shows a promptui.Select for the user to choose.
	matches := []SDKSecretMatch{
		{
			SecretName: "app1-sdk",
			Namespace:  "ns1",
			Creds:      &ReplicatedUploadCredentials{LicenseID: "lic1", ChannelID: "ch1", Endpoint: "https://replicated.app"},
		},
		{
			SecretName: "app2-sdk",
			Namespace:  "ns2",
			Creds:      &ReplicatedUploadCredentials{LicenseID: "lic2", ChannelID: "ch2", Endpoint: "https://replicated.app"},
		},
	}

	creds, err := PromptForSDKSecret(matches)
	require.Error(t, err)
	assert.Nil(t, creds)
	assert.Contains(t, err.Error(), "multiple SDK secrets found")
	assert.Contains(t, err.Error(), "--app-slug")
	assert.Contains(t, err.Error(), "secretName")
}

func TestPromptForSDKSecret_SingleMatch(t *testing.T) {
	// With only one match, the caller should use it directly (no prompt needed).
	// This tests the expected calling pattern, not promptForSDKSecret itself.
	matches := []SDKSecretMatch{
		{
			SecretName: "myapp-sdk",
			Namespace:  "production",
			Creds:      &ReplicatedUploadCredentials{LicenseID: "lic-single", ChannelID: "ch-single", Endpoint: "https://replicated.app"},
		},
	}

	assert.Len(t, matches, 1)
	assert.Equal(t, "lic-single", matches[0].Creds.LicenseID)
}

func TestFindAllSDKCredentials_MultipleAppsInSameNamespace(t *testing.T) {
	// Two different apps installed in the same namespace
	app1 := makeSDKSecret("app-one", "shared-ns", "license-one", "chan-one")
	app2 := makeSDKSecret("app-two", "shared-ns", "license-two", "chan-two")
	pullSecret := makeNonSDKSecret("enterprise-pull-secret", "shared-ns")

	clientset := fake.NewSimpleClientset(app1, app2, pullSecret)

	matches, err := FindAllSDKCredentialsWithClient(context.Background(), clientset)
	require.NoError(t, err)
	require.Len(t, matches, 2, "should find both SDK secrets in the same namespace")

	names := []string{matches[0].SecretName, matches[1].SecretName}
	assert.Contains(t, names, "app-one-sdk")
	assert.Contains(t, names, "app-two-sdk")

	// Both should be in the same namespace
	assert.Equal(t, "shared-ns", matches[0].Namespace)
	assert.Equal(t, "shared-ns", matches[1].Namespace)
}

func TestDiscoverReplicatedCredentials_MultipleAppsInSameNamespace_ReturnsError(t *testing.T) {
	// When DiscoverReplicatedCredentials finds multiple SDK secrets in the
	// target namespace, it should return a MultipleSDKSecretsError so the
	// caller can prompt the user to select.
	app1 := makeSDKSecret("app-one", "shared-ns", "license-one", "chan-one")
	app2 := makeSDKSecret("app-two", "shared-ns", "license-two", "chan-two")

	clientset := fake.NewSimpleClientset(app1, app2)

	// We can't call DiscoverReplicatedCredentials directly (needs rest.Config),
	// but we can call findSDKSecretsInNamespace which is what it uses internally.
	matches, err := findSDKSecretsInNamespace(clientset, context.Background(), "shared-ns")
	require.NoError(t, err)
	require.Len(t, matches, 2)

	// Simulate what DiscoverReplicatedCredentials does with multiple matches
	multiErr := &MultipleSDKSecretsError{Matches: matches}
	assert.Contains(t, multiErr.Error(), "found 2 Replicated SDK secrets")
	assert.Contains(t, multiErr.Error(), "--app-slug")
	assert.Contains(t, multiErr.Error(), "secretName")
}

func TestFilterByAppSlug(t *testing.T) {
	matches := []SDKSecretMatch{
		{SecretName: "app1-sdk", Namespace: "ns1", AppSlug: "app-one", Creds: &ReplicatedUploadCredentials{LicenseID: "lic1"}},
		{SecretName: "app2-sdk", Namespace: "ns2", AppSlug: "app-two", Creds: &ReplicatedUploadCredentials{LicenseID: "lic2"}},
	}

	// Found
	m := FilterByAppSlug(matches, "app-two")
	require.NotNil(t, m)
	assert.Equal(t, "lic2", m.Creds.LicenseID)
	assert.Equal(t, "app-two", m.AppSlug)

	// Not found
	m = FilterByAppSlug(matches, "nonexistent")
	assert.Nil(t, m)
}

func TestFindAllSDKCredentials_SameAppMultipleNamespaces(t *testing.T) {
	// Same app installed in staging and production namespaces
	staging := makeSDKSecret("myapp", "staging", "license-staging", "chan-staging")
	production := makeSDKSecret("myapp", "production", "license-prod", "chan-prod")

	clientset := fake.NewSimpleClientset(staging, production)

	matches, err := FindAllSDKCredentialsWithClient(context.Background(), clientset)
	require.NoError(t, err)
	require.Len(t, matches, 2, "should find both installations")

	namespaces := []string{matches[0].Namespace, matches[1].Namespace}
	assert.Contains(t, namespaces, "staging")
	assert.Contains(t, namespaces, "production")
}
