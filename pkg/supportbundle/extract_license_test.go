package supportbundle

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// makeBundleTarGz creates a temporary .tar.gz file containing the given entries.
// Each entry is a (path, content) pair. Returns the file path; caller must remove it.
func makeBundleTarGz(t *testing.T, entries []struct{ name, content string }) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "bundle-*.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	for _, e := range entries {
		hdr := &tar.Header{
			Name:     e.name,
			Typeflag: tar.TypeReg,
			Size:     int64(len(e.content)),
			Mode:     0644,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(e.content)); err != nil {
			t.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	return f.Name()
}

func TestExtractLicenseFromBundle_PrefersLicenseJSONOverConfigmap(t *testing.T) {
	// The configmap appears first in the tar but license.json should win.
	configmapContent := `{
  "kind": "ConfigMapList",
  "apiVersion": "v1",
  "items": [{
    "data": {
      "license": "configmapLicenseIDAAAAAAAAAAAA"
    }
  }]
}`
	licenseJSONContent := `{"licenseID":"correctLicenseIDAAAAAAAAAAAA","appSlug":"my-app"}`

	bundlePath := makeBundleTarGz(t, []struct{ name, content string }{
		// configmap comes first in the tar — this is the bug trigger
		{"bundle/cluster-resources/configmaps/kotsadm.json", configmapContent},
		{"bundle/cluster-resources/license.json", licenseJSONContent},
	})

	licenseID, appSlug, err := ExtractLicenseFromBundle(bundlePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if licenseID != "correctLicenseIDAAAAAAAAAAAA" {
		t.Errorf("licenseID = %q, want %q", licenseID, "correctLicenseIDAAAAAAAAAAAA")
	}
	if appSlug != "my-app" {
		t.Errorf("appSlug = %q, want %q", appSlug, "my-app")
	}
}

func TestExtractLicenseFromBundle_FallsBackToConfigmap(t *testing.T) {
	// No license.json — should fall back to configmap scan.
	configmapContent := `{
  "kind": "ConfigMapList",
  "apiVersion": "v1",
  "items": [{
    "data": {
      "licenseID": "fallbackLicenseIDAAAAAAAAAA"
    }
  }]
}`
	bundlePath := makeBundleTarGz(t, []struct{ name, content string }{
		{"bundle/cluster-resources/configmaps/my-app.json", configmapContent},
	})

	licenseID, appSlug, err := ExtractLicenseFromBundle(bundlePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if licenseID != "fallbackLicenseIDAAAAAAAAAA" {
		t.Errorf("licenseID = %q, want %q", licenseID, "fallbackLicenseIDAAAAAAAAAA")
	}
	if appSlug != "my-app" {
		t.Errorf("appSlug = %q, want %q", appSlug, "my-app")
	}
}

func TestExtractLicenseFromBundle_ReturnsEmptyWhenNotFound(t *testing.T) {
	bundlePath := makeBundleTarGz(t, []struct{ name, content string }{
		{"bundle/cluster-resources/configmaps/some.json", `{"kind":"ConfigMapList"}`},
	})

	licenseID, appSlug, err := ExtractLicenseFromBundle(bundlePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if licenseID != "" || appSlug != "" {
		t.Errorf("expected empty results, got licenseID=%q appSlug=%q", licenseID, appSlug)
	}
}

func TestExtractLicenseFromBundle_RealBundle(t *testing.T) {
	// Validates against the actual support bundle that triggered the bug.
	// The bundle has license.json at tar entry #853 but kotsadm configmap at #94.
	bundlePath := filepath.Join("..", "..", "support-bundle-2026-04-10T17_13_13.tar.gz")
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		t.Skip("real bundle not present")
	}

	licenseID, appSlug, err := ExtractLicenseFromBundle(bundlePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if licenseID != "36G95wTeTQoX7UYcm2QvhssCIkH" {
		t.Errorf("licenseID = %q, want %q", licenseID, "36G95wTeTQoX7UYcm2QvhssCIkH")
	}
	if appSlug != "embedded-cluster-smoke-test-staging-app" {
		t.Errorf("appSlug = %q, want %q", appSlug, "embedded-cluster-smoke-test-staging-app")
	}
}
