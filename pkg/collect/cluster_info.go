package collect

import (
	"encoding/json"
	"fmt"

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
}

func ClusterInfo() error {
	cfg, err := config.GetConfig()
	if err != nil {
		return err
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}

	clusterInfoOutput := ClusterInfoOutput{}

	// cluster version
	clusterVersion, err := clusterVersion(client)
	if err != nil {
		return err
	}
	clusterInfoOutput.ClusterVersion = clusterVersion

	b, err := json.MarshalIndent(clusterInfoOutput, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", b)

	return nil
}

func clusterVersion(client *kubernetes.Clientset) ([]byte, error) {
	k8sVersion, err := client.ServerVersion()
	if err != nil {
		return nil, err
	}

	clusterVersion := ClusterVersion{
		Info:   k8sVersion,
		String: k8sVersion.String(),
	}

	b, err := json.MarshalIndent(clusterVersion, "", "  ")
	if err != nil {
		return nil, err
	}
	return b, nil
}
