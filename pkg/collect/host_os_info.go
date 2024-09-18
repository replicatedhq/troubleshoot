package collect

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	osutils "github.com/shirou/gopsutil/v3/host"
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
const HostOSInfoDir = `host-collectors/system`
const HostOSInfoJSON = `hostos_info.json`
const HostOSNodes = `host-collectors/system/hostos_info_nodes.json`

type CollectHostOS struct {
	hostCollector *troubleshootv1beta2.HostOS
	BundlePath    string
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
			output.SaveResult(c.BundlePath, fmt.Sprintf("host-collectors/system/%s/%s", node, HostOSInfoJSON), bytes.NewBuffer(b))
		}
	}

	nodesBytes, err := json.MarshalIndent(HostOSInfoNodes{Nodes: nodes}, "", " ")
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal host os info nodes")
	}
	output.SaveResult(c.BundlePath, HostOSNodes, bytes.NewBuffer(nodesBytes))
	return output, nil
}
