package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"

	"helm.sh/helm/v3/pkg/action"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

type CollectHelm struct {
	Collector    *troubleshootv1beta2.Helm
	BundlePath   string
	Namespace    string
	ClientConfig *rest.Config
	Client       kubernetes.Interface
	Context      context.Context
	RBACErrors
}

// Helm release information struct
type ReleaseInfo struct {
	ReleaseName  string        `json:"releaseName"`
	Chart        string        `json:"chart,omitempty"`
	ChartVersion string        `json:"chartVersion,omitempty"`
	AppVersion   string        `json:"appVersion,omitempty"`
	Namespace    string        `json:"namespace,omitempty"`
	VersionInfo  []VersionInfo `json:"releaseHistory,omitempty"`
}

// Helm release version information struct
type VersionInfo struct {
	Revision  string                 `json:"revision"`
	Date      string                 `json:"date"`
	Status    string                 `json:"status"`
	IsPending bool                   `json:"isPending,omitempty"`
	Values    map[string]interface{} `json:"values,omitempty"`
}

func (c *CollectHelm) Title() string {
	return getCollectorName(c)
}

func (c *CollectHelm) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectHelm) Collect(progressChan chan<- interface{}) (CollectorResult, error) {

	output := NewResult()

	releaseInfos, err := helmReleaseHistoryCollector(c.Collector.ReleaseName, c.Collector.Namespace, c.Collector.CollectValues)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get Helm release history")
	}

	releaseInfoByNamespace := helmReleaseInfoByNamespaces(releaseInfos)

	for namespace, releaseInfo := range releaseInfoByNamespace {

		helmHistoryJson, errJson := json.MarshalIndent(releaseInfo, "", "\t")
		if errJson != nil {
			return nil, errJson
		}

		filePath := fmt.Sprintf("helm/%s.json", namespace)
		if c.Collector.ReleaseName != "" {
			filePath = fmt.Sprintf("helm/%s/%s.json", namespace, c.Collector.ReleaseName)
		}

		err := output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(helmHistoryJson))
		if err != nil {
			return nil, err
		}
	}

	return output, nil
}

func helmReleaseHistoryCollector(releaseName string, namespace string, collectValues bool) ([]ReleaseInfo, error) {
	var results []ReleaseInfo

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(nil, namespace, "", klog.V(2).Infof); err != nil {
		return nil, errors.Wrap(err, "failed to initialize Helm action config")
	}

	// If releaseName is specified, get the history of that release
	if releaseName != "" {
		getAction := action.NewGet(actionConfig)
		r, err := getAction.Run(releaseName)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get Helm release %s", releaseName)
		}
		results = append(results, ReleaseInfo{
			ReleaseName:  r.Name,
			Chart:        r.Chart.Metadata.Name,
			ChartVersion: r.Chart.Metadata.Version,
			AppVersion:   r.Chart.Metadata.AppVersion,
			Namespace:    r.Namespace,
			VersionInfo:  getVersionInfo(actionConfig, releaseName, collectValues),
		})
		return results, nil
	}

	// If releaseName is not specified, get the history of all releases
	// If namespace is specified, get the history of all releases in that namespace
	// If namespace is not specified, get the history of all releases in all namespaces
	listAction := action.NewList(actionConfig)
	releases, err := listAction.Run()
	if err != nil {
		log.Fatalf("Failed to list Helm releases: %v", err)
	}

	for _, r := range releases {
		results = append(results, ReleaseInfo{
			ReleaseName:  r.Name,
			Chart:        r.Chart.Metadata.Name,
			ChartVersion: r.Chart.Metadata.Version,
			AppVersion:   r.Chart.Metadata.AppVersion,
			Namespace:    r.Namespace,
			VersionInfo:  getVersionInfo(actionConfig, r.Name, collectValues),
		})
	}

	return results, nil
}

func getVersionInfo(actionConfig *action.Configuration, releaseName string, collectValues bool) []VersionInfo {

	versionCollect := []VersionInfo{}

	history, _ := action.NewHistory(actionConfig).Run(releaseName)

	for _, release := range history {
		values := map[string]interface{}{}
		if collectValues {
			values = getHelmValues(actionConfig, releaseName, release.Version)
		}

		versionCollect = append(versionCollect, VersionInfo{
			Revision:  strconv.Itoa(release.Version),
			Date:      release.Info.LastDeployed.String(),
			Status:    release.Info.Status.String(),
			IsPending: release.Info.Status.IsPending(),
			Values:    values,
		})
	}
	return versionCollect
}

func helmReleaseInfoByNamespaces(releaseInfo []ReleaseInfo) map[string][]ReleaseInfo {
	releaseInfoByNamespace := make(map[string][]ReleaseInfo)

	for _, r := range releaseInfo {
		releaseInfoByNamespace[r.Namespace] = append(releaseInfoByNamespace[r.Namespace], r)
	}

	return releaseInfoByNamespace
}

func getHelmValues(actionConfig *action.Configuration, releaseName string, revision int) map[string]interface{} {
	getAction := action.NewGetValues(actionConfig)
	getAction.Version = revision
	helmValues, _ := getAction.Run(releaseName)
	return helmValues
}
