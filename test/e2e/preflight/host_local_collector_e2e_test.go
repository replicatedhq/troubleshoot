package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestHostLocalCollector(t *testing.T) {
	tests := []struct {
		paths            []string
		notExpectedPaths []string
		expectType       string
	}{
		{
			paths: []string{
				"cpu.json",
			},
			notExpectedPaths: []string{
				"node_list.json",
			},
			expectType: "file",
		},
	}

	feature := features.New("Preflight Host Local Collector").
		Assess("check preflight catch host local collector", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			var errOut bytes.Buffer
			preflightName := "preflightbundle"
			cmd := exec.CommandContext(ctx, preflightBinary(), "spec/localHostCollectors.yaml", "--interactive=false")
			cmd.Stdout = &out
			cmd.Stderr = &errOut

			err := cmd.Run()
			tarPath := GetFilename(errOut.String(), preflightName)
			if err != nil {
				if tarPath == "" {
					t.Error(err)
				}
			}

			defer func() {
				err := os.Remove(tarPath)
				if err != nil {
					t.Error("Error remove file:", err)
				}
			}()

			targetFile := fmt.Sprintf("%s/host-collectors/system/", strings.TrimSuffix(tarPath, ".tar.gz"))

			files, _, err := readFilesAndFoldersFromTar(tarPath, targetFile)
			if err != nil {
				t.Error(err)
			}

			for _, test := range tests {
				if test.expectType == "file" {
					for _, path := range test.paths {
						if !slices.Contains(files, path) {
							t.Errorf("Expected file %s not found in the tarball", path)
						}
					}
					for _, path := range test.notExpectedPaths {
						if slices.Contains(files, path) {
							t.Errorf("Unexpected file %s found in the tarball", path)
						}
					}
				}
			}
			return ctx
		}).Feature()
	testenv.Test(t, feature)
}

func GetFilename(input, prefix string) string {
	// Split the input into words
	words := strings.Fields(input)
	// Iterate over each word to find the one starting with the prefix
	for _, word := range words {
		if strings.HasPrefix(word, prefix) {
			return word
		}
	}

	// Return an empty string if no match is found
	return ""
}
