package e2e

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

type sonobuoyContextKey string

func Test_SonobuoyCollector(t *testing.T) {

	feature := features.New("Collector Sonobuoy Results").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			tmpdir := t.TempDir()

			cluster := getClusterFromContext(t, ctx, ClusterName)

			// download sonobuoy
			resp, err := http.Get(fmt.Sprintf(
				"https://github.com/vmware-tanzu/sonobuoy/releases/download/v0.57.1/sonobuoy_0.57.1_%s_%s.tar.gz",
				runtime.GOOS, runtime.GOARCH,
			))
			require.NoError(t, err, "failed to download sonobuoy")
			defer resp.Body.Close()
			f, err := os.Create(filepath.Join(tmpdir, "sonobuoy.tar.gz"))
			require.NoError(t, err)
			defer f.Close()
			_, err = io.Copy(f, resp.Body)
			require.NoError(t, err)
			err = exec.Command("tar", "-xvf", filepath.Join(tmpdir, "sonobuoy.tar.gz"), "-C", tmpdir).Run()
			require.NoError(t, err, "failed to extract sonobuoy.tar.gz")
			sonobuoy := filepath.Join(tmpdir, "sonobuoy")
			ctx = context.WithValue(ctx, sonobuoyContextKey("sonobuoy"), sonobuoy)

			// run sonobuoy
			_ = exec.Command(sonobuoy, "delete", "--kubeconfig", cluster.GetKubeconfig(), "--wait").Run()
			out, err := exec.Command(sonobuoy, "run", "--kubeconfig", cluster.GetKubeconfig(), "--mode", "quick", "--wait").CombinedOutput()
			require.NoError(t, err, "failed to run sonobuoy: %s", string(out))

			return ctx
		}).
		Assess("check support bundle catch helm release", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			tmpdir := t.TempDir()

			tarPath := filepath.Join(tmpdir, "bundle.tar.gz")
			targetFolder := "bundle/sonobuoy"
			// 202402130428_sonobuoy_92a8ac6c-b5ed-4af5-bfc6-8bb454ceb0f0.tar.gz
			targetFileMatch := regexp.MustCompile(".*_sonobuoy_.*.tar.gz")

			cmd := exec.CommandContext(ctx, sbBinary(), "spec/sonobuoy.yaml", "--interactive=false", fmt.Sprintf("-o=%s", tarPath))
			out, err := cmd.CombinedOutput()
			t.Log(string(out))
			require.NoError(t, err, "failed to run support-bundle")

			// validate the tarball
			files, _, err := readFilesAndFoldersFromTar(tarPath, targetFolder)
			require.NoError(t, err, "failed to read files and folders from tarball")

			found := false
			for _, file := range files {
				if targetFileMatch.MatchString(file) {
					found = true
				}
			}
			require.True(t, found, "sonobuoy tarball not found in support bundle")

			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			sonobuoy := ctx.Value(sonobuoyContextKey("sonobuoy")).(string)

			cluster := getClusterFromContext(t, ctx, ClusterName)

			out, err := exec.Command(sonobuoy, "delete", "--kubeconfig", cluster.GetKubeconfig(), "--wait").CombinedOutput()
			if err != nil {
				t.Logf("Error deleting sonobuoy: %s", string(out))
			}

			return ctx
		}).
		Feature()
	testenv.Test(t, feature)
}
