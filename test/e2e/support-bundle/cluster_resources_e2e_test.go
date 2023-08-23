package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
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
			tarPath := fmt.Sprintf("%s.tar.gz", supportBundleName)
			targetFolder := fmt.Sprintf("%s/cluster-resources/", supportBundleName)
			cmd := exec.Command("../../../bin/support-bundle", "spec/clusterResources.yaml", "--interactive=false", fmt.Sprintf("-o=%s", supportBundleName))
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				err := os.Remove(fmt.Sprintf("%s.tar.gz", supportBundleName))
				if err != nil {
					t.Fatal("Error remove file:", err)
				}
			}()

			files, folders, err := readFilesAndFoldersFromTar(tarPath, targetFolder)

			if err != nil {
				t.Fatal(err)
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

			return ctx
		}).Feature()
	testenv.Test(t, feature)
}
