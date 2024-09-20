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

func TestHostOSRemoteCollector(t *testing.T) {
	tests := []struct {
		paths      []string
		expectType string
	}{
		{
			paths: []string{
				"hostos_info.json",
			},
			expectType: "file",
		},
	}

	feature := features.New("Host OS Remote Collector Test").
		Assess("check support bundle catch host os remote collector", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			supportbundleName := "host-os-remote-collector"
			tarPath := fmt.Sprintf("%s.tar.gz", supportbundleName)
			targetFolder := fmt.Sprintf("%s/host-collectors/system/", supportbundleName)
			cmd := exec.CommandContext(ctx, sbBinary(), "spec/hostOSRemoteCollector.yaml", "--interactive=false", fmt.Sprintf("-o=%s", supportbundleName))
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}

			defer func() {
				err := os.Remove(fmt.Sprintf("%s.tar.gz", supportbundleName))
				if err != nil {
					t.Fatal("Error remove file:", err)
				}
			}()

			files, _, err := readFilesAndFoldersFromTar(tarPath, targetFolder)
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
				}
			}

			return ctx
		}).Feature()
	testenv.Test(t, feature)
}
