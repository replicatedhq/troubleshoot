package e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/exp/slices"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestClusterResources(t *testing.T) {
	tests := []struct {
		path       string
		expectType string
	}{
		{
			path:       "clusterroles.json",
			expectType: "file",
		},
		{
			path:       "volumeattachments.json",
			expectType: "file",
		},
		{
			path:       "daemonsets",
			expectType: "folder",
		},
	}

	feature := features.New("Cluster Resouces Test").
		Assess("check support bundle catch cluster resouces", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			supportBundleName := "cluster-resources"
			cmd := exec.Command("../../../bin/support-bundle", "spec/clusterResources.yaml", "--interactive=false", fmt.Sprintf("-o=%s", supportBundleName))
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}

			file, err := os.Open(fmt.Sprintf("%s.tar.gz", supportBundleName))
			if err != nil {
				t.Fatal("Error opening file:", err)
				return ctx
			}

			// Initialize gzip reader
			gzipReader, err := gzip.NewReader(file)
			if err != nil {
				t.Fatal("Error initializing gzip reader:", err)
				return ctx
			}

			tarReader := tar.NewReader(gzipReader)

			// Folder to look for
			targetFolder := fmt.Sprintf("%s/cluster-resources/", supportBundleName)
			var files []string
			var folders []string

			// Loop to read each file in the tar archive
			for {
				header, err := tarReader.Next()

				if err == io.EOF {
					break // End of archive, break loop
				}

				if err != nil {
					t.Fatal("Error reading tar:", err)
					return ctx
				}

				// Check if the current file or folder is under the target folder
				if strings.HasPrefix(header.Name, targetFolder) {
					// Remove the prefix to display only the relative path under the target folder
					relativePath := strings.TrimPrefix(header.Name, targetFolder)

					if relativePath != "" { // Skip the target folder itself
						relativeDir := filepath.Dir(relativePath)
						if relativeDir != "." {
							parentDir := strings.Split(relativeDir, "/")
							folders = append(folders, parentDir[0])
						} else {
							files = append(files, relativePath)
						}
					}
				}
			}

			for _, test := range tests {
				if test.expectType == "file" {
					if !slices.Contains(files, test.path) {
						t.Fatalf("Expected file %s not found", test.path)
					}
				} else if test.expectType == "folder" {
					if !slices.Contains(folders, test.path) {
						t.Fatalf("Expected folder %s not found", test.path)
					}
				}
			}

			defer func() {
				file.Close()
				gzipReader.Close()
				err := os.Remove(fmt.Sprintf("%s.tar.gz", supportBundleName))
				if err != nil {
					t.Fatal("Error remove file:", err)
				}
			}()

			return ctx
		}).Feature()
	testenv.Test(t, feature)
}
