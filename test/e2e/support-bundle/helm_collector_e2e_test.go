package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"

	collect "github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
)

var curDir, _ = os.Getwd()

func Test_HelmCollector(t *testing.T) {
	releaseName := "nginx-release"

	feature := features.New("Collector Helm Release").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster, ok := envfuncs.GetKindClusterFromContext(ctx, ClusterName)
			if !ok {
				t.Fatalf("Failed to extract kind cluster %s from context", ClusterName)
			}
			manager := helm.New(cluster.GetKubeconfig())
			manager.RunInstall(helm.WithName(releaseName), helm.WithNamespace(c.Namespace()), helm.WithChart(filepath.Join(curDir, "testdata/charts/nginx-15.2.0.tgz")), helm.WithWait(), helm.WithTimeout("1m"))
			//ignore error to allow test to speed up, helm collector will catch the pending or deployed helm release status
			return ctx
		}).
		Assess("check support bundle catch helm release", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			var results []collect.ReleaseInfo

			supportBundleName := "helm-release"
			namespace := c.Namespace()
			tarPath := fmt.Sprintf("%s.tar.gz", supportBundleName)
			targetFile := fmt.Sprintf("%s/helm/%s.json", supportBundleName, namespace)
			cmd := exec.Command("../../../bin/support-bundle", "spec/helm.yaml", "--interactive=false", fmt.Sprintf("-o=%s", supportBundleName))
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

			resultJSON, err := readFileFromTar(tarPath, targetFile)
			if err != nil {
				t.Fatal(err)
			}

			err = json.Unmarshal(resultJSON, &results)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, 1, len(results))
			assert.Equal(t, releaseName, results[0].ReleaseName)
			assert.Equal(t, "nginx", results[0].Chart)
			return ctx
		}).
		Feature()
	testenv.Test(t, feature)
}
