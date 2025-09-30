package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
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

	// Create bundle files
	validBundle := filepath.Join(tempDir, "test-bundle.tar.gz")
	if err := os.WriteFile(validBundle, []byte("dummy content"), 0644); err != nil {
		t.Fatalf("Failed to create test bundle: %v", err)
	}

	validTgz := filepath.Join(tempDir, "test-bundle.tgz")
	if err := os.WriteFile(validTgz, []byte("dummy content"), 0644); err != nil {
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
			bundlePath: validTgz,
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

	if err := writeTarGz(oldBundle, map[string]string{
		"root/cluster-resources/version.txt": "v1\n",
		"root/logs/app.log":                  "line1\n",
	}); err != nil {
		t.Fatalf("Failed to create old bundle: %v", err)
	}

	if err := writeTarGz(newBundle, map[string]string{
		"root/cluster-resources/version.txt": "v2\n",
		"root/logs/app.log":                  "line1\nline2\n",
		"root/added.txt":                     "new\n",
	}); err != nil {
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

// writeTarGz creates a gzipped tar file at tarPath with the provided files map.
// Keys are entry names inside the archive, values are file contents.
func writeTarGz(tarPath string, files map[string]string) error {
	f, err := os.Create(tarPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	for name, content := range files {
		data := []byte(content)
		hdr := &tar.Header{
			Name:     name,
			Mode:     0644,
			Size:     int64(len(data)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if _, err := bytes.NewReader(data).WriteTo(tw); err != nil {
			return err
		}
	}
	return nil
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

	report := generateTextDiffReport(result, true)

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

func TestOutputDiffResult_JSON(t *testing.T) {
	// Minimal result
	result := &DiffResult{
		Summary: DiffSummary{},
		Metadata: DiffMetadata{
			OldBundle:   BundleMetadata{Path: "/old.tar.gz"},
			NewBundle:   BundleMetadata{Path: "/new.tar.gz"},
			GeneratedAt: time.Now().Format(time.RFC3339),
			Version:     "v1",
		},
		Changes:      []Change{{Type: "modified", Category: "files", Path: "/a", Impact: "low"}},
		Significance: "low",
	}

	v := viper.New()
	v.Set("format", "json")

	// Write to a temp file via -o to exercise file write path
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "diff.json")
	v.Set("output", outPath)

	if err := outputDiffResult(result, v); err != nil {
		t.Fatalf("outputDiffResult(json) error = %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	// Basic JSON sanity checks
	s := string(data)
	if !strings.Contains(s, "\"summary\"") || !strings.Contains(s, "\"changes\"") {
		t.Fatalf("json output missing keys: %s", s)
	}
	if !strings.Contains(s, "\"path\": \"/a\"") {
		t.Fatalf("json output missing change path: %s", s)
	}
}

func TestGenerateTextDiffReport_DiffVisibility(t *testing.T) {
	result := &DiffResult{
		Summary: DiffSummary{TotalChanges: 1, FilesModified: 1},
		Changes: []Change{
			{
				Type:     "modified",
				Category: "files",
				Path:     "/path.txt",
				Impact:   "low",
				Details:  map[string]interface{}{"diff": "--- old:/path.txt\n+++ new:/path.txt\n@@\n-a\n+b"},
			},
		},
		Metadata: DiffMetadata{GeneratedAt: time.Now().Format(time.RFC3339)},
	}

	reportShown := generateTextDiffReport(result, true)
	if !strings.Contains(reportShown, "Diff:") || !strings.Contains(reportShown, "+ new:/path.txt") {
		t.Fatalf("expected diff to be shown when enabled; got: %s", reportShown)
	}

	reportHidden := generateTextDiffReport(result, false)
	if strings.Contains(reportHidden, "Diff:") {
		t.Fatalf("expected diff to be hidden when disabled; got: %s", reportHidden)
	}
}

func TestCategorizePath(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{"cluster-resources/pods/logs/ns/pod/container.log", "logs"},
		{"some/ns/logs/thing.log", "logs"},
		{"all-logs/ns/pod/container.log", "logs"},
		{"logs/app.log", "logs"},
		{"cluster-resources/configmaps/ns.json", "resources:configmaps"},
		{"cluster-resources/", "resources"},
		{"config/settings.yaml", "config"},
		{"random/file.json", "config"},
		{"random/file.txt", "files"},
	}
	for _, c := range cases {
		if got := categorizePath(c.in); got != c.out {
			t.Errorf("categorizePath(%q)=%q want %q", c.in, got, c.out)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	cases := []struct {
		in  string
		out string
	}{
		{"root/foo.txt", "foo.txt"},
		{"support-bundle-123/foo.txt", "foo.txt"},
		{"Support-Bundle-ABC/bar/baz.txt", "bar/baz.txt"},
		{"cluster-resources/pods/logs/whatever.log", "cluster-resources/pods/logs/whatever.log"},
		{"all-logs/whatever.log", "all-logs/whatever.log"},
	}
	for _, c := range cases {
		if got := normalizePath(c.in); got != c.out {
			t.Errorf("normalizePath(%q)=%q want %q", c.in, got, c.out)
		}
	}
}

func TestGenerateUnifiedDiff_TruncationAndContext(t *testing.T) {
	old := "line1\nline2\nline3\nline4\nline5\n"
	newv := "line1\nline2-mod\nline3\nline4\nline5\n"
	// context=1 should include headers and minimal context; max lines small to force truncation
	diff := generateUnifiedDiff(old, newv, "/file.txt", 1, 5)
	if diff == "" {
		t.Fatal("expected non-empty diff")
	}
	if !strings.Contains(diff, "old:/file.txt") || !strings.Contains(diff, "new:/file.txt") {
		t.Errorf("diff missing headers: %s", diff)
	}
	if !strings.Contains(diff, "... (diff truncated)") {
		t.Errorf("expected truncated marker in diff: %s", diff)
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

func TestCreateTarFileReader(t *testing.T) {
	tempDir := t.TempDir()
	bundlePath := filepath.Join(tempDir, "test-bundle.tar.gz")

	// Create test bundle with text and binary files
	testFiles := map[string]string{
		"root/text-file.txt":   "line1\nline2\nline3\n",
		"root/config.yaml":     "key: value\nother: data\n",
		"root/binary-file.bin": string([]byte{0x00, 0x01, 0x02, 0xFF}), // Binary content
	}

	if err := writeTarGz(bundlePath, testFiles); err != nil {
		t.Fatalf("Failed to create test bundle: %v", err)
	}

	// Test reading existing text file
	reader, err := createTarFileReader(bundlePath, "text-file.txt")
	if err != nil {
		t.Fatalf("createTarFileReader() error = %v", err)
	}
	defer reader.Close()

	content := make([]byte, 100)
	n, err := reader.Read(content)
	if err != nil && err != io.EOF {
		t.Errorf("Read() error = %v", err)
	}

	contentStr := string(content[:n])
	if !strings.Contains(contentStr, "line1") {
		t.Errorf("Expected file content not found, got: %s", contentStr)
	}

	// Test reading non-existent file
	_, err = createTarFileReader(bundlePath, "non-existent.txt")
	if err == nil {
		t.Error("Expected error for non-existent file")
	}

	// Test reading binary file (should fail)
	_, err = createTarFileReader(bundlePath, "binary-file.bin")
	if err == nil {
		t.Error("Expected error for binary file")
	}
}

func TestGenerateStreamingUnifiedDiff(t *testing.T) {
	tempDir := t.TempDir()
	oldBundle := filepath.Join(tempDir, "old-bundle.tar.gz")
	newBundle := filepath.Join(tempDir, "new-bundle.tar.gz")

	// Create bundles with different versions of the same file
	if err := writeTarGz(oldBundle, map[string]string{
		"root/config.yaml": "version: 1.0\napp: test\nfeature: disabled\n",
	}); err != nil {
		t.Fatalf("Failed to create old bundle: %v", err)
	}

	if err := writeTarGz(newBundle, map[string]string{
		"root/config.yaml": "version: 2.0\napp: test\nfeature: enabled\n",
	}); err != nil {
		t.Fatalf("Failed to create new bundle: %v", err)
	}

	// Generate streaming diff
	diff := generateStreamingUnifiedDiff(oldBundle, newBundle, "/config.yaml", 3, 100)

	// Verify diff content
	if diff == "" {
		t.Error("Expected non-empty diff")
	}

	expectedStrings := []string{
		"old:/config.yaml",
		"new:/config.yaml",
		"-version: 1.0",
		"+version: 2.0",
		"-feature: disabled",
		"+feature: enabled",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(diff, expected) {
			t.Errorf("Diff missing expected string '%s'. Got: %s", expected, diff)
		}
	}
}

func TestReadLinesFromReader(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		maxBytes int
		wantLen  int
		wantLast string
	}{
		{
			name:     "small content",
			content:  "line1\nline2\nline3\n",
			maxBytes: 1000,
			wantLen:  3,
			wantLast: "line3\n",
		},
		{
			name:     "content exceeds limit",
			content:  "line1\nline2\nline3\nline4\nline5\n",
			maxBytes: 15, // Only allows first 2 lines plus truncation marker
			wantLen:  3,
			wantLast: "... (content truncated due to size)\n",
		},
		{
			name:     "empty content",
			content:  "",
			maxBytes: 1000,
			wantLen:  0,
			wantLast: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.content)
			lines, err := readLinesFromReader(reader, tt.maxBytes)

			if err != nil {
				t.Errorf("readLinesFromReader() error = %v", err)
			}

			if len(lines) != tt.wantLen {
				t.Errorf("readLinesFromReader() got %d lines, want %d", len(lines), tt.wantLen)
			}

			if tt.wantLen > 0 && lines[len(lines)-1] != tt.wantLast {
				t.Errorf("readLinesFromReader() last line = %s, want %s", lines[len(lines)-1], tt.wantLast)
			}
		})
	}
}
