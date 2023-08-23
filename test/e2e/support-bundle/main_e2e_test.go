package e2e

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

var testenv env.Environment

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName := envconf.RandomName("cluster-resource-cluster", 16)
	namespace := envconf.RandomName("crashloop-ns", 16)
	testenv.Setup(
		envfuncs.CreateKindCluster(kindClusterName),
		envfuncs.CreateNamespace(namespace),
	)
	testenv.Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyKindCluster(kindClusterName),
	)
	os.Exit(testenv.Run(m))
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
			relativePath := strings.TrimPrefix(header.Name, targetFolder)
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
