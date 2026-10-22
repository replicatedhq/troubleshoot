package supportbundle

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func TestGetCollectTimeout(t *testing.T) {
	t.Run("value not set returns default", func(t *testing.T) {
		opts := SupportBundleCreateOpts{}
		actualTimeout := getCollectTimeout(opts)
		if actualTimeout != defaultTimeout {
			t.Errorf("getCollectTimeout() = %d, want default %d", actualTimeout, defaultTimeout)
		}
	})

	t.Run("value set returns configured timeout", func(t *testing.T) {
		opts := SupportBundleCreateOpts{RemoteHostCollectTimeoutSeconds: 90}
		actualTimeout := getCollectTimeout(opts)
		expectedTimeout := 90
		if actualTimeout != expectedTimeout {
			t.Errorf("getCollectTimeout() = %d, want %d", actualTimeout, expectedTimeout)
		}
	})
}

func TestBundleDirPath(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{
			name:     "absolute filename",
			filename: filepath.Join(outputDir, "support-bundle.tar.gz"),
			want:     filepath.Join(tmpDir, "support-bundle"),
		},
		{
			name:     "relative filename",
			filename: "support-bundle.tar.gz",
			want:     filepath.Join(tmpDir, "support-bundle"),
		},
		{
			name:     "filename with duplicate suffix",
			filename: filepath.Join(outputDir, "support-bundle (1).tar.gz"),
			want:     filepath.Join(tmpDir, "support-bundle (1)"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bundleDirPath(tmpDir, tt.filename)
			if got != tt.want {
				t.Errorf("bundleDirPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBundleDirPathArchiveLayout(t *testing.T) {
	tmpDir := t.TempDir()
	filename := filepath.Join(t.TempDir(), "support-bundle.tar.gz")

	bundlePath := bundleDirPath(tmpDir, filename)
	if err := os.MkdirAll(bundlePath, 0777); err != nil {
		t.Fatalf("create bundle dir: %v", err)
	}

	result := make(collect.CollectorResult)
	if err := result.SaveResult(bundlePath, "cluster-resources/pods.json", strings.NewReader("{}")); err != nil {
		t.Fatalf("save result: %v", err)
	}

	if err := result.ArchiveBundle(bundlePath, filename); err != nil {
		t.Fatalf("archive bundle: %v", err)
	}

	archiveFile, err := os.Open(filename)
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer archiveFile.Close()

	gzipReader, err := gzip.NewReader(archiveFile)
	if err != nil {
		t.Fatalf("open gzip reader: %v", err)
	}
	tarReader := tar.NewReader(gzipReader)

	var names []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read archive: %v", err)
		}
		names = append(names, header.Name)
	}

	want := "support-bundle/cluster-resources/pods.json"
	if len(names) != 1 || names[0] != want {
		t.Errorf("archive entries = %v, want [%q]", names, want)
	}
}
