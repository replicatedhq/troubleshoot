package images

import (
	"testing"
)

func TestNewDigestResolver(t *testing.T) {
	options := GetDefaultCollectionOptions()
	resolver := NewDigestResolver(options)
	
	if resolver == nil {
		t.Error("NewDigestResolver() returned nil")
	}
	
	if resolver.registryFactory == nil {
		t.Error("NewDigestResolver() registryFactory is nil")
	}
}

func TestDigestResolver_ValidateDigest(t *testing.T) {
	resolver := NewDigestResolver(GetDefaultCollectionOptions())

	tests := []struct {
		name    string
		digest  string
		wantErr bool
	}{
		{
			name:    "valid sha256",
			digest:  "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantErr: false,
		},
		{
			name:    "valid sha512",
			digest:  "sha512:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
			wantErr: false,
		},
		{
			name:    "empty digest",
			digest:  "",
			wantErr: true,
		},
		{
			name:    "missing algorithm",
			digest:  "1234567890abcdef",
			wantErr: true,
		},
		{
			name:    "invalid algorithm",
			digest:  "md5:1234567890abcdef",
			wantErr: true,
		},
		{
			name:    "wrong length for sha256",
			digest:  "sha256:123456",
			wantErr: true,
		},
		{
			name:    "invalid hex characters",
			digest:  "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdefg",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := resolver.ValidateDigest(tt.digest)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDigest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDigestResolver_NormalizeImageReference(t *testing.T) {
	resolver := NewDigestResolver(GetDefaultCollectionOptions())

	tests := []struct {
		name string
		ref  ImageReference
		want ImageReference
	}{
		{
			name: "empty registry gets default",
			ref: ImageReference{
				Repository: "nginx",
				Tag:        "latest",
			},
			want: ImageReference{
				Registry:   DefaultRegistry,
				Repository: "library/nginx", // Docker Hub library namespace
				Tag:        "latest",
			},
		},
		{
			name: "no tag gets default",
			ref: ImageReference{
				Registry:   "gcr.io",
				Repository: "project/app",
			},
			want: ImageReference{
				Registry:   "gcr.io",
				Repository: "project/app",
				Tag:        DefaultTag,
			},
		},
		{
			name: "digest takes precedence over tag",
			ref: ImageReference{
				Registry:   "gcr.io",
				Repository: "project/app",
				Tag:        "v1.0",
				Digest:     "sha256:abc123",
			},
			want: ImageReference{
				Registry:   "gcr.io",
				Repository: "project/app",
				Tag:        "v1.0",
				Digest:     "sha256:abc123",
			},
		},
		{
			name: "already normalized",
			ref: ImageReference{
				Registry:   "gcr.io",
				Repository: "project/app",
				Tag:        "v1.0",
			},
			want: ImageReference{
				Registry:   "gcr.io",
				Repository: "project/app",
				Tag:        "v1.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolver.NormalizeImageReference(tt.ref)
			
			if got.Registry != tt.want.Registry {
				t.Errorf("NormalizeImageReference() registry = %v, want %v", got.Registry, tt.want.Registry)
			}
			if got.Repository != tt.want.Repository {
				t.Errorf("NormalizeImageReference() repository = %v, want %v", got.Repository, tt.want.Repository)
			}
			if got.Tag != tt.want.Tag {
				t.Errorf("NormalizeImageReference() tag = %v, want %v", got.Tag, tt.want.Tag)
			}
			if got.Digest != tt.want.Digest {
				t.Errorf("NormalizeImageReference() digest = %v, want %v", got.Digest, tt.want.Digest)
			}
		})
	}
}

func TestDigestResolver_getCredentialsForRegistry(t *testing.T) {
	options := GetDefaultCollectionOptions()
	options.Credentials = map[string]RegistryCredentials{
		"gcr.io": {
			Username: "gcr-user",
			Password: "gcr-pass",
		},
		"https://registry-1.docker.io": {
			Username: "docker-user", 
			Password: "docker-pass",
		},
	}

	resolver := NewDigestResolver(options)

	tests := []struct {
		name     string
		registry string
		want     string // username to check
	}{
		{
			name:     "exact match",
			registry: "gcr.io",
			want:     "gcr-user",
		},
		{
			name:     "docker hub with URL",
			registry: "https://registry-1.docker.io",
			want:     "docker-user",
		},
		{
			name:     "no match",
			registry: "quay.io",
			want:     "",
		},
		{
			name:     "partial match",
			registry: "https://gcr.io",
			want:     "gcr-user", // Should match gcr.io
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			creds := resolver.getCredentialsForRegistry(tt.registry)
			if creds.Username != tt.want {
				t.Errorf("getCredentialsForRegistry() username = %v, want %v", creds.Username, tt.want)
			}
		})
	}
}

func TestExtractDigestFromManifestResponse(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string][]string
		want    string
	}{
		{
			name: "docker content digest",
			headers: map[string][]string{
				"Docker-Content-Digest": {"sha256:abcdef123456"},
			},
			want: "sha256:abcdef123456",
		},
		{
			name: "content digest fallback",
			headers: map[string][]string{
				"Content-Digest": {"sha256:fallback123"},
			},
			want: "sha256:fallback123",
		},
		{
			name:    "no digest header",
			headers: map[string][]string{},
			want:    "",
		},
		{
			name: "docker digest takes precedence",
			headers: map[string][]string{
				"Docker-Content-Digest": {"sha256:docker123"},
				"Content-Digest":        {"sha256:fallback123"},
			},
			want: "sha256:docker123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractDigestFromManifestResponse(tt.headers)
			if got != tt.want {
				t.Errorf("ExtractDigestFromManifestResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkDigestResolver_ValidateDigest(b *testing.B) {
	resolver := NewDigestResolver(GetDefaultCollectionOptions())
	digest := "sha256:1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := resolver.ValidateDigest(digest)
		if err != nil {
			b.Fatalf("ValidateDigest failed: %v", err)
		}
	}
}

func BenchmarkDigestResolver_NormalizeImageReference(b *testing.B) {
	resolver := NewDigestResolver(GetDefaultCollectionOptions())
	
	refs := []ImageReference{
		{Repository: "nginx"},
		{Registry: "gcr.io", Repository: "project/app", Tag: "v1.0"},
		{Repository: "library/redis", Tag: "alpine"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, ref := range refs {
			resolver.NormalizeImageReference(ref)
		}
	}
}
