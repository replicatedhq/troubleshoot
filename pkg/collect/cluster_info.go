package collect

import (
	"encoding/json"
	"path/filepath"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
)

type ClusterVersion struct {
	Info   *version.Info `json:"info"`
	String string        `json:"string"`
}

func ClusterInfo(c *Collector) (map[string][]byte, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create kubernetes clientset")
	}

	clusterInfoOutput := map[string][]byte{}

	// cluster version
	clusterVersion, clusterErrors := clusterVersion(client)
	clusterInfoOutput[filepath.Join("cluster-info", "cluster_version.json")] = clusterVersion
	clusterInfoOutput[filepath.Join("cluster-info", "errors.json")], err = marshalNonNil(clusterErrors)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal errors")
	}

	return clusterInfoOutput, nil
}

func clusterVersion(client *kubernetes.Clientset) ([]byte, []string) {
	k8sVersion, err := client.ServerVersion()
	if err != nil {
		return nil, []string{err.Error()}
	}

	clusterVersion := ClusterVersion{
		Info:   k8sVersion,
		String: k8sVersion.String(),
	}

	b, err := json.MarshalIndent(clusterVersion, "", "  ")
	if err != nil {
		return nil, []string{err.Error()}
	}
	return b, nil
}
