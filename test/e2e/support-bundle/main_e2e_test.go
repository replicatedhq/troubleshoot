package e2e

import (
	"archive/tar"
	"bytes"
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
	kindClusterName := envconf.RandomName("e2e-cluster", 16)
	namespace := envconf.RandomName("default", 16)
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
	return nil, fmt.Errorf("File not found: %s", targetFile)
}
