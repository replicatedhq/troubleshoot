package supportbundle

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
