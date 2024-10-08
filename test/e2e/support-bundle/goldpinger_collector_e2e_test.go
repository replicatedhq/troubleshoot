package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var specTemplate = `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: goldpinger
spec:
  collectors:
    - clusterResources:
        exclude: true
    - clusterInfo:
        exclude: true
    - goldpinger:
        namespace: $NAMESPACE
        collectDelay: 20s
  analyzers:
    - goldpinger: {}
`

func Test_GoldpingerCollector(t *testing.T) {
	feature := features.New("Goldpinger collector and analyser").
		Assess("collect and analyse goldpinger pings", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			var stdErr bytes.Buffer

			namespace := c.Namespace()
			supportBundleName := "goldpinger-test"
			spec := strings.ReplaceAll(specTemplate, "$NAMESPACE", namespace)
			specPath := filepath.Join(t.TempDir(), "goldpinger.yaml")

			err := os.WriteFile(specPath, []byte(spec), 0644)
			require.NoError(t, err)

			tarPath := filepath.Join(t.TempDir(), fmt.Sprintf("%s.tar.gz", supportBundleName))
			cmd := exec.CommandContext(ctx, sbBinary(), specPath, "--interactive=false", "-v=2", fmt.Sprintf("-o=%s", tarPath))
			cmd.Stdout = &out
			cmd.Stderr = &stdErr
			err = cmd.Run()
			if err != nil {
				t.Logf("Stdout: %s\n", out.String())
				t.Logf("Stderr: %s\n", stdErr.String())
				t.Fatal(err)
			}

			analysisJSON, err := readFileFromTar(tarPath, fmt.Sprintf("%s/analysis.json", supportBundleName))
			require.NoError(t, err)

			var analysisResults []convert.Result
			err = json.Unmarshal(analysisJSON, &analysisResults)
			require.NoError(t, err)

			// Check that we analysed collected goldpinger results.
			// We should expect a single analysis result for goldpinger.
			require.Equal(t, 1, len(analysisResults))
			assert.True(t, strings.HasPrefix(analysisResults[0].Name, "pings.to.ts.goldpinger."))
			if t.Failed() {
				t.Logf("Analysis results: %s\n", analysisJSON)
				t.Logf("Stdout: %s\n", out.String())
				t.Logf("Stderr: %s\n", stdErr.String())
				t.FailNow()
			}

			return ctx
		}).Feature()
	testenv.Test(t, feature)
}
