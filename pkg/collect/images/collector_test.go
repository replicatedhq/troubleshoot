package images

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewImageCollector(t *testing.T) {
	tests := []struct {
		name    string
		options CollectionOptions
		wantErr bool
	}{
		{
			name:    "default options",
			options: GetDefaultCollectionOptions(),
			wantErr: false,
		},
		{
			name: "custom options",
			options: CollectionOptions{
				IncludeLayers:   true,
				IncludeConfig:   true,
				Timeout:         10 * time.Second,
				MaxConcurrency:  3,
				ContinueOnError: true,
				EnableCache:     false,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewImageCollector(tt.options)
			if collector == nil {
				t.Error("NewImageCollector() returned nil")
			}
		})
	}
}

func TestParseImageReference(t *testing.T) {
	tests := []struct {
		name     string
		imageStr string
		want     ImageReference
		wantErr  bool
	}{
		{
			name:     "simple image",
			imageStr: "nginx",
			want: ImageReference{
				Registry:   DefaultRegistry,
				Repository: "library/nginx", // Docker Hub library namespace
				Tag:        DefaultTag,
			},
			wantErr: false,
		},
		{
			name:     "image with tag",
			imageStr: "nginx:1.20",
			want: ImageReference{
				Registry:   DefaultRegistry,
				Repository: "library/nginx",
				Tag:        "1.20",
			},
			wantErr: false,
		},
		{
			name:     "image with registry",
			imageStr: "gcr.io/my-project/my-app:v1.0",
			want: ImageReference{
				Registry:   "gcr.io",
				Repository: "my-project/my-app",
				Tag:        "v1.0",
			},
			wantErr: false,
		},
		{
			name:     "image with digest",
			imageStr: "nginx@sha256:abcdef123456",
			want: ImageReference{
				Registry:   DefaultRegistry,
				Repository: "library/nginx",
				Digest:     "sha256:abcdef123456",
			},
			wantErr: false,
		},
		{
			name:     "full reference with registry and digest",
			imageStr: "gcr.io/my-project/my-app@sha256:abcdef123456",
			want: ImageReference{
				Registry:   "gcr.io",
				Repository: "my-project/my-app",
				Digest:     "sha256:abcdef123456",
			},
			wantErr: false,
		},
		{
			name:     "registry with port",
			imageStr: "localhost:5000/my-app:latest",
			want: ImageReference{
				Registry:   "localhost:5000",
				Repository: "my-app",
				Tag:        "latest",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseImageReference(tt.imageStr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseImageReference() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Registry != tt.want.Registry {
					t.Errorf("ParseImageReference() registry = %v, want %v", got.Registry, tt.want.Registry)
				}
				if got.Repository != tt.want.Repository {
					t.Errorf("ParseImageReference() repository = %v, want %v", got.Repository, tt.want.Repository)
				}
				if got.Tag != tt.want.Tag && got.Digest == "" {
					t.Errorf("ParseImageReference() tag = %v, want %v", got.Tag, tt.want.Tag)
				}
				if got.Digest != tt.want.Digest {
					t.Errorf("ParseImageReference() digest = %v, want %v", got.Digest, tt.want.Digest)
				}
			}
		})
	}
}

func TestImageReference_String(t *testing.T) {
	tests := []struct {
		name string
		ref  ImageReference
		want string
	}{
		{
			name: "with tag",
			ref: ImageReference{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "1.20",
			},
			want: "docker.io/library/nginx:1.20",
		},
		{
			name: "with digest",
			ref: ImageReference{
				Registry:   "gcr.io",
				Repository: "my-project/my-app",
				Digest:     "sha256:abcdef123456",
			},
			want: "gcr.io/my-project/my-app@sha256:abcdef123456",
		},
		{
			name: "no tag or digest",
			ref: ImageReference{
				Registry:   "docker.io",
				Repository: "library/nginx",
			},
			want: "docker.io/library/nginx:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ref.String()
			if got != tt.want {
				t.Errorf("ImageReference.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateFactsBundle(t *testing.T) {
	imageFacts := []ImageFacts{
		{
			Repository: "library/nginx",
			Tag:        "1.20",
			Registry:   "docker.io",
			Size:       100 * 1024 * 1024, // 100MB
			Error:      "",
		},
		{
			Repository: "my-project/my-app",
			Tag:        "v1.0",
			Registry:   "gcr.io",
			Size:       50 * 1024 * 1024, // 50MB
			Error:      "collection failed",
		},
	}

	bundle := CreateFactsBundle("test-namespace", imageFacts)

	if bundle.Version != "v1" {
		t.Errorf("CreateFactsBundle() version = %v, want v1", bundle.Version)
	}

	if bundle.Namespace != "test-namespace" {
		t.Errorf("CreateFactsBundle() namespace = %v, want test-namespace", bundle.Namespace)
	}

	if len(bundle.ImageFacts) != 2 {
		t.Errorf("CreateFactsBundle() image facts count = %v, want 2", len(bundle.ImageFacts))
	}

	// Check summary
	if bundle.Summary.TotalImages != 2 {
		t.Errorf("CreateFactsBundle() total images = %v, want 2", bundle.Summary.TotalImages)
	}

	if bundle.Summary.UniqueRegistries != 2 {
		t.Errorf("CreateFactsBundle() unique registries = %v, want 2", bundle.Summary.UniqueRegistries)
	}

	expectedSize := int64(150 * 1024 * 1024) // 150MB
	if bundle.Summary.TotalSize != expectedSize {
		t.Errorf("CreateFactsBundle() total size = %v, want %v", bundle.Summary.TotalSize, expectedSize)
	}

	if bundle.Summary.CollectionErrors != 1 {
		t.Errorf("CreateFactsBundle() collection errors = %v, want 1", bundle.Summary.CollectionErrors)
	}
}

func TestDefaultImageCollector_SetCredentials(t *testing.T) {
	collector := NewImageCollector(GetDefaultCollectionOptions())

	creds := RegistryCredentials{
		Username: "testuser",
		Password: "testpass",
	}

	err := collector.SetCredentials("gcr.io", creds)
	if err != nil {
		t.Errorf("SetCredentials() error = %v", err)
	}

	// Verify credentials were stored
	storedCreds := collector.options.Credentials["gcr.io"]
	if storedCreds.Username != creds.Username {
		t.Errorf("Stored username = %v, want %v", storedCreds.Username, creds.Username)
	}
}

// Mock registry server for testing
func setupMockRegistry() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/":
			// Ping endpoint
			w.WriteHeader(http.StatusOK)

		case strings.Contains(r.URL.Path, "/manifests/"):
			// Manifest endpoint
			w.Header().Set("Content-Type", DockerManifestSchema2)
			w.Header().Set("Docker-Content-Digest", "sha256:1234567890abcdef")

			// Return a minimal v2 manifest
			manifest := `{
				"schemaVersion": 2,
				"mediaType": "application/vnd.docker.distribution.manifest.v2+json",
				"config": {
					"mediaType": "application/vnd.docker.container.image.v1+json",
					"size": 1469,
					"digest": "sha256:config123"
				},
				"layers": [
					{
						"mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
						"size": 977,
						"digest": "sha256:layer123"
					}
				]
			}`
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(manifest))

		case strings.Contains(r.URL.Path, "/blobs/"):
			// Blob endpoint (for config)
			if strings.Contains(r.URL.Path, "sha256:config123") {
				config := `{
					"architecture": "amd64",
					"os": "linux",
					"config": {
						"Env": ["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],
						"Cmd": ["nginx", "-g", "daemon off;"]
					},
					"created": "2021-01-01T00:00:00Z"
				}`
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(config))
			} else {
				w.WriteHeader(http.StatusNotFound)
			}

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func TestDefaultImageCollector_CollectImageFacts_Integration(t *testing.T) {
	// Setup mock registry
	server := setupMockRegistry()
	defer server.Close()

	options := GetDefaultCollectionOptions()
	options.Timeout = 5 * time.Second
	options.ContinueOnError = true

	collector := NewImageCollector(options)

	imageRef := ImageReference{
		Registry:   server.URL,
		Repository: "test/nginx",
		Tag:        "latest",
	}

	ctx := context.Background()
	facts, err := collector.CollectImageFacts(ctx, imageRef)

	if err != nil {
		t.Fatalf("CollectImageFacts() error = %v", err)
	}

	if facts == nil {
		t.Fatal("CollectImageFacts() returned nil facts")
	}

	// Verify basic facts
	if facts.Repository != imageRef.Repository {
		t.Errorf("CollectImageFacts() repository = %v, want %v", facts.Repository, imageRef.Repository)
	}

	if facts.Registry != imageRef.Registry {
		t.Errorf("CollectImageFacts() registry = %v, want %v", facts.Registry, imageRef.Registry)
	}

	if facts.MediaType != DockerManifestSchema2 {
		t.Errorf("CollectImageFacts() mediaType = %v, want %v", facts.MediaType, DockerManifestSchema2)
	}

	if facts.SchemaVersion != 2 {
		t.Errorf("CollectImageFacts() schemaVersion = %v, want 2", facts.SchemaVersion)
	}
}

func TestDefaultImageCollector_CollectMultipleImageFacts(t *testing.T) {
	// Setup mock registry
	server := setupMockRegistry()
	defer server.Close()

	options := GetDefaultCollectionOptions()
	options.MaxConcurrency = 2
	options.ContinueOnError = true

	collector := NewImageCollector(options)

	imageRefs := []ImageReference{
		{
			Registry:   server.URL,
			Repository: "test/nginx",
			Tag:        "latest",
		},
		{
			Registry:   server.URL,
			Repository: "test/apache",
			Tag:        "2.4",
		},
	}

	ctx := context.Background()
	factsList, err := collector.CollectMultipleImageFacts(ctx, imageRefs)

	if err != nil {
		t.Fatalf("CollectMultipleImageFacts() error = %v", err)
	}

	if len(factsList) != 2 {
		t.Errorf("CollectMultipleImageFacts() returned %d facts, want 2", len(factsList))
	}

	// Verify each result
	for i, facts := range factsList {
		if facts.Repository != imageRefs[i].Repository {
			t.Errorf("Facts[%d] repository = %v, want %v", i, facts.Repository, imageRefs[i].Repository)
		}
	}
}

func BenchmarkParseImageReference(b *testing.B) {
	testImages := []string{
		"nginx",
		"nginx:1.20",
		"gcr.io/my-project/my-app:v1.0",
		"nginx@sha256:abcdef123456",
		"localhost:5000/my-app:latest",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, img := range testImages {
			_, err := ParseImageReference(img)
			if err != nil {
				b.Fatalf("ParseImageReference failed: %v", err)
			}
		}
	}
}

func BenchmarkDefaultImageCollector_CollectImageFacts(b *testing.B) {
	// Setup mock registry
	server := setupMockRegistry()
	defer server.Close()

	options := GetDefaultCollectionOptions()
	options.EnableCache = false // Disable cache for benchmarking

	collector := NewImageCollector(options)

	imageRef := ImageReference{
		Registry:   server.URL,
		Repository: "test/nginx",
		Tag:        "latest",
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := collector.CollectImageFacts(ctx, imageRef)
		if err != nil {
			b.Fatalf("CollectImageFacts failed: %v", err)
		}
	}
}

func TestDefaultImageCollector_getCredentialsForRegistry(t *testing.T) {
	options := GetDefaultCollectionOptions()
	options.Credentials = map[string]RegistryCredentials{
		"gcr.io": {
			Username: "gcr-user",
			Password: "gcr-pass",
		},
		"docker.io": {
			Username: "docker-user",
			Password: "docker-pass",
		},
	}

	collector := NewImageCollector(options)

	tests := []struct {
		name     string
		registry string
		want     string // username to verify
	}{
		{
			name:     "exact match",
			registry: "gcr.io",
			want:     "gcr-user",
		},
		{
			name:     "docker hub",
			registry: "docker.io",
			want:     "docker-user",
		},
		{
			name:     "no credentials",
			registry: "unknown-registry.com",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := collector.getCredentialsForRegistry(tt.registry)
			if creds.Username != tt.want {
				t.Errorf("getCredentialsForRegistry() username = %v, want %v", creds.Username, tt.want)
			}
		})
	}
}
