package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

// TestNodeResourcesNoFiles verifies the behavior of the nodeResources
// analyzer when cluster-resources/nodes.json is not present in the
// bundle. The spec excludes the clusterResources collector so nodes.json
// is never written, then runs `support-bundle analyze` against the
// generated bundle.
//
// Expected outcomes:
//   - "warn-default":        warn outcome (default behavior)
//   - "warn-explicit-false": warn outcome (ignoreIfNoFiles: false)
//   - "ignored":             no result (ignoreIfNoFiles: true)
func TestNodeResourcesNoFiles(t *testing.T) {
	feature := features.New("Node Resources No Files").
		Assess("warns per analyzer when nodes.json is missing and respects ignoreIfNoFiles", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			supportBundleName := "node-resources-no-files-test"
			tarPath := fmt.Sprintf("%s.tar.gz", supportBundleName)
			specPath := "spec/nodeResourcesNoFiles.yaml"

			var collectOut bytes.Buffer
			collectCmd := exec.CommandContext(ctx, sbBinary(), specPath,
				"--interactive=false",
				fmt.Sprintf("-o=%s", supportBundleName),
			)
			collectCmd.Stdout = &collectOut
			collectCmd.Stderr = &collectOut
			require.NoErrorf(t, collectCmd.Run(), "support-bundle collect failed: %s", collectOut.String())

			defer func() {
				if err := os.Remove(tarPath); err != nil {
					t.Logf("Error removing %s: %v", tarPath, err)
				}
			}()

			// Sanity check: nodes.json should NOT have been collected.
			_, err := readFileFromTar(tarPath, fmt.Sprintf("%s/cluster-resources/nodes.json", supportBundleName))
			require.Error(t, err, "nodes.json should not exist in the bundle")

			var analyzeOut bytes.Buffer
			var analyzeErr bytes.Buffer
			analyzeCmd := exec.CommandContext(ctx, sbBinary(), "analyze",
				"--bundle", tarPath,
				"--output", "json",
				specPath,
			)
			analyzeCmd.Stdout = &analyzeOut
			analyzeCmd.Stderr = &analyzeErr
			require.NoErrorf(t, analyzeCmd.Run(), "support-bundle analyze failed: %s", analyzeErr.String())

			type analyzeResult struct {
				IsPass  bool   `json:"IsPass"`
				IsFail  bool   `json:"IsFail"`
				IsWarn  bool   `json:"IsWarn"`
				Title   string `json:"Title"`
				Message string `json:"Message"`
			}
			var results []analyzeResult
			require.NoError(t, json.Unmarshal(analyzeOut.Bytes(), &results), "analyzer JSON output: %s", analyzeOut.String())

			byTitle := map[string]analyzeResult{}
			for _, r := range results {
				byTitle[r.Title] = r
			}

			// Two warns, no result for the suppressed entry.
			assert.Len(t, results, 2, "expected exactly two analyzer results, got %d: %s", len(results), analyzeOut.String())

			for _, title := range []string{"warn-default", "warn-explicit-false"} {
				r, ok := byTitle[title]
				if !assert.Truef(t, ok, "expected an analyzer result with title %q", title) {
					continue
				}
				assert.Truef(t, r.IsWarn, "%q: expected IsWarn=true", title)
				assert.Falsef(t, r.IsFail, "%q: expected IsFail=false", title)
				assert.Falsef(t, r.IsPass, "%q: expected IsPass=false", title)
				assert.Containsf(t, r.Message, "No node resources were collected", "%q: unexpected message %q", title, r.Message)
			}

			_, ignored := byTitle["ignored"]
			assert.Falsef(t, ignored, "analyzer with ignoreIfNoFiles: true should have produced no result")

			return ctx
		}).Feature()

	testenv.Test(t, feature)
}
