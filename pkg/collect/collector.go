package collect

import (
	"context"
	"strconv"
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
	GetRBACErrors() []error
	HasRBACErrors() bool
	CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error
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

func GetCollector(collector *troubleshootv1beta2.Collect, bundlePath string, namespace string, clientConfig *rest.Config, client kubernetes.Interface, sinceTime *time.Time, RBACErrors []error) (interface{}, bool) {

	ctx := context.TODO()

	switch {
	case collector.ClusterInfo != nil:
		return &CollectClusterInfo{collector.ClusterInfo, bundlePath, namespace, clientConfig, RBACErrors}, true
	case collector.ClusterResources != nil:
		return &CollectClusterResources{collector.ClusterResources, bundlePath, namespace, clientConfig, RBACErrors}, true
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
	case collector.Exec != nil:
		return &CollectExec{collector.Exec, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Data != nil:
		return &CollectData{collector.Data, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.Copy != nil:
		return &CollectCopy{collector.Copy, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.CopyFromHost != nil:
		return &CollectCopyFromHost{collector.CopyFromHost, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
	case collector.HTTP != nil:
		return &CollectHTTP{collector.HTTP, bundlePath, namespace, clientConfig, client, RBACErrors}, true
	case collector.Postgres != nil:
		return &CollectPostgres{collector.Postgres, bundlePath, namespace, clientConfig, client, ctx, RBACErrors}, true
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
	default:
		return nil, false
	}
}

func collectorTitleOrDefault(meta troubleshootv1beta2.CollectorMeta, defaultTitle string) string {
	if meta.CollectorName != "" {
		return meta.CollectorName
	}
	return defaultTitle
}
