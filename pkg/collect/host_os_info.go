package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	osutils "github.com/shirou/gopsutil/v3/host"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type HostOSInfo struct {
	Name            string `json:"name"`
	KernelVersion   string `json:"kernelVersion"`
	PlatformVersion string `json:"platformVersion"`
	Platform        string `json:"platform"`
}

type HostOSInfoNodes struct {
	Nodes []string `json:"nodes"`
}

const HostOSInfoPath = `host-collectors/system/hostos_info.json`
const NodeInfoBaseDir = `host-collectors/system`
const HostInfoFileName = `hostos_info.json`

type CollectHostOS struct {
	hostCollector *troubleshootv1beta2.HostOS
	BundlePath    string
	ClientConfig  *rest.Config
	Image         string
	PullPolicy    string
	LabelSelector string
	Namespace     string
	Timeout       time.Duration
	NamePrefix    string
	RBACErrors    []error
}

func (c *CollectHostOS) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Host OS Info")
}

func (c *CollectHostOS) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostOS) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	infoStat, err := osutils.Info()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get os info")
	}
	hostInfo := HostOSInfo{}
	hostInfo.Platform = infoStat.Platform
	hostInfo.KernelVersion = infoStat.KernelVersion
	hostInfo.Name = infoStat.Hostname
	hostInfo.PlatformVersion = infoStat.PlatformVersion

	b, err := json.MarshalIndent(hostInfo, "", " ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal host os info")
	}

	output := NewResult()
	output.SaveResult(c.BundlePath, HostOSInfoPath, bytes.NewBuffer(b))

	return output, nil
}

func (c *CollectHostOS) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	allCollectedData := make(map[string][]byte)

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, errors.Wrap(err, "failed to add runtime scheme")
	}

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return nil, err
	}

	runner := &podRunner{
		client:       client,
		scheme:       scheme,
		image:        c.Image,
		pullPolicy:   c.PullPolicy,
		waitInterval: remoteCollectorDefaultInterval,
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.Timeout)
	defer cancel()

	// Get all the nodes where we should run.
	nodes, err := listNodesNamesInSelector(ctx, client, c.LabelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the list of nodes matching a nodeSelector")
	}

	if c.NamePrefix == "" {
		c.NamePrefix = remoteCollectorNamePrefix
	}

	// empty host collect var
	hostCollector := &troubleshootv1beta2.HostCollect{
		HostOS: c.hostCollector,
	}

	result, err := runRemote(ctx, runner, nodes, hostCollector, names.SimpleNameGenerator, c.NamePrefix, c.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run collector remotely")
	}

	for k, v := range result {
		if curBytes, ok := allCollectedData[k]; ok {
			var curResults map[string]string
			if err := json.Unmarshal(curBytes, &curResults); err != nil {
				progressChan <- errors.Errorf("failed to read existing results for collector %s: %v\n", c.Title(), err)
				continue
			}
			var newResults map[string]string
			if err := json.Unmarshal(v, &newResults); err != nil {
				progressChan <- errors.Errorf("failed to read new results for collector %s: %v\n", c.Title(), err)
				continue
			}
			for file, data := range newResults {
				curResults[file] = data
			}
			combinedResults, err := json.Marshal(curResults)
			if err != nil {
				progressChan <- errors.Errorf("failed to combine results for collector %s: %v\n", c.Title(), err)
				continue
			}
			allCollectedData[k] = combinedResults
		} else {
			allCollectedData[k] = v
		}

	}

	output := NewResult()

	// save the first result we find in the node and save it
	for node, result := range allCollectedData {
		var nodeResult map[string]string
		if err := json.Unmarshal(result, &nodeResult); err != nil {
			return nil, errors.Wrap(err, "failed to marshal node results")
		}

		for _, collectorResult := range nodeResult {
			var collectedItems HostOSInfo
			if err := json.Unmarshal([]byte(collectorResult), &collectedItems); err != nil {
				return nil, errors.Wrap(err, "failed to marshal collector results")
			}

			b, err := json.MarshalIndent(collectedItems, "", " ")
			if err != nil {
				return nil, errors.Wrap(err, "failed to marshal host os info")
			}
			nodes = append(nodes, node)
			output.SaveResult(c.BundlePath, fmt.Sprintf("host-collectors/system/%s/%s", node, HostInfoFileName), bytes.NewBuffer(b))
		}
	}

	// check if NODE_LIST_FILE exists
	_, err = os.Stat(NODE_LIST_FILE)
	// if it not exists, save the nodes list
	if err != nil {
		nodesBytes, err := json.MarshalIndent(HostOSInfoNodes{Nodes: nodes}, "", " ")
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal host os info nodes")
		}
		output.SaveResult(c.BundlePath, NODE_LIST_FILE, bytes.NewBuffer(nodesBytes))
	}
	return output, nil
}

/*func (c *CollectHostOS) RemoteHostCollect(progressChan chan<- interface{}) (map[string][]byte, error) {

	restConfig, err := k8sutil.GetRESTConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert kube flags to rest config")
	}

	createOpts := CollectorRunOpts{
		KubernetesRestConfig: restConfig,
		Image:                "replicated/troubleshoot:latest",
		Namespace:            "default",
		Timeout:              defaultTimeout,
		NamePrefix:           "hostos-remote",
		ProgressChan:         progressChan,
	}

	remoteCollector := &troubleshootv1beta2.RemoteCollector{
		Spec: troubleshootv1beta2.RemoteCollectorSpec{
			Collectors: []*troubleshootv1beta2.RemoteCollect{
				{
					HostOS: &troubleshootv1beta2.RemoteHostOS{},
				},
			},
		},
	}
	// empty redactor for now
	additionalRedactors := &troubleshootv1beta2.Redactor{}
	// re-use the collect.CollectRemote function to run the remote collector
	results, err := CollectRemote(remoteCollector, additionalRedactors, createOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run remote collector")
	}

	output := NewResult()
	nodes := []string{}

	// save the first result we find in the node and save it
	for node, result := range results.AllCollectedData {
		var nodeResult map[string]string
		if err := json.Unmarshal(result, &nodeResult); err != nil {
			return nil, errors.Wrap(err, "failed to marshal node results")
		}

		for _, collectorResult := range nodeResult {
			var collectedItems HostOSInfo
			if err := json.Unmarshal([]byte(collectorResult), &collectedItems); err != nil {
				return nil, errors.Wrap(err, "failed to marshal collector results")
			}

			b, err := json.MarshalIndent(collectedItems, "", " ")
			if err != nil {
				return nil, errors.Wrap(err, "failed to marshal host os info")
			}
			nodes = append(nodes, node)
			output.SaveResult(c.BundlePath, fmt.Sprintf("host-collectors/system/%s/%s", node, HostInfoFileName), bytes.NewBuffer(b))
		}
	}

	// check if NODE_LIST_FILE exists
	_, err = os.Stat(NODE_LIST_FILE)
	// if it not exists, save the nodes list
	if err != nil {
		nodesBytes, err := json.MarshalIndent(HostOSInfoNodes{Nodes: nodes}, "", " ")
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal host os info nodes")
		}
		output.SaveResult(c.BundlePath, NODE_LIST_FILE, bytes.NewBuffer(nodesBytes))
	}
	return output, nil
}*/

/*func (c *CollectHostOS) CheckRBAC(ctx context.Context) error {
	if _, err := c.IsExcluded(); err != nil {
		return nil // excluded collectors require no permissions
	}

	client, err := kubernetes.NewForConfig(c.ClientConfig)
	if err != nil {
		return errors.Wrap(err, "failed to create client from config")
	}

	forbidden := make([]error, 0)

	specs := c.hostCollector.AccessReviewSpecs(c.Namespace)
	for _, spec := range specs {
		sar := &authorizationv1.SelfSubjectAccessReview{
			Spec: spec,
		}

		resp, err := client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
		if err != nil {
			return errors.Wrap(err, "failed to run subject review")
		}

		if !resp.Status.Allowed { // all other fields of Status are empty...
			forbidden = append(forbidden, RBACError{
				DisplayName: c.Title(),
				Namespace:   spec.ResourceAttributes.Namespace,
				Resource:    spec.ResourceAttributes.Resource,
				Verb:        spec.ResourceAttributes.Verb,
			})
		}
	}
	c.RBACErrors = forbidden

	return nil
}*/
