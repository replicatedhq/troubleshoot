package analyzer

import (
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/pkg/errors"
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

			getCollectedFileContents := func(dirName string) (map[string][]byte, error) {
				files, err := filepath.Glob(filepath.Join("files/support-bundle", dirName))
				if err != nil {
					return nil, errors.Wrapf(err, "invalid glob %q", dirName)
				}
				fileArr := map[string][]byte{}
				for _, filePath := range files {
					bytes, err := ioutil.ReadFile(filePath)
					if err != nil {
						return nil, errors.Wrapf(err, "read %q", filePath)
					}
					fileArr[filePath] = bytes
				}
				return fileArr, nil
			}

			analyzer := &test.analyzer
			_, err := FindResource(analyzer.Kind, analyzer.Namespace, analyzer.Name, getCollectedFileContents)

			if !test.isError {
				req.NoError(err)
			} else {
				req.Error(err)
			}
		})
	}
}
