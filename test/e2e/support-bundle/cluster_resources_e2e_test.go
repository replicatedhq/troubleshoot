package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/convert"
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
			},
			expectType: "folder",
		},
	}

	analysisTests := []struct {
		name     string
		severity convert.Severity
		detail   string
	}{
		{
			name:     "total.cpu.cores.in.the.cluster.is.2.or.greater",
			severity: convert.SeverityDebug,
			detail:   "There are at least 2 cores in the cluster.",
		},
	}

	feature := features.New("Cluster Resouces Test").
		Assess("check support bundle catch cluster resouces", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var stdout, stderr bytes.Buffer
			supportBundleName := "cluster-resources"
			tarPath := fmt.Sprintf("%s.tar.gz", supportBundleName)
			targetFolder := fmt.Sprintf("%s/cluster-resources/", supportBundleName)
			cmd := exec.CommandContext(ctx, sbBinary(), "spec/clusterResources.yaml", "-v=2", "--interactive=false", fmt.Sprintf("-o=%s", supportBundleName))
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			t.Log("Stdout: ", stdout.String())
			t.Log("Stderr: ", stderr.String())

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

			targetFile := fmt.Sprintf("%s/analysis.json", supportBundleName)
			analysisJSON, err := readFileFromTar(tarPath, targetFile)
			if err != nil {
				t.Fatal(err)
			}
			t.Log("analysisJSON: ", string(analysisJSON))

			analysisResults := []convert.Result{}
			err = json.Unmarshal(analysisJSON, &analysisResults)
			if err != nil {
				t.Fatal(err)
			}

			for _, test := range analysisTests {
				found := false
				for _, result := range analysisResults {
					if result.Name == test.name {
						found = true
						if result.Severity != test.severity {
							t.Fatalf("Expected severity %s, got %s", test.severity, result.Severity)
						}
						if result.Insight.Detail != test.detail {
							t.Fatalf("Expected detail %s, got %s", test.detail, result.Insight.Detail)
						}
					}
				}
				if !found {
					t.Fatalf("Expected result %q not found", test.name)
				}
			}

			return ctx
		}).Feature()
	testenv.Test(t, feature)
}
