package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestValidateBundleFile(t *testing.T) {
	// Create temporary test files
	tempDir := t.TempDir()

	// Create valid bundle files
	validBundle := filepath.Join(tempDir, "test-bundle.tar.gz")
	if err := os.WriteFile(validBundle, []byte("dummy content"), 0644); err != nil {
		t.Fatalf("Failed to create test bundle: %v", err)
	}

	validTgzBundle := filepath.Join(tempDir, "test-bundle.tgz")
	if err := os.WriteFile(validTgzBundle, []byte("dummy content"), 0644); err != nil {
		t.Fatalf("Failed to create test tgz bundle: %v", err)
	}

	tests := []struct {
		name       string
		bundlePath string
		wantErr    bool
	}{
		{
			name:       "valid tar.gz bundle",
			bundlePath: validBundle,
			wantErr:    false,
		},
		{
			name:       "valid tgz bundle",
			bundlePath: validTgzBundle,
			wantErr:    false,
		},
		{
			name:       "empty path",
			bundlePath: "",
			wantErr:    true,
		},
		{
			name:       "non-existent file",
			bundlePath: "/path/to/nonexistent.tar.gz",
			wantErr:    true,
		},
		{
			name:       "invalid extension",
			bundlePath: filepath.Join(tempDir, "invalid.txt"),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For invalid extension test, create the file
			if strings.HasSuffix(tt.bundlePath, "invalid.txt") {
				os.WriteFile(tt.bundlePath, []byte("content"), 0644)
			}

			err := validateBundleFile(tt.bundlePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBundleFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetBundleMetadata(t *testing.T) {
	// Create a temporary test file
	tempDir := t.TempDir()
	testBundle := filepath.Join(tempDir, "test-bundle.tar.gz")
	testContent := []byte("test bundle content")
	
	if err := os.WriteFile(testBundle, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test bundle: %v", err)
	}

	metadata := getBundleMetadata(testBundle)

	if metadata.Path != testBundle {
		t.Errorf("getBundleMetadata() path = %v, want %v", metadata.Path, testBundle)
	}

	if metadata.Size != int64(len(testContent)) {
		t.Errorf("getBundleMetadata() size = %v, want %v", metadata.Size, len(testContent))
	}

	if metadata.CreatedAt == "" {
		t.Error("getBundleMetadata() createdAt should not be empty")
	}

	// Validate timestamp format
	if _, err := time.Parse(time.RFC3339, metadata.CreatedAt); err != nil {
		t.Errorf("getBundleMetadata() createdAt has invalid format: %v", err)
	}
}

func TestPerformBundleDiff(t *testing.T) {
	// Create temporary test bundles
	tempDir := t.TempDir()
	
	oldBundle := filepath.Join(tempDir, "old-bundle.tar.gz")
	newBundle := filepath.Join(tempDir, "new-bundle.tar.gz")
	
	if err := os.WriteFile(oldBundle, []byte("old content"), 0644); err != nil {
		t.Fatalf("Failed to create old bundle: %v", err)
	}
	
	if err := os.WriteFile(newBundle, []byte("new content"), 0644); err != nil {
		t.Fatalf("Failed to create new bundle: %v", err)
	}

	v := viper.New()
	result, err := performBundleDiff(oldBundle, newBundle, v)
	
	if err != nil {
		t.Fatalf("performBundleDiff() error = %v", err)
	}

	if result == nil {
		t.Fatal("performBundleDiff() returned nil result")
	}

	// Verify result structure
	if result.Metadata.Version != "v1" {
		t.Errorf("performBundleDiff() version = %v, want v1", result.Metadata.Version)
	}

	if result.Metadata.OldBundle.Path != oldBundle {
		t.Errorf("performBundleDiff() old bundle path = %v, want %v", result.Metadata.OldBundle.Path, oldBundle)
	}

	if result.Metadata.NewBundle.Path != newBundle {
		t.Errorf("performBundleDiff() new bundle path = %v, want %v", result.Metadata.NewBundle.Path, newBundle)
	}

	if result.Metadata.GeneratedAt == "" {
		t.Error("performBundleDiff() generatedAt should not be empty")
	}
}

func TestGenerateTextDiffReport(t *testing.T) {
	result := &DiffResult{
		Summary: DiffSummary{
			TotalChanges:      3,
			FilesAdded:        1,
			FilesRemoved:      1,
			FilesModified:     1,
			HighImpactChanges: 1,
		},
		Changes: []Change{
			{
				Type:     "added",
				Category: "config",
				Path:     "/new-config.yaml",
				Impact:   "medium",
				Remediation: &RemediationStep{
					Description: "Review new configuration",
				},
			},
			{
				Type:     "removed",
				Category: "resource",
				Path:     "/old-deployment.yaml",
				Impact:   "high",
			},
		},
		Metadata: DiffMetadata{
			OldBundle: BundleMetadata{
				Path: "/old/bundle.tar.gz",
				Size: 1024,
			},
			NewBundle: BundleMetadata{
				Path: "/new/bundle.tar.gz", 
				Size: 2048,
			},
			GeneratedAt: "2023-01-01T00:00:00Z",
		},
		Significance: "high",
	}

	report := generateTextDiffReport(result)

	// Check that report contains expected elements
	expectedStrings := []string{
		"Support Bundle Diff Report",
		"Total Changes: 3",
		"Files Added: 1",
		"High Impact Changes: 1",
		"Significance: high",
		"/new-config.yaml",
		"/old-deployment.yaml",
		"Remediation: Review new configuration",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(report, expected) {
			t.Errorf("generateTextDiffReport() missing expected string: %s", expected)
		}
	}
}

func TestGenerateHTMLDiffReport(t *testing.T) {
	result := &DiffResult{
		Summary: DiffSummary{
			TotalChanges: 2,
		},
		Changes: []Change{
			{
				Type:     "added",
				Path:     "/new-file.yaml",
				Impact:   "low",
			},
		},
		Metadata: DiffMetadata{
			GeneratedAt: "2023-01-01T00:00:00Z",
		},
		Significance: "low",
	}

	html := generateHTMLDiffReport(result)

	// Check that HTML contains expected elements
	expectedStrings := []string{
		"<!DOCTYPE html>",
		"Support Bundle Diff Report",
		"Total Changes:",
		"/new-file.yaml",
		"class=\"change added impact-low\"",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(html, expected) {
			t.Errorf("generateHTMLDiffReport() missing expected string: %s", expected)
		}
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
			name:  "zero",
			bytes: 0,
			want:  "0 B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSize(tt.bytes)
			if got != tt.want {
				t.Errorf("formatSize() = %v, want %v", got, tt.want)
			}
		})
	}
}
