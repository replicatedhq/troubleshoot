package collect

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	osutils "github.com/shirou/gopsutil/v3/host"
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
	params := &RemoteCollectParams{
		ProgressChan:  progressChan,
		HostCollector: &troubleshootv1beta2.HostCollect{HostOS: c.hostCollector},
		BundlePath:    c.BundlePath,
		ClientConfig:  c.ClientConfig,
		Image:         c.Image,
		PullPolicy:    c.PullPolicy,
		Timeout:       c.Timeout,
		LabelSelector: c.LabelSelector,
		NamePrefix:    c.NamePrefix,
		Namespace:     c.Namespace,
		Title:         c.Title(),
	}

	output, err := remoteHostCollect(*params)
	if err != nil {
		return nil, errors.Wrap(err, "failed to run remote host os collector")
	}
	return output, nil
}

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
