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

func TestSkippedCollectors(t *testing.T) {
	feature := features.New("Skipped Collectors").
		Assess("bundle contains skipped-collectors.json for excluded collectors", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer

			supportBundleName := "skipped-collectors-test"
			tarPath := fmt.Sprintf("%s.tar.gz", supportBundleName)
			cmd := exec.CommandContext(ctx, sbBinary(), "spec/skippedCollectors.yaml",
				"--interactive=false",
				fmt.Sprintf("-o=%s", supportBundleName),
			)
			cmd.Stdout = &out
			cmd.Stderr = &out
			err := cmd.Run()
			if err != nil {
				t.Fatalf("support-bundle command failed: %v\nOutput: %s", err, out.String())
			}

			defer func() {
				err := os.Remove(tarPath)
				if err != nil {
					t.Fatal("Error removing file:", err)
				}
			}()

			// Read skipped-collectors.json from the bundle
			skippedJSON, err := readFileFromTar(tarPath, fmt.Sprintf("%s/skipped-collectors.json", supportBundleName))
			require.NoError(t, err, "skipped-collectors.json should exist in the bundle")

			var skipped []struct {
				Collector string   `json:"collector"`
				Reason    string   `json:"reason"`
				Errors    []string `json:"errors"`
				Timestamp string   `json:"timestamp"`
			}
			err = json.Unmarshal(skippedJSON, &skipped)
			require.NoError(t, err)

			// Both excluded collectors should be recorded
			assert.Len(t, skipped, 2)

			collectors := map[string]string{}
			for _, s := range skipped {
				collectors[s.Collector] = s.Reason
				assert.NotEmpty(t, s.Timestamp, "timestamp should be set")
			}

			assert.Equal(t, "excluded", collectors["cluster-resources"])
			assert.Equal(t, "excluded", collectors["cluster-info"])

			return ctx
		}).Feature()

	testenv.Test(t, feature)
}
