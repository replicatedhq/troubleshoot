package e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/klog/v2"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kind"
)

var testenv env.Environment

const ClusterName = "kind-cluster"

func TestMain(m *testing.M) {
	// enable klog
	klog.InitFlags(nil)
	if os.Getenv("E2E_VERBOSE") == "1" {
		_ = flag.Set("v", "10")
	}

	testenv = env.New()
	namespace := envconf.RandomName("default", 16)
	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), ClusterName),
		envfuncs.CreateNamespace(namespace),
	)
	testenv.Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyCluster(ClusterName),
	)
	os.Exit(testenv.Run(m))
}

func getClusterFromContext(t *testing.T, ctx context.Context, clusterName string) *kind.Cluster {
	provider, ok := envfuncs.GetClusterFromContext(ctx, ClusterName)
	if !ok {
		t.Fatalf("Failed to extract kind cluster %s from context", ClusterName)
	}
	cluster, ok := provider.(*kind.Cluster)
	if !ok {
		t.Fatalf("Failed to cast kind cluster %s from provider", ClusterName)
	}

	return cluster
}

func readFilesAndFoldersFromTar(tarPath, targetFolder string) ([]string, []string, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return nil, nil, fmt.Errorf("Error opening file: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, nil, fmt.Errorf("Error initializing gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	var files []string
	var folders []string

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("Error reading tar: %w", err)
		}

		if strings.HasPrefix(header.Name, targetFolder) {
			relativePath, err := filepath.Rel(targetFolder, header.Name)
			if err != nil {
				return nil, nil, fmt.Errorf("Error getting relative path: %w", err)
			}
			if relativePath != "" {
				relativeDir := filepath.Dir(relativePath)
				if relativeDir != "." {
					parentDir := strings.Split(relativeDir, "/")[0]
					folders = append(folders, parentDir)
				} else {
					files = append(files, relativePath)
				}
			}
		}
	}

	return files, folders, nil
}

func readFileFromTar(tarPath, targetFile string) ([]byte, error) {
	file, err := os.Open(tarPath)
	if err != nil {
		return nil, fmt.Errorf("Error opening file: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("Error initializing gzip reader: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("Error reading tar: %w", err)
		}

		if header.Name == targetFile {
			buf := new(bytes.Buffer)
			_, err = io.Copy(buf, tarReader)
			if err != nil {
				return nil, fmt.Errorf("Error copying data: %w", err)
			}
			return buf.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("File not found: %q", targetFile)
}

func sbBinary() string {
	return "../../../bin/support-bundle"
}
