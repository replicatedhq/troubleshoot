package collect

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type ClusterVersion struct {
	Info   *version.Info `json:"info"`
	String string        `json:"string"`
}

type ClusterInfoOutput struct {
	ClusterVersion []byte `json:"cluster-info/cluster_version.json,omitempty"`
	Errors         []byte `json:"cluster-info/errors.json,omitempty"`
}

func ClusterInfo() error {
	cfg, err := config.GetConfig()
	if err != nil {
		return errors.Wrap(err, "failed to get kubernetes config")
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return errors.Wrap(err, "Failed to create kuberenetes clientset")
	}

	clusterInfoOutput := ClusterInfoOutput{}

	// cluster version
	clusterVersion, clusterErrors := clusterVersion(client)
	clusterInfoOutput.ClusterVersion = clusterVersion
	clusterInfoOutput.Errors, err = marshalNonNil(clusterErrors)
	if err != nil {
		return errors.Wrap(err, "failed to marshal errors")
	}

	b, err := json.MarshalIndent(clusterInfoOutput, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to marshal cluster info")
	}

	fmt.Printf("%s\n", b)

	return nil
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
