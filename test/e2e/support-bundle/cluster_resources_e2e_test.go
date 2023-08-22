package e2e

import (
	"bytes"
	"context"
	"os/exec"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestClusterResources(t *testing.T) {
	feature := features.New("Cluster Resouces Test").
		Assess("check support bundle catch cluster resouces", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			cmd := exec.Command("../../../bin/support-bundle", "spec/clusterResources.yaml", "--interactive=false", "-o test")
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()
	testenv.Test(t, feature)
}
