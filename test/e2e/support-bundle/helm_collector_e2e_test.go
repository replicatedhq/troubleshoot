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
			cluster := getClusterFromContext(t, ctx, ClusterName)
			manager := helm.New(cluster.GetKubeconfig())
			manager.RunInstall(helm.WithName(releaseName), helm.WithNamespace(c.Namespace()), helm.WithChart(filepath.Join(curDir, "testdata/charts/nginx-15.2.0.tgz")), helm.WithArgs("-f "+filepath.Join(curDir, "testdata/helm-values.yaml")), helm.WithWait(), helm.WithTimeout("1m"))
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
			cmd := exec.CommandContext(ctx, sbBinary(), "spec/helm.yaml", "--load-cluster-specs=false", "--interactive=false", fmt.Sprintf("-o=%s", supportBundleName))
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
			assert.Equal(t, map[string]interface{}{"name": "TEST_ENV_VAR", "value": "test-value"}, results[0].VersionInfo[0].Values["extraEnvVars"].([]interface{})[0])
			assert.Equal(t, "1.25.2-debian-11-r3", results[0].VersionInfo[0].Values["image"].(map[string]interface{})["tag"])
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster := getClusterFromContext(t, ctx, ClusterName)
			manager := helm.New(cluster.GetKubeconfig())
			manager.RunUninstall(helm.WithName(releaseName), helm.WithNamespace(c.Namespace()))
			return ctx
		}).
		Feature()
	testenv.Test(t, feature)
}
