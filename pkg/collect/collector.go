package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Collector interface {
	Title() string
	IsExcluded() (bool, error)
	SkipRedaction() bool
	GetRBACErrors() []error
	HasRBACErrors() bool
	CheckRBAC(ctx context.Context, c Collector, collector *troubleshootv1beta2.Collect, clientConfig *rest.Config, namespace string) error
	Collect(progressChan chan<- interface{}) (CollectorResult, error)
}

type MergeableCollector interface {
	Collector
	Merge(allCollectors []Collector) ([]Collector, error)
}

//type Collectors []*Collector

func isExcluded(excludeVal *multitype.BoolOrString) (bool, error) {
	if excludeVal == nil {
		return false, nil
	}

	if excludeVal.Type == multitype.Bool {
		return excludeVal.BoolVal, nil
	}

	if excludeVal.StrVal == "" {
		return false, nil
	}

	parsed, err := strconv.ParseBool(excludeVal.StrVal)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse bool string")
	}

	return parsed, nil
}

func GetCollector(collector *troubleshootv1beta2.Collect, bundlePath string, namespace string, clientConfig *rest.Config, client kubernetes.Interface, sinceTime *time.Time) (Collector, bool) {

	ctx := context.TODO()

	var RBACErrors []error

	switch {
	case collector.ClusterInfo != nil:
		return &CollectClusterInfo{collector.ClusterInfo, bundlePath, namespace, clientConfig, RBACErrors}, true
	case collector.ClusterResources != nil:
		return &CollectClusterResources{collector.ClusterResources, bundlePath, namespace, clientConfig, RBACErrors}, true
	case collector.CustomMetrics != nil:
		return &CollectMetrics{collector.CustomMetrics, bundlePath, clientConfig, client, ctx, RBACErrors}, true
	case collector.Secret != nil:
		return &CollectSecret{collector.Secret, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.ConfigMap != nil:
		return &CollectConfigMap{collector.ConfigMap, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Logs != nil:
		return &CollectLogs{collector.Logs, bundlePath, namespace, clientConfig, client, ctx, sinceTime, RBACErrors}, true
	case collector.Run != nil:
		return &CollectRun{collector.Run, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.RunPod != nil:
		return &CollectRunPod{collector.RunPod, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.RunDaemonSet != nil:
		return &CollectRunDaemonSet{collector.RunDaemonSet, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Exec != nil:
		return &CollectExec{collector.Exec, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Data != nil:
		return &CollectData{collector.Data, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Copy != nil:
		return &CollectCopy{collector.Copy, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.CopyFromHost != nil:
		return &CollectCopyFromHost{
			Collector:        collector.CopyFromHost,
			BundlePath:       bundlePath,
			Namespace:        namespace,
			ClientConfig:     clientConfig,
			Client:           client,
			Context:          ctx,
			RetryFailedMount: true,
			RBACErrors:       RBACErrors,
		}, true
	case collector.HTTP != nil:
		return &CollectHTTP{collector.HTTP, bundlePath, namespace, clientConfig, client, RBACErrors}, true
	case collector.Postgres != nil:
		return &CollectPostgres{collector.Postgres, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Mssql != nil:
		return &CollectMssql{collector.Mssql, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Mysql != nil:
		return &CollectMysql{collector.Mysql, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Redis != nil:
		return &CollectRedis{collector.Redis, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Collectd != nil:
		return &CollectCollectd{collector.Collectd, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Ceph != nil:
		return &CollectCeph{collector.Ceph, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Longhorn != nil:
		return &CollectLonghorn{collector.Longhorn, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.RegistryImages != nil:
		return &CollectRegistry{collector.RegistryImages, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Sysctl != nil:
		return &CollectSysctl{collector.Sysctl, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Certificates != nil:
		return &CollectCertificates{collector.Certificates, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Helm != nil:
		return &CollectHelm{collector.Helm, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Goldpinger != nil:
		return &CollectGoldpinger{collector.Goldpinger, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Sonobuoy != nil:
		return &CollectSonobuoyResults{collector.Sonobuoy, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.NodeMetrics != nil:
		return &CollectNodeMetrics{collector.NodeMetrics, bundlePath, clientConfig, client, ctx, RBACErrors}, true
	case collector.DNS != nil:
		return &CollectDNS{collector.DNS, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Etcd != nil:
		return &CollectEtcd{collector.Etcd, bundlePath, clientConfig, client, ctx, RBACErrors}, true
	default:
		return nil, false
	}
}

func getCollectorName(c interface{}) string {
	var collector, name, selector string

	switch v := c.(type) {
	case *CollectClusterInfo:
		collector = "cluster-info"
	case *CollectClusterResources:
		collector = "cluster-resources"
	case *CollectMetrics:
		collector = "custom-metrics"
		name = v.Collector.CollectorName
	case *CollectSecret:
		collector = "secret"
		name = v.Collector.CollectorName
		selector = strings.Join(v.Collector.Selector, ",")
	case *CollectConfigMap:
		collector = "configmap"
		name = v.Collector.CollectorName
		selector = strings.Join(v.Collector.Selector, ",")
	case *CollectLogs:
		collector = "logs"
		name = v.Collector.CollectorName
		selector = strings.Join(v.Collector.Selector, ",")
	case *CollectRun:
		collector = "run"
		name = v.Collector.CollectorName
	case *CollectRunPod:
		collector = "run-pod"
		name = v.Collector.CollectorName
	case *CollectRunDaemonSet:
		collector = "run-daemonset"
		name = v.Collector.CollectorName
	case *CollectExec:
		collector = "exec"
		name = v.Collector.CollectorName
		selector = strings.Join(v.Collector.Selector, ",")
	case *CollectData:
		collector = "data"
		name = v.Collector.CollectorName
	case *CollectCopy:
		collector = "copy"
		name = v.Collector.CollectorName
		selector = strings.Join(v.Collector.Selector, ",")
	case *CollectCopyFromHost:
		collector = "copy-from-host"
		name = v.Collector.CollectorName
	case *CollectHTTP:
		collector = "http"
		name = v.Collector.CollectorName
	case *CollectPostgres:
		collector = "postgres"
		name = v.Collector.CollectorName
	case *CollectMssql:
		collector = "mssql"
		name = v.Collector.CollectorName
	case *CollectMysql:
		collector = "mysql"
		name = v.Collector.CollectorName
	case *CollectRedis:
		collector = "redis"
		name = v.Collector.CollectorName
	case *CollectCollectd:
		collector = "collectd"
		name = v.Collector.CollectorName
	case *CollectCeph:
		collector = "ceph"
		name = v.Collector.CollectorName
	case *CollectLonghorn:
		collector = "longhorn"
		name = v.Collector.CollectorName
	case *CollectRegistry:
		collector = "registry-images"
		name = v.Collector.CollectorName
	case *CollectSysctl:
		collector = "sysctl"
		name = v.Collector.Name
	case *CollectCertificates:
		collector = "certificates"
	case *CollectHelm:
		collector = "helm"
	case *CollectGoldpinger:
		collector = "goldpinger"
	case *CollectSonobuoyResults:
		collector = "sonobuoy"
	case *CollectNodeMetrics:
		collector = "node-metrics"
	case *CollectDNS:
		collector = "dns"
	case *CollectEtcd:
		collector = "etcd"
	default:
		collector = "<none>"
	}

	if name != "" {
		return fmt.Sprintf("%s/%s", collector, name)
	}
	if selector != "" {
		return fmt.Sprintf("%s/%s", collector, selector)
	}
	return collector
}

// Ensure that the specified collector is in the list of collectors
func EnsureCollectorInList(list []*troubleshootv1beta2.Collect, collector troubleshootv1beta2.Collect) []*troubleshootv1beta2.Collect {
	for _, inList := range list {
		if collector.ClusterResources != nil && inList.ClusterResources != nil {
			return list
		}
		if collector.ClusterInfo != nil && inList.ClusterInfo != nil {
			return list
		}
	}

	return append(list, &collector)
}

// collect ClusterResources earliest in the list so the pod list does not include pods started by collectors
func EnsureClusterResourcesFirst(list []*troubleshootv1beta2.Collect) []*troubleshootv1beta2.Collect {
	sliceOfClusterResources := []*troubleshootv1beta2.Collect{}
	sliceOfOtherCollectors := []*troubleshootv1beta2.Collect{}
	for _, collector := range list {
		if collector.ClusterResources != nil {
			sliceOfClusterResources = append(sliceOfClusterResources, []*troubleshootv1beta2.Collect{collector}...)
		} else {
			sliceOfOtherCollectors = append(sliceOfOtherCollectors, []*troubleshootv1beta2.Collect{collector}...)
		}
	}
	return append(sliceOfClusterResources, sliceOfOtherCollectors...)
}

// deduplicates a list of troubleshootv1beta2.Collect objects
// marshals object to json and then uses its string value to check for uniqueness
// there is no sorting of the keys in the collect object's spec so if the spec isn't an exact match line for line as written, no dedup will occur
func DedupCollectors(allCollectors []*troubleshootv1beta2.Collect) []*troubleshootv1beta2.Collect {
	uniqueCollectors := make(map[string]bool)
	finalCollectors := []*troubleshootv1beta2.Collect{}

	for _, collector := range allCollectors {
		data, err := json.Marshal(collector)
		if err != nil {
			// return collector as is if for whatever reason it can't be marshalled into json
			finalCollectors = append(finalCollectors, collector)
		} else {
			stringData := string(data)
			if _, value := uniqueCollectors[stringData]; !value {
				uniqueCollectors[stringData] = true
				finalCollectors = append(finalCollectors, collector)
			}
		}
	}
	return finalCollectors
}

// Ensure Copy collectors are last in the list
// This is because copy collectors are expected to copy files from other collectors such as Exec, RunPod, RunDaemonSet
func EnsureCopyLast(allCollectors []Collector) []Collector {
	otherCollectors := make([]Collector, 0)
	copyCollectors := make([]Collector, 0)

	for _, collector := range allCollectors {
		if _, ok := collector.(*CollectCopy); ok {
			copyCollectors = append(copyCollectors, collector)
		} else {
			otherCollectors = append(otherCollectors, collector)
		}
	}

	return append(otherCollectors, copyCollectors...)
}
