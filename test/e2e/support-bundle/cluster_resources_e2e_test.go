package analyzer

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

var testenv env.Environment

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName := envconf.RandomName("cluster-resource-cluster", 16)
	testenv.Setup(
		envfuncs.CreateKindCluster(kindClusterName),
	)
	testenv.Finish(
		envfuncs.DestroyKindCluster(kindClusterName),
	)
	os.Exit(testenv.Run(m))
}

func TestClusterResources(t *testing.T) {
	feature := features.New("Cluster Resouces Test").
		Assess("check support bundle catch cluster resouces", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			cmd := exec.Command("../../../bin/support-bundle", "spec/clusterResources.yaml", "--interactive=false")
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				t.Fatal(err)
			}
			return ctx
		}).Feature()
	testenv.Test(t, feature)
}
