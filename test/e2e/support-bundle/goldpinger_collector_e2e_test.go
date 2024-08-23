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
	"time"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	"github.com/replicatedhq/troubleshoot/pkg/convert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"
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
  analyzers:
    - goldpinger: {}
`

func Test_GoldpingerCollector(t *testing.T) {
	releaseName := "goldpinger"

	feature := features.New("Goldpinger collector and analyser").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster := getClusterFromContext(t, ctx, ClusterName)

			manager := helm.New(cluster.GetKubeconfig())
			err := manager.RunInstall(
				helm.WithName(releaseName),
				helm.WithNamespace(c.Namespace()),
				helm.WithChart(testutils.TestFixtureFilePath(t, "charts/goldpinger-6.0.1.tgz")),
				helm.WithWait(),
				helm.WithTimeout("2m"),
			)
			require.NoError(t, err)

			client, err := c.NewClient()
			require.NoError(t, err)

			// Lets wait for the goldpinger pods to be running
			ds := &appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "goldpinger", Namespace: c.Namespace()}}
			err = wait.For(
				conditions.New(client.Resources()).DaemonSetReady(ds),
				wait.WithTimeout(time.Second*30),
			)
			require.NoError(t, err)

			// HACK: wait for goldpinger to do its thing
			time.Sleep(time.Second * 30)

			return ctx
		}).
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
			assert.Equal(t, 1, len(analysisResults))
			assert.True(t, strings.HasPrefix(analysisResults[0].Name, "pings.to.goldpinger."))
			if t.Failed() {
				t.Logf("Analysis results: %s\n", analysisJSON)
				t.Logf("Stdout: %s\n", out.String())
				t.Logf("Stderr: %s\n", stdErr.String())
				t.FailNow()
			}

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster := getClusterFromContext(t, ctx, ClusterName)
			manager := helm.New(cluster.GetKubeconfig())
			err := manager.RunUninstall(helm.WithName(releaseName), helm.WithNamespace(c.Namespace()))
			require.NoError(t, err)
			return ctx
		}).
		Feature()
	testenv.Test(t, feature)
}
