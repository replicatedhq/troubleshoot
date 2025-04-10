package collect

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

const (
	summaryUrlTemplate = "/api/v1/nodes/%s/proxy/stats/summary"
)

type CollectNodeMetrics struct {
	Collector    *troubleshootv1beta2.NodeMetrics
	BundlePath   string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

func (c *CollectNodeMetrics) Title() string {
	return getCollectorName(c)
}

func (c *CollectNodeMetrics) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectNodeMetrics) SkipRedaction() bool {
	return c.Collector.SkipRedaction
}

func (c *CollectNodeMetrics) Collect(progressChan chan<- interface{}) (CollectorResult, error) {
	output := NewResult()
	nodesMap := c.constructNodesMap()
	if len(nodesMap) == 0 {
		klog.V(2).Info("no nodes found to collect metrics for")
		return output, nil
	}

	nodeNames := make([]string, 0, len(nodesMap))
	for nodeName := range nodesMap {
		nodeNames = append(nodeNames, nodeName)
	}

	klog.V(2).Infof("collecting node metrics for [%s] nodes", strings.Join(nodeNames, ", "))

	for nodeName, endpoint := range nodesMap {
		// Equivalent to `kubectl get --raw "/api/v1/nodes/<nodeName>/proxy/stats/summary"`
		klog.V(2).Infof("querying: %+v\n", endpoint)
		response, err := c.Client.CoreV1().RESTClient().Get().AbsPath(endpoint).DoRaw(c.Context)
		if err != nil {
			return output, errors.Wrapf(err, "could not query endpoint %s", endpoint)
		}
		err = output.SaveResult(c.BundlePath, fmt.Sprintf("node-metrics/%s.json", nodeName), bytes.NewBuffer(response))
		if err != nil {
			klog.Errorf("failed to save node metrics for %s: %v", nodeName, err)
		}

	}
	return output, nil
}

func (c *CollectNodeMetrics) constructNodesMap() map[string]string {
	nodesMap := map[string]string{}

	if c.Collector.NodeNames == nil && c.Collector.Selector == nil {
		// If no node names or selectors are provided, collect all nodes
		nodes, err := c.Client.CoreV1().Nodes().List(c.Context, metav1.ListOptions{})
		if err != nil {
			klog.Errorf("failed to list nodes: %v", err)
		}
		for _, node := range nodes.Items {
			nodesMap[node.Name] = fmt.Sprintf(summaryUrlTemplate, node.Name)
		}
		return nodesMap
	}

	for _, nodeName := range c.Collector.NodeNames {
		nodesMap[nodeName] = fmt.Sprintf(summaryUrlTemplate, nodeName)
	}

	// Find nodes by label selector
	if c.Collector.Selector != nil {
		nodes, err := c.Client.CoreV1().Nodes().List(c.Context, metav1.ListOptions{
			LabelSelector: strings.Join(c.Collector.Selector, ","),
		})
		if err != nil {
			klog.Errorf("failed to list nodes by label selector: %v", err)
		}
		for _, node := range nodes.Items {
			nodesMap[node.Name] = fmt.Sprintf(summaryUrlTemplate, node.Name)
		}
	}

	return nodesMap
}
