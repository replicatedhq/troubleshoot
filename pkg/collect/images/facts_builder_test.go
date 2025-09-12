package images

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewFactsBuilder(t *testing.T) {
	options := GetDefaultCollectionOptions()
	builder := NewFactsBuilder(options)

	if builder == nil {
		t.Error("NewFactsBuilder() returned nil")
	}

	if builder.collector == nil {
		t.Error("NewFactsBuilder() collector is nil")
	}

	if builder.resolver == nil {
		t.Error("NewFactsBuilder() resolver is nil")
	}
}

func TestFactsBuilder_BuildFactsFromImageStrings(t *testing.T) {
	options := GetDefaultCollectionOptions()
	options.ContinueOnError = true
	builder := NewFactsBuilder(options)

	tests := []struct {
		name      string
		imageStrs []string
		source    string
		wantCount int
		wantErr   bool
	}{
		{
			name:      "valid images",
			imageStrs: []string{"nginx:1.20", "redis:alpine"},
			source:    "test-deployment",
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "empty list",
			imageStrs: []string{},
			source:    "test-deployment",
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "mixed valid and invalid",
			imageStrs: []string{"nginx:1.20", "invalid:image:format:bad"},
			source:    "test-deployment",
			wantCount: 1, // Should continue on error
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			facts, err := builder.BuildFactsFromImageStrings(ctx, tt.imageStrs, tt.source)

			if (err != nil) != tt.wantErr {
				t.Errorf("BuildFactsFromImageStrings() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(facts) != tt.wantCount {
				t.Errorf("BuildFactsFromImageStrings() returned %d facts, want %d", len(facts), tt.wantCount)
			}

			// Verify source is set correctly
			for _, fact := range facts {
				if fact.Source != tt.source {
					t.Errorf("BuildFactsFromImageStrings() fact source = %v, want %v", fact.Source, tt.source)
				}
			}
		})
	}
}

func TestFactsBuilder_DeduplicateImageFacts(t *testing.T) {
	builder := NewFactsBuilder(GetDefaultCollectionOptions())

	tests := []struct {
		name      string
		facts     []ImageFacts
		wantCount int
	}{
		{
			name: "no duplicates",
			facts: []ImageFacts{
				{Repository: "nginx", Tag: "1.20", Registry: "docker.io"},
				{Repository: "redis", Tag: "alpine", Registry: "docker.io"},
			},
			wantCount: 2,
		},
		{
			name: "duplicate by digest",
			facts: []ImageFacts{
				{Repository: "nginx", Tag: "1.20", Digest: "sha256:abc123", Registry: "docker.io"},
				{Repository: "nginx", Tag: "latest", Digest: "sha256:abc123", Registry: "docker.io"},
			},
			wantCount: 1, // Should deduplicate by digest
		},
		{
			name: "duplicate by repo:tag",
			facts: []ImageFacts{
				{Repository: "nginx", Tag: "1.20", Registry: "docker.io"},
				{Repository: "nginx", Tag: "1.20", Registry: "docker.io"},
			},
			wantCount: 1, // Should deduplicate by repo:tag
		},
		{
			name:      "empty list",
			facts:     []ImageFacts{},
			wantCount: 0,
		},
		{
			name: "single item",
			facts: []ImageFacts{
				{Repository: "nginx", Tag: "1.20", Registry: "docker.io"},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deduplicated := builder.DeduplicateImageFacts(tt.facts)

			if len(deduplicated) != tt.wantCount {
				t.Errorf("DeduplicateImageFacts() returned %d facts, want %d", len(deduplicated), tt.wantCount)
			}
		})
	}
}

func TestFactsBuilder_ValidateImageFacts(t *testing.T) {
	builder := NewFactsBuilder(GetDefaultCollectionOptions())

	tests := []struct {
		name    string
		facts   ImageFacts
		wantErr bool
	}{
		{
			name: "valid facts",
			facts: ImageFacts{
				Repository: "nginx",
				Registry:   "docker.io",
				Tag:        "1.20",
				Platform: Platform{
					Architecture: "amd64",
					OS:           "linux",
				},
				Size:        100,
				CollectedAt: time.Now(),
			},
			wantErr: false,
		},
		{
			name: "missing repository",
			facts: ImageFacts{
				Registry: "docker.io",
				Tag:      "1.20",
				Platform: Platform{
					Architecture: "amd64",
					OS:           "linux",
				},
				Size:        100,
				CollectedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing registry",
			facts: ImageFacts{
				Repository: "nginx",
				Tag:        "1.20",
				Platform: Platform{
					Architecture: "amd64",
					OS:           "linux",
				},
				Size:        100,
				CollectedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing tag and digest",
			facts: ImageFacts{
				Repository: "nginx",
				Registry:   "docker.io",
				Platform: Platform{
					Architecture: "amd64",
					OS:           "linux",
				},
				Size:        100,
				CollectedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "invalid digest",
			facts: ImageFacts{
				Repository: "nginx",
				Registry:   "docker.io",
				Digest:     "invalid-digest",
				Platform: Platform{
					Architecture: "amd64",
					OS:           "linux",
				},
				Size:        100,
				CollectedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "missing platform architecture",
			facts: ImageFacts{
				Repository: "nginx",
				Registry:   "docker.io",
				Tag:        "1.20",
				Platform: Platform{
					OS: "linux",
				},
				Size:        100,
				CollectedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "negative size",
			facts: ImageFacts{
				Repository: "nginx",
				Registry:   "docker.io",
				Tag:        "1.20",
				Platform: Platform{
					Architecture: "amd64",
					OS:           "linux",
				},
				Size:        -1,
				CollectedAt: time.Now(),
			},
			wantErr: true,
		},
		{
			name: "zero timestamp",
			facts: ImageFacts{
				Repository: "nginx",
				Registry:   "docker.io",
				Tag:        "1.20",
				Platform: Platform{
					Architecture: "amd64",
					OS:           "linux",
				},
				Size: 100,
				// CollectedAt is zero time
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := builder.ValidateImageFacts(tt.facts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateImageFacts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFactsBuilder_SerializeFactsToJSON(t *testing.T) {
	builder := NewFactsBuilder(GetDefaultCollectionOptions())

	imageFacts := []ImageFacts{
		{
			Repository:  "nginx",
			Registry:    "docker.io",
			Tag:         "1.20",
			Size:        100 * 1024 * 1024,
			CollectedAt: time.Now(),
			Platform: Platform{
				Architecture: "amd64",
				OS:           "linux",
			},
		},
	}

	data, err := builder.SerializeFactsToJSON(imageFacts, "test-namespace")
	if err != nil {
		t.Fatalf("SerializeFactsToJSON() error = %v", err)
	}

	if len(data) == 0 {
		t.Error("SerializeFactsToJSON() returned empty data")
	}

	// Test deserialization
	bundle, err := builder.DeserializeFactsFromJSON(data)
	if err != nil {
		t.Fatalf("DeserializeFactsFromJSON() error = %v", err)
	}

	if bundle.Namespace != "test-namespace" {
		t.Errorf("Deserialized namespace = %v, want test-namespace", bundle.Namespace)
	}

	if len(bundle.ImageFacts) != 1 {
		t.Errorf("Deserialized image facts count = %v, want 1", len(bundle.ImageFacts))
	}
}

func TestFactsBuilder_GetImageFactsSummary(t *testing.T) {
	builder := NewFactsBuilder(GetDefaultCollectionOptions())

	imageFacts := []ImageFacts{
		{
			Repository: "nginx",
			Registry:   "docker.io",
			Size:       100 * 1024 * 1024,
		},
		{
			Repository: "redis",
			Registry:   "docker.io",
			Size:       50 * 1024 * 1024,
		},
		{
			Repository: "my-app",
			Registry:   "gcr.io",
			Size:       75 * 1024 * 1024,
			Error:      "collection failed",
		},
	}

	summary := builder.GetImageFactsSummary(imageFacts)

	if summary.TotalImages != 3 {
		t.Errorf("GetImageFactsSummary() total images = %v, want 3", summary.TotalImages)
	}

	if summary.UniqueRegistries != 2 {
		t.Errorf("GetImageFactsSummary() unique registries = %v, want 2", summary.UniqueRegistries)
	}

	if summary.UniqueRepositories != 3 {
		t.Errorf("GetImageFactsSummary() unique repositories = %v, want 3", summary.UniqueRepositories)
	}

	expectedSize := int64(225 * 1024 * 1024) // 225MB
	if summary.TotalSize != expectedSize {
		t.Errorf("GetImageFactsSummary() total size = %v, want %v", summary.TotalSize, expectedSize)
	}

	if summary.CollectionErrors != 1 {
		t.Errorf("GetImageFactsSummary() collection errors = %v, want 1", summary.CollectionErrors)
	}
}

func TestFactsBuilder_ExtractUniqueImages(t *testing.T) {
	builder := NewFactsBuilder(GetDefaultCollectionOptions())

	imageFacts := []ImageFacts{
		{
			Repository: "nginx",
			Registry:   "docker.io",
			Tag:        "1.20",
		},
		{
			Repository: "nginx",
			Registry:   "docker.io",
			Digest:     "sha256:abc123",
		},
		{
			Repository: "redis",
			Registry:   "docker.io",
			Tag:        "alpine",
		},
	}

	unique := builder.ExtractUniqueImages(imageFacts)

	expectedCount := 3 // All are unique
	if len(unique) != expectedCount {
		t.Errorf("ExtractUniqueImages() returned %d images, want %d", len(unique), expectedCount)
	}

	// Check that images are properly formatted
	for _, img := range unique {
		if !strings.Contains(img, "docker.io") {
			t.Errorf("ExtractUniqueImages() image %s should contain registry", img)
		}
	}
}

func TestFactsBuilder_GetLargestImages(t *testing.T) {
	builder := NewFactsBuilder(GetDefaultCollectionOptions())

	imageFacts := []ImageFacts{
		{Repository: "small", Size: 10 * 1024 * 1024},  // 10MB
		{Repository: "large", Size: 100 * 1024 * 1024}, // 100MB
		{Repository: "medium", Size: 50 * 1024 * 1024}, // 50MB
	}

	tests := []struct {
		name      string
		count     int
		wantCount int
		wantFirst string // Repository name of first (largest) image
	}{
		{
			name:      "top 2",
			count:     2,
			wantCount: 2,
			wantFirst: "large",
		},
		{
			name:      "all images",
			count:     5, // More than available
			wantCount: 3,
			wantFirst: "large",
		},
		{
			name:      "zero count",
			count:     0,
			wantCount: 0,
		},
		{
			name:      "single image",
			count:     1,
			wantCount: 1,
			wantFirst: "large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			largest := builder.GetLargestImages(imageFacts, tt.count)

			if len(largest) != tt.wantCount {
				t.Errorf("GetLargestImages() returned %d images, want %d", len(largest), tt.wantCount)
			}

			if tt.wantCount > 0 && largest[0].Repository != tt.wantFirst {
				t.Errorf("GetLargestImages() first image = %v, want %v", largest[0].Repository, tt.wantFirst)
			}

			// Verify sorted by size (largest first)
			for i := 1; i < len(largest); i++ {
				if largest[i-1].Size < largest[i].Size {
					t.Error("GetLargestImages() should return images sorted by size (largest first)")
					break
				}
			}
		})
	}
}

func TestFactsBuilder_GetImagesByRegistry(t *testing.T) {
	builder := NewFactsBuilder(GetDefaultCollectionOptions())

	imageFacts := []ImageFacts{
		{Repository: "nginx", Registry: "docker.io"},
		{Repository: "redis", Registry: "docker.io"},
		{Repository: "my-app", Registry: "gcr.io"},
		{Repository: "unknown-app", Registry: ""},
	}

	registryMap := builder.GetImagesByRegistry(imageFacts)

	// Should have 3 registry groups: docker.io, gcr.io, unknown
	if len(registryMap) != 3 {
		t.Errorf("GetImagesByRegistry() returned %d registries, want 3", len(registryMap))
	}

	// Check docker.io has 2 images
	if len(registryMap["docker.io"]) != 2 {
		t.Errorf("GetImagesByRegistry() docker.io has %d images, want 2", len(registryMap["docker.io"]))
	}

	// Check gcr.io has 1 image
	if len(registryMap["gcr.io"]) != 1 {
		t.Errorf("GetImagesByRegistry() gcr.io has %d images, want 1", len(registryMap["gcr.io"]))
	}

	// Check unknown registry
	if len(registryMap["unknown"]) != 1 {
		t.Errorf("GetImagesByRegistry() unknown registry has %d images, want 1", len(registryMap["unknown"]))
	}
}

func TestFactsBuilder_GetFailedCollections(t *testing.T) {
	builder := NewFactsBuilder(GetDefaultCollectionOptions())

	imageFacts := []ImageFacts{
		{Repository: "success1", Error: ""},
		{Repository: "failed1", Error: "network timeout"},
		{Repository: "success2", Error: ""},
		{Repository: "failed2", Error: "authentication failed"},
	}

	failed := builder.GetFailedCollections(imageFacts)
	successful := builder.GetSuccessfulCollections(imageFacts)

	if len(failed) != 2 {
		t.Errorf("GetFailedCollections() returned %d failed, want 2", len(failed))
	}

	if len(successful) != 2 {
		t.Errorf("GetSuccessfulCollections() returned %d successful, want 2", len(successful))
	}

	// Verify failed collections have errors
	for _, fact := range failed {
		if fact.Error == "" {
			t.Error("GetFailedCollections() should only return facts with errors")
		}
	}

	// Verify successful collections have no errors
	for _, fact := range successful {
		if fact.Error != "" {
			t.Error("GetSuccessfulCollections() should only return facts without errors")
		}
	}
}

func TestFactsBuilder_FilterImageFactsByRegistry(t *testing.T) {
	builder := NewFactsBuilder(GetDefaultCollectionOptions())

	imageFacts := []ImageFacts{
		{Repository: "nginx", Registry: "docker.io"},
		{Repository: "my-app", Registry: "gcr.io"},
		{Repository: "redis", Registry: "docker.io"},
		{Repository: "other-app", Registry: "quay.io"},
	}

	tests := []struct {
		name              string
		allowedRegistries []string
		wantCount         int
	}{
		{
			name:              "single registry",
			allowedRegistries: []string{"docker.io"},
			wantCount:         2,
		},
		{
			name:              "multiple registries",
			allowedRegistries: []string{"docker.io", "gcr.io"},
			wantCount:         3,
		},
		{
			name:              "no filter",
			allowedRegistries: []string{},
			wantCount:         4, // All images
		},
		{
			name:              "non-existent registry",
			allowedRegistries: []string{"non-existent.io"},
			wantCount:         0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := builder.FilterImageFactsByRegistry(imageFacts, tt.allowedRegistries)

			if len(filtered) != tt.wantCount {
				t.Errorf("FilterImageFactsByRegistry() returned %d facts, want %d", len(filtered), tt.wantCount)
			}

			// Verify all returned facts are from allowed registries
			if len(tt.allowedRegistries) > 0 {
				allowedSet := make(map[string]bool)
				for _, reg := range tt.allowedRegistries {
					allowedSet[reg] = true
				}

				for _, fact := range filtered {
					if !allowedSet[fact.Registry] {
						t.Errorf("FilterImageFactsByRegistry() returned fact from disallowed registry: %s", fact.Registry)
					}
				}
			}
		})
	}
}

func BenchmarkFactsBuilder_DeduplicateImageFacts(b *testing.B) {
	builder := NewFactsBuilder(GetDefaultCollectionOptions())

	// Create a large slice with some duplicates
	var imageFacts []ImageFacts
	for i := 0; i < 1000; i++ {
		facts := ImageFacts{
			Repository: fmt.Sprintf("app-%d", i%100), // 10% duplicates
			Registry:   "docker.io",
			Tag:        "latest",
			Digest:     fmt.Sprintf("sha256:%064d", i%100),
		}
		imageFacts = append(imageFacts, facts)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder.DeduplicateImageFacts(imageFacts)
	}
}

func BenchmarkFactsBuilder_SerializeFactsToJSON(b *testing.B) {
	builder := NewFactsBuilder(GetDefaultCollectionOptions())

	// Create test data
	var imageFacts []ImageFacts
	for i := 0; i < 100; i++ {
		facts := ImageFacts{
			Repository:  fmt.Sprintf("app-%d", i),
			Registry:    "docker.io",
			Tag:         "latest",
			Size:        int64(i * 1024 * 1024),
			CollectedAt: time.Now(),
			Platform: Platform{
				Architecture: "amd64",
				OS:           "linux",
			},
		}
		imageFacts = append(imageFacts, facts)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := builder.SerializeFactsToJSON(imageFacts, "test-namespace")
		if err != nil {
			b.Fatalf("SerializeFactsToJSON failed: %v", err)
		}
	}
}
