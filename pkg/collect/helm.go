package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/k8sutil"
	"helm.sh/helm/v3/pkg/action"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
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

type configGetter struct {
	restConfig *rest.Config
}

// ToDiscoveryClient implements genericclioptions.RESTClientGetter.
func (c configGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(c.restConfig)
	if err != nil {
		return nil, err
	}
	cached := memory.NewMemCacheClient(discoveryClient)
	return cached, nil
}

// ToRESTConfig implements genericclioptions.RESTClientGetter.
func (c configGetter) ToRESTConfig() (*rest.Config, error) {
	return c.restConfig, nil
}

// ToRESTMapper implements genericclioptions.RESTClientGetter.
func (c configGetter) ToRESTMapper() (meta.RESTMapper, error) {
	discoveryClient, err := c.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discoveryClient)
	return mapper, nil
}

// ToRawKubeConfigLoader implements genericclioptions.RESTClientGetter.
func (c configGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return k8sutil.GetKubeconfig()
}

var _ genericclioptions.RESTClientGetter = configGetter{}

func (c *CollectHelm) Title() string {
	return getCollectorName(c)
}

func (c *CollectHelm) IsExcluded() (bool, error) {
	return isExcluded(c.Collector.Exclude)
}

func (c *CollectHelm) Collect(progressChan chan<- interface{}) (CollectorResult, error) {

	output := NewResult()

	releaseInfos, err := helmReleaseHistoryCollector(c.ClientConfig, c.Collector.ReleaseName, c.Collector.Namespace, c.Collector.CollectValues)
	if err != nil {
		errsToMarhsal := []string{}
		for _, e := range err {
			errsToMarhsal = append(errsToMarhsal, e.Error())
		}
		output.SaveResult(c.BundlePath, "helm/errors.json", marshalErrors(errsToMarhsal))
		klog.Errorf("error collecting helm release info: %v", err)
		return output, nil
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

		output.SaveResult(c.BundlePath, filePath, bytes.NewBuffer(helmHistoryJson))
	}

	return output, nil
}

func helmReleaseHistoryCollector(config *rest.Config, releaseName string, namespace string, collectValues bool) ([]ReleaseInfo, []error) {
	var results []ReleaseInfo
	error_list := []error{}

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(configGetter{config}, namespace, "", klog.V(2).Infof); err != nil {
		return nil, []error{err}
	}

	// If releaseName is specified, get the history of that release
	if releaseName != "" {
		getAction := action.NewGet(actionConfig)
		r, err := getAction.Run(releaseName)
		if err != nil {
			return nil, []error{err}
		}
		versionInfo, err := getVersionInfo(actionConfig, r.Name, r.Namespace, collectValues)
		if err != nil {
			return nil, []error{err}
		}
		results = append(results, ReleaseInfo{
			ReleaseName:  r.Name,
			Chart:        r.Chart.Metadata.Name,
			ChartVersion: r.Chart.Metadata.Version,
			AppVersion:   r.Chart.Metadata.AppVersion,
			Namespace:    r.Namespace,
			VersionInfo:  versionInfo,
		})
		return results, nil
	}

	// If releaseName is not specified, get the history of all releases
	// If namespace is specified, get the history of all releases in that namespace
	// If namespace is not specified, get the history of all releases in all namespaces
	listAction := action.NewList(actionConfig)
	releases, err := listAction.Run()
	if err != nil {
		return nil, []error{err}
	}

	for _, r := range releases {
		versionInfo, err := getVersionInfo(actionConfig, r.Name, r.Namespace, collectValues)
		if err != nil {
			error_list = append(error_list, err)
		}
		results = append(results, ReleaseInfo{
			ReleaseName:  r.Name,
			Chart:        r.Chart.Metadata.Name,
			ChartVersion: r.Chart.Metadata.Version,
			AppVersion:   r.Chart.Metadata.AppVersion,
			Namespace:    r.Namespace,
			VersionInfo:  versionInfo,
		})
	}
	if len(error_list) > 0 {
		return nil, error_list
	}
	return results, nil
}

func getVersionInfo(actionConfig *action.Configuration, releaseName, namespace string, collectValues bool) ([]VersionInfo, error) {

	versionCollect := []VersionInfo{}
	error_list := []error{}

	history, err := action.NewHistory(actionConfig).Run(releaseName)
	if err != nil {
		return nil, err
	}

	for _, release := range history {
		values := map[string]interface{}{}
		if collectValues {
			values, err = getHelmValues(releaseName, namespace, release.Version)
			if err != nil {
				error_list = append(error_list, err)
			}
		}

		versionCollect = append(versionCollect, VersionInfo{
			Revision:  strconv.Itoa(release.Version),
			Date:      release.Info.LastDeployed.String(),
			Status:    release.Info.Status.String(),
			IsPending: release.Info.Status.IsPending(),
			Values:    values,
		})
	}
	if len(error_list) > 0 {
		errs := []string{}
		for _, e := range error_list {
			errs = append(errs, e.Error())
		}
		return nil, errors.New(strings.Join(errs, "\n"))
	}
	return versionCollect, nil
}

func helmReleaseInfoByNamespaces(releaseInfo []ReleaseInfo) map[string][]ReleaseInfo {
	releaseInfoByNamespace := make(map[string][]ReleaseInfo)

	for _, r := range releaseInfo {
		releaseInfoByNamespace[r.Namespace] = append(releaseInfoByNamespace[r.Namespace], r)
	}

	return releaseInfoByNamespace
}

func getHelmValues(releaseName, namespace string, revision int) (map[string]interface{}, error) {
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(nil, namespace, "", klog.V(2).Infof); err != nil {
		return nil, err
	}
	getAction := action.NewGetValues(actionConfig)
	getAction.AllValues = true
	getAction.Version = revision
	helmValues, err := getAction.Run(releaseName)
	return helmValues, err
}
