package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type ClusterVersion struct {
	Info   *version.Info `json:"info"`
	String string        `json:"string"`
}

type CollectClusterInfo struct {
	Collector    *troubleshootv1beta2.ClusterInfo
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	RBACErrors   []error
}

func (c *CollectClusterInfo) Title() string {
	return collectorTitleOrDefault(c.Collector.CollectorMeta, "Cluster Info")
}

func (c *CollectClusterInfo) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectClusterInfo) GetRBACErrors() []error {
	return c.RBACErrors
}

func (c *CollectClusterInfo) HasRBACErrors() bool {
	return len(c.RBACErrors) > 0
}

func (c *CollectClusterInfo) CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error {
	exclude, err := c.IsExcluded()
	if err != nil || exclude != true {
		return nil
	}

	rbacErrors, err := checkRBAC(ctx, c.ClientConfig, c.Namespace, c.Title(), collector)
	if err != nil {
		return err
	}

	c.RBACErrors = rbacErrors

	return nil
}

func (c *CollectClusterInfo) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
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
