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
		paths      []string
		expectType string
	}{
		{
			paths: []string{
				"clusterroles.json",
				"volumeattachments.json",
				"nodes.json",
				"pvs.json",
				"resources.json",
				"custom-resource-definitions.json",
				"groups.json",
				"priorityclasses.json",
				"namespaces.json",
				"clusterrolebindings.json",
				"storage-classes.json",
			},
			expectType: "file",
		},
		{
			paths: []string{
				"cronjobs",
				"limitranges",
				"daemonsets",
				"deployments",
				"pvcs",
				"leases",
				"auth-cani-list",
				"services",
				"roles",
				"events",
				"rolebindings",
				"statefulsets-errors.json",
				"jobs",
				"serviceaccounts",
				"configmaps",
				"statefulsets",
				"endpoints",
				"network-policy",
				"resource-quota",
				"ingress",
				"pods",
				"pod-disruption-budgets",
			},
			expectType: "folder",
		},
	}

	feature := features.New("Cluster Resources Test").
		Assess("check support bundle catch cluster resources", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			supportBundleName := "cluster-resources"
			tarPath := fmt.Sprintf("%s.tar.gz", supportBundleName)
			targetFolder := fmt.Sprintf("%s/cluster-resources/", supportBundleName)
			cmd := exec.CommandContext(ctx, sbBinary(), "spec/clusterResources.yaml", "--interactive=false", fmt.Sprintf("-o=%s", supportBundleName))
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
					for _, path := range test.paths {
						if !slices.Contains(files, path) {
							t.Fatalf("Expected file %s not found", path)
						}
					}
				} else if test.expectType == "folder" {
					for _, path := range test.paths {
						if !slices.Contains(folders, path) {
							t.Fatalf("Expected folder %s not found", path)
						}
					}
				}
			}

			return ctx
		}).Feature()
	testenv.Test(t, feature)
}
