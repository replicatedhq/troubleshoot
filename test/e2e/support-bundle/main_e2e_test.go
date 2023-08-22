package e2e

import (
	"os"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
)

var testenv env.Environment

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName := envconf.RandomName("cluster-resource-cluster", 16)
	namespace := envconf.RandomName("crashloop-ns", 16)
	testenv.Setup(
		envfuncs.CreateKindCluster(kindClusterName),
		envfuncs.CreateNamespace(namespace),
	)
	testenv.Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.DestroyKindCluster(kindClusterName),
	)
	os.Exit(testenv.Run(m))
}
