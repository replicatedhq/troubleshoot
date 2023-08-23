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
				"pod-disruption-budgets-errors.json",
				"resources.json",
				"cronjobs-errors.json",
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
				"limitranges",
				"daemonsets",
				"deployments",
				"pvcs",
				"leases",
				"auth-cani-list",
				"services",
				"deployments",
				"pvcs",
				"roles",
				"events",
				"rolebindings",
				"statefulsets-errors.json",
				"pvcs",
				"jobs",
				"serviceaccounts",
				"serviceaccounts",
				"configmaps",
				"serviceaccounts",
				"statefulsets",
				"endpoints",
				"limitranges",
				"services",
				"daemonsets",
				"leases",
				"auth-cani-list",
				"endpoints",
				"endpoints",
				"pods",
				"auth-cani-list",
				"deployments",
				"network-policy",
				"statefulsets",
				"endpoints",
				"network-policy",
				"pvcs",
				"resource-quota",
				"roles",
				"serviceaccounts",
				"pods",
				"deployments",
				"statefulsets-errors.json",
				"configmaps",
				"ingress",
				"services",
				"pods",
				"configmaps",
				"pods",
				"rolebindings",
				"roles",
				"events",
				"leases",
				"resource-quota",
				"roles",
				"roles",
				"events",
				"daemonsets",
				"statefulsets-errors.json",
				"limitranges",
				"jobs",
				"network-policy",
				"endpoints",
				"services",
				"serviceaccounts",
				"pvcs",
				"daemonsets",
				"ingress",
				"rolebindings",
				"deployments",
				"pods",
				"auth-cani-list",
				"jobs",
				"serviceaccounts",
				"configmaps",
				"rolebindings",
				"resource-quota",
				"resource-quota",
				"daemonsets",
				"statefulsets",
				"limitranges",
				"statefulsets",
				"endpoints",
				"network-policy",
				"pvcs",
				"daemonsets",
				"limitranges",
				"resource-quota",
				"auth-cani-list",
				"network-policy",
				"statefulsets",
				"leases",
				"statefulsets-errors.json",
				"events",
				"statefulsets-errors.json",
				"jobs",
				"pods",
				"configmaps",
				"events",
				"pods",
				"ingress",
				"ingress",
				"statefulsets",
				"events",
				"limitranges",
				"ingress",
				"leases",
				"pods",
				"services",
				"configmaps",
				"jobs",
				"ingress",
				"rolebindings",
				"pods",
				"services",
				"rolebindings",
				"roles",
				"leases",
				"statefulsets-errors.json",
				"pods",
				"auth-cani-list",
				"jobs",
				"deployments",
				"network-policy",
				"resource-quota",
			},
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
