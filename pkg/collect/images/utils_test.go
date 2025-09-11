package images

import (
	"fmt"
	"testing"
	"time"
)

func TestComputeDigest(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{
			name: "hello world",
			data: []byte("hello world"),
			want: "sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9",
		},
		{
			name: "empty data",
			data: []byte(""),
			want: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name: "json data",
			data: []byte(`{"test": "value"}`),
			want: "sha256:71e1ec59dd990e14f06592c6146a79cbce0e1997810dd011923cc72a2ef1d1ae",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeDigest(tt.data)
			if err != nil {
				t.Errorf("ComputeDigest() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("ComputeDigest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsValidImageName(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		want      bool
	}{
		{
			name:      "valid simple name",
			imageName: "nginx",
			want:      true,
		},
		{
			name:      "valid with namespace",
			imageName: "library/nginx",
			want:      true,
		},
		{
			name:      "valid with registry",
			imageName: "gcr.io/project/app",
			want:      true,
		},
		{
			name:      "empty name",
			imageName: "",
			want:      false,
		},
		{
			name:      "with spaces",
			imageName: "nginx with spaces",
			want:      false,
		},
		{
			name:      "with tabs",
			imageName: "nginx\twith\ttabs",
			want:      false,
		},
		{
			name:      "starts with slash",
			imageName: "/nginx",
			want:      false,
		},
		{
			name:      "ends with slash",
			imageName: "nginx/",
			want:      false,
		},
		{
			name:      "double slash",
			imageName: "nginx//latest",
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidImageName(tt.imageName)
			if got != tt.want {
				t.Errorf("IsValidImageName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNormalizeRegistryURL(t *testing.T) {
	tests := []struct {
		name        string
		registryURL string
		want        string
	}{
		{
			name:        "add https",
			registryURL: "gcr.io",
			want:        "https://gcr.io",
		},
		{
			name:        "remove trailing slash",
			registryURL: "https://gcr.io/",
			want:        "https://gcr.io",
		},
		{
			name:        "docker hub special case",
			registryURL: "docker.io",
			want:        "https://registry-1.docker.io",
		},
		{
			name:        "already normalized",
			registryURL: "https://quay.io",
			want:        "https://quay.io",
		},
		{
			name:        "http preserved",
			registryURL: "http://localhost:5000",
			want:        "http://localhost:5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeRegistryURL(tt.registryURL)
			if got != tt.want {
				t.Errorf("NormalizeRegistryURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetRegistryType(t *testing.T) {
	tests := []struct {
		name        string
		registryURL string
		want        string
	}{
		{
			name:        "docker hub",
			registryURL: "https://registry-1.docker.io",
			want:        "dockerhub",
		},
		{
			name:        "google container registry",
			registryURL: "https://gcr.io",
			want:        "gcr",
		},
		{
			name:        "aws ecr",
			registryURL: "https://123456789.dkr.ecr.us-east-1.amazonaws.com",
			want:        "ecr",
		},
		{
			name:        "quay",
			registryURL: "https://quay.io",
			want:        "quay",
		},
		{
			name:        "red hat registry",
			registryURL: "https://registry.redhat.io",
			want:        "redhat",
		},
		{
			name:        "harbor",
			registryURL: "https://harbor.example.com",
			want:        "harbor",
		},
		{
			name:        "generic registry",
			registryURL: "https://my-registry.com",
			want:        "generic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRegistryType(tt.registryURL)
			if got != tt.want {
				t.Errorf("GetRegistryType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateCredentials(t *testing.T) {
	tests := []struct {
		name    string
		creds   RegistryCredentials
		wantErr bool
	}{
		{
			name:    "empty credentials",
			creds:   RegistryCredentials{},
			wantErr: false, // Valid for public registries
		},
		{
			name: "valid basic auth",
			creds: RegistryCredentials{
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
		{
			name: "valid token auth",
			creds: RegistryCredentials{
				Token: "token123",
			},
			wantErr: false,
		},
		{
			name: "valid identity token",
			creds: RegistryCredentials{
				IdentityToken: "identity123",
			},
			wantErr: false,
		},
		{
			name: "username without password",
			creds: RegistryCredentials{
				Username: "user",
			},
			wantErr: true,
		},
		{
			name: "password without username",
			creds: RegistryCredentials{
				Password: "pass",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCredentials(tt.creds)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{
			name:  "bytes",
			bytes: 512,
			want:  "512 B",
		},
		{
			name:  "kilobytes",
			bytes: 1536, // 1.5 KB
			want:  "1.5 KiB",
		},
		{
			name:  "megabytes",
			bytes: 1572864, // 1.5 MB
			want:  "1.5 MiB",
		},
		{
			name:  "gigabytes",
			bytes: 1610612736, // 1.5 GB
			want:  "1.5 GiB",
		},
		{
			name:  "zero bytes",
			bytes: 0,
			want:  "0 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("FormatSize() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOfficialImage(t *testing.T) {
	tests := []struct {
		name       string
		registry   string
		repository string
		want       bool
	}{
		{
			name:       "official docker hub image",
			registry:   "docker.io",
			repository: "library/nginx",
			want:       true,
		},
		{
			name:       "docker hub user image",
			registry:   "docker.io",
			repository: "user/nginx",
			want:       false,
		},
		{
			name:       "gcr image",
			registry:   "gcr.io",
			repository: "library/nginx",
			want:       false,
		},
		{
			name:       "default registry official",
			registry:   DefaultRegistry,
			repository: "library/redis",
			want:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsOfficialImage(tt.registry, tt.repository)
			if got != tt.want {
				t.Errorf("IsOfficialImage() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetImageShortName(t *testing.T) {
	tests := []struct {
		name  string
		facts ImageFacts
		want  string
	}{
		{
			name: "official docker hub image",
			facts: ImageFacts{
				Registry:   "docker.io",
				Repository: "library/nginx",
				Tag:        "1.20",
			},
			want: "nginx:1.20",
		},
		{
			name: "official image with latest tag",
			facts: ImageFacts{
				Registry:   DefaultRegistry,
				Repository: "library/redis",
				Tag:        DefaultTag,
			},
			want: "redis",
		},
		{
			name: "user image on docker hub",
			facts: ImageFacts{
				Registry:   "docker.io",
				Repository: "user/myapp",
				Tag:        "v1.0",
			},
			want: "user/myapp:v1.0",
		},
		{
			name: "gcr image",
			facts: ImageFacts{
				Registry:   "gcr.io",
				Repository: "project/myapp",
				Tag:        "latest",
			},
			want: "gcr.io/project/myapp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetImageShortName(tt.facts)
			if got != tt.want {
				t.Errorf("GetImageShortName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageCache(t *testing.T) {
	cache := newImageCache(100 * time.Millisecond) // Short TTL for testing

	facts := &ImageFacts{
		Repository: "nginx",
		Registry:   "docker.io",
		Tag:        "latest",
	}

	key := "docker.io/nginx:latest"

	// Test cache miss
	result := cache.Get(key)
	if result != nil {
		t.Error("Cache should initially be empty")
	}

	// Test cache set and hit
	cache.Set(key, facts)
	result = cache.Get(key)
	if result == nil {
		t.Error("Cache should contain the set key")
	}
	if result.Repository != facts.Repository {
		t.Error("Cache should return the correct facts")
	}

	// Test cache expiration
	time.Sleep(150 * time.Millisecond) // Wait for expiration
	result = cache.Get(key)
	if result != nil {
		t.Error("Cache entry should have expired")
	}

	// Test cache size
	if cache.Size() != 1 {
		t.Errorf("Cache size = %v, want 1", cache.Size())
	}

	// Test cache clear
	cache.Clear()
	if cache.Size() != 0 {
		t.Errorf("Cache should be empty after Clear(), size = %v", cache.Size())
	}
}

func TestImageCache_Cleanup(t *testing.T) {
	cache := newImageCache(50 * time.Millisecond) // Very short TTL
	cache.maxSize = 3                             // Small max size for testing

	// Add more entries than max size
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("image-%d", i)
		facts := &ImageFacts{
			Repository: fmt.Sprintf("app-%d", i),
			Registry:   "docker.io",
		}
		cache.Set(key, facts)
	}

	// Cache should have triggered cleanup
	if cache.Size() > cache.maxSize {
		t.Errorf("Cache size %d exceeds max size %d", cache.Size(), cache.maxSize)
	}

	// Wait for expiration
	time.Sleep(60 * time.Millisecond)

	// Add another entry to trigger cleanup of expired entries
	cache.Set("new-entry", &ImageFacts{Repository: "new-app"})

	// Should have fewer entries due to expiration cleanup
	if cache.Size() > 2 {
		t.Errorf("Cache should have cleaned up expired entries, size = %v", cache.Size())
	}
}

func TestGetDefaultCollectionOptions(t *testing.T) {
	options := GetDefaultCollectionOptions()

	if !options.IncludeLayers {
		t.Error("Default options should include layers")
	}

	if !options.IncludeConfig {
		t.Error("Default options should include config")
	}

	if options.Timeout != DefaultTimeout {
		t.Errorf("Default timeout = %v, want %v", options.Timeout, DefaultTimeout)
	}

	if options.MaxConcurrency != DefaultMaxConcurrency {
		t.Errorf("Default max concurrency = %v, want %v", options.MaxConcurrency, DefaultMaxConcurrency)
	}

	if !options.ContinueOnError {
		t.Error("Default options should continue on error")
	}

	if options.SkipTLSVerify {
		t.Error("Default options should not skip TLS verify")
	}

	if !options.EnableCache {
		t.Error("Default options should enable cache")
	}

	if options.CacheDuration != DefaultCacheDuration {
		t.Errorf("Default cache duration = %v, want %v", options.CacheDuration, DefaultCacheDuration)
	}

	if options.Credentials == nil {
		t.Error("Default options should have credentials map")
	}
}

func BenchmarkComputeDigest(b *testing.B) {
	data := make([]byte, 1024) // 1KB of data
	for i := range data {
		data[i] = byte(i % 256)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ComputeDigest(data)
		if err != nil {
			b.Fatalf("ComputeDigest failed: %v", err)
		}
	}
}

func BenchmarkImageCache_SetGet(b *testing.B) {
	cache := newImageCache(1 * time.Hour) // Long TTL for benchmarking

	facts := &ImageFacts{
		Repository: "nginx",
		Registry:   "docker.io",
		Tag:        "latest",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("image-%d", i%100) // Reuse some keys
		cache.Set(key, facts)
		cache.Get(key)
	}
}

func TestExtractRepositoryHost(t *testing.T) {
	tests := []struct {
		name       string
		repository string
		want       string
	}{
		{
			name:       "simple repository",
			repository: "nginx",
			want:       "nginx",
		},
		{
			name:       "namespace/repository",
			repository: "library/nginx",
			want:       "library",
		},
		{
			name:       "registry/namespace/repository",
			repository: "gcr.io/project/app",
			want:       "gcr.io",
		},
		{
			name:       "empty repository",
			repository: "",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractRepositoryHost(tt.repository)
			if got != tt.want {
				t.Errorf("ExtractRepositoryHost() = %v, want %v", got, tt.want)
			}
		})
	}
}
