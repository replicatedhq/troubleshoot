package analyzer

import (
	"path/filepath"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/require"
)

func Test_clusterResource(t *testing.T) {
	tests := []struct {
		name     string
		isError  bool
		analyzer troubleshootv1beta2.ClusterResource
	}{
		{
			name: "namespaced resource",
			analyzer: troubleshootv1beta2.ClusterResource{
				CollectorName: "Check namespaced resource",
				Kind:          "Deployment",
				Namespace:     "kube-system",
				Name:          "coredns",
			},
		},
		{
			name: "cluster scoped resource",
			analyzer: troubleshootv1beta2.ClusterResource{
				CollectorName: "Check namespaced resource",
				Kind:          "Node",
				Name:          "repldev-marc",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			rootDir := filepath.Join("files", "support-bundle")
			fcp := fileContentProvider{rootDir: rootDir}

			analyzer := &test.analyzer
			_, err := FindResource(analyzer.Kind, analyzer.Namespace, analyzer.Name, fcp.getFileContents)

			if !test.isError {
				req.NoError(err)
			} else {
				req.Error(err)
			}
		})
	}
}
