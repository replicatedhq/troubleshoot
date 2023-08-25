package e2e

import (
	"context"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/third_party/helm"
)

func Test_HelmCollector(t *testing.T) {
	feature := features.New("Collector Helm Release").
		Setup(func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			cluster, ok := envfuncs.GetKindClusterFromContext(ctx, ClusterName)
			if !ok {
				t.Fatalf("Failed to extract kind cluster %s from context", ClusterName)
			}
			manager := helm.New(cluster.GetKubeconfig())
			err := manager.RunInstall(helm.WithName(ClusterName), helm.WithNamespace("default"), helm.WithChart("nginx"), helm.WithWait(), helm.WithTimeout("10m"))
			if err != nil {
				t.Fatal("failed to invoke helm install operation due to an error", err)
			}
			return ctx
		}).
		Feature()
	testenv.Test(t, feature)
}
