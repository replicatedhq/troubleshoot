package collect

import (
	"bytes"
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

func ClusterInfo(c *Collector) (CollectorResult, error) {
	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create kubernetes clientset")
	}

	output := NewResult()

	clusterVersion, clusterErrors := clusterVersion(client)

	output.SaveResult(c.BundlePath, filepath.Join("cluster-info", "cluster_version.json"), bytes.NewBuffer(clusterVersion))
	output.SaveResult(c.BundlePath, filepath.Join("cluster-info", "errors.json"), marshalErrors(clusterErrors))

	return output, nil
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
