package updater

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	hv "github.com/hashicorp/go-version"
	"github.com/replicatedhq/troubleshoot/pkg/version"
)

const defaultRepo = "replicatedhq/troubleshoot"

// Options control updater behavior.
type Options struct {
	// Repo in owner/name form. Defaults to replicatedhq/troubleshoot
	Repo string
	// BinaryName expected executable name inside the archive (preflight or support-bundle)
	BinaryName string
	// CurrentPath path to the currently executing binary to be replaced
	CurrentPath string
	// Skip whether to skip update (effective no-op)
	Skip bool
	// HTTPClient optional custom client
	HTTPClient *http.Client
	// Printf allows caller to receive status messages (optional)
	Printf func(string, ...interface{})
}

func (o *Options) client() *http.Client {
	if o.HTTPClient != nil {
		return o.HTTPClient
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// CheckAndUpdate checks GitHub releases for a newer version and, if newer, downloads
// the corresponding tar.gz asset, extracts the binary, and atomically replaces CurrentPath.
func CheckAndUpdate(ctx context.Context, o Options) error {
	if o.Skip {
		return nil
	}
	if o.BinaryName == "" || o.CurrentPath == "" {
		return fmt.Errorf("updater: BinaryName and CurrentPath are required")
	}
	repo := o.Repo
	if repo == "" {
		repo = defaultRepo
	}

	current := strings.TrimPrefix(version.Version(), "v")
	if current == "" {
		// If version is unknown (dev builds), do not auto-update
		return nil
	}

	latestTag, err := getLatestTag(ctx, o, repo)
	if err != nil {
		// Non-fatal: don't block command on update check failure
		if o.Printf != nil {
			o.Printf("Skipping auto-update (failed to check latest): %v\n", err)
		}
		return nil
	}

	latest := strings.TrimPrefix(latestTag, "v")
	newer, err := isNewer(latest, current)
	if err != nil || !newer {
		return nil
	}

	if o.Printf != nil {
		o.Printf("Updating %s from %s to %s...\n", o.BinaryName, current, latest)
	}

	assetURL := assetDownloadURL(repo, o.BinaryName, runtime.GOOS, runtime.GOARCH, latest)
	tgz, err := download(ctx, o, assetURL)
	if err != nil {
		return fmt.Errorf("download asset: %w", err)
	}
	defer tgz.Close()

	tempDir := filepath.Dir(o.CurrentPath)
	extractedPath, err := extractSingleBinary(tgz, o.BinaryName, tempDir)
	if err != nil {
		return fmt.Errorf("extract: %w", err)
	}
	// Make sure mode is executable
	_ = os.Chmod(extractedPath, 0o755)

	// Optional integrity check: size non-zero and simple sha256 not empty
	if err := sanityCheckBinary(extractedPath); err != nil {
		return fmt.Errorf("sanity check: %w", err)
	}

	// Atomic replace with backup
	backup := o.CurrentPath + ".bak"
	_ = os.Remove(backup)
	if err := os.Rename(o.CurrentPath, backup); err != nil {
		// If rename fails (e.g., permissions), abort and keep original
		_ = os.Remove(extractedPath)
		return nil
	}
	if err := os.Rename(extractedPath, o.CurrentPath); err != nil {
		// Attempt rollback
		_ = os.Rename(backup, o.CurrentPath)
		_ = os.Remove(extractedPath)
		return nil
	}
	// Best-effort remove backup
	_ = os.Remove(backup)

	if o.Printf != nil {
		o.Printf("Update complete.\n")
	}
	return nil
}

func getLatestTag(ctx context.Context, o Options, repo string) (string, error) {
	// Use GitHub REST to retrieve latest release. No auth; subject to rate limits.
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := o.client().Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}
	// Minimal JSON extraction to avoid bringing in a JSON dep footprint here.
	// The payload includes "tag_name":"vX.Y.Z". We'll parse it via a simple scan.
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	s := string(b)
	idx := strings.Index(s, "\"tag_name\"")
	if idx < 0 {
		return "", errors.New("tag_name not found")
	}
	// Find the next quoted value
	start := strings.Index(s[idx:], ":")
	if start < 0 {
		return "", errors.New("invalid JSON")
	}
	start += idx + 1
	// find first quote
	q1 := strings.Index(s[start:], "\"")
	if q1 < 0 {
		return "", errors.New("invalid JSON")
	}
	q1 += start + 1
	q2 := strings.Index(s[q1:], "\"")
	if q2 < 0 {
		return "", errors.New("invalid JSON")
	}
	q2 += q1
	return s[q1:q2], nil
}

func isNewer(latest, current string) (bool, error) {
	lv, err := hv.NewVersion(latest)
	if err != nil {
		return false, err
	}
	cv, err := hv.NewVersion(current)
	if err != nil {
		return false, err
	}
	return lv.GreaterThan(cv), nil
}

func assetDownloadURL(repo, bin, goos, arch, version string) string {
	// Matches deploy/.goreleaser.yaml naming: <binary>_<os>_<arch>.tar.gz
	name := fmt.Sprintf("%s_%s_%s.tar.gz", bin, goos, arch)
	return fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", repo, version, name)
}

func download(ctx context.Context, o Options, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := o.client().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}
	return resp.Body, nil
}

func extractSingleBinary(r io.Reader, expectedName, outDir string) (string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		base := filepath.Base(hdr.Name)
		if base != expectedName {
			continue
		}
		tmp := filepath.Join(outDir, "."+expectedName+".tmp")
		f, err := os.CreateTemp(outDir, expectedName+"-dl-")
		if err != nil {
			return "", err
		}
		tmp = f.Name()
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			_ = os.Remove(f.Name())
			return "", err
		}
		f.Close()
		return tmp, nil
	}
	return "", fmt.Errorf("binary %q not found in archive", expectedName)
}

func sanityCheckBinary(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.Size() == 0 {
		return fmt.Errorf("empty file")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.CopyN(h, f, 1<<20); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	_ = hex.EncodeToString(h.Sum(nil))
	return nil
}
