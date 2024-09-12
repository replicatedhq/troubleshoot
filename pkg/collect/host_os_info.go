package collect

import (
	"bytes"
	"encoding/json"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/common"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	osutils "github.com/shirou/gopsutil/v3/host"
)

type HostOSInfo struct {
	Name            string `json:"name"`
	KernelVersion   string `json:"kernelVersion"`
	PlatformVersion string `json:"platformVersion"`
	Platform        string `json:"platform"`
}

const HostOSInfoPath = `host-collectors/system/hostos_info.json`

type NodeInfo struct {
	HostOSInfo HostOSInfo `json:"host-collectors/system/hostos_info.json"`
}
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

	progressCh := make(chan interface{})
	defer close(progressCh)
	go func() {
		for range progressCh {
		}
	}()

	createOpts := common.CollectorRunOpts{
		KubernetesRestConfig: restConfig,
		Image:                "replicated/troubleshoot:latest",
		Namespace:            "default",
		Timeout:              defaultTimeout,
		ProgressChan:         progressCh,
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
	additionalRedactors := &troubleshootv1beta2.Redactor{}
	results, err := common.CollectRemote(remoteCollector, additionalRedactors, createOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run remote collector")
	}

	output := NewResult()

	for _, result := range results.AllCollectedData {
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
			output.SaveResult(c.BundlePath, HostOSInfoPath, bytes.NewBuffer(b))
			return output, nil
		}
	}

	return nil, errors.New("failed to find host os info")
}

func (c *CollectHostOS) HasRemoted() bool {
	return true
}
