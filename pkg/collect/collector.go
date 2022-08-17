package collect

import (
	"context"
	"strconv"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"k8s.io/client-go/rest"
)

type Collector interface {
	Title() string
	IsExcluded() (bool, error)
	CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error
	Collect(progressChan chan<- interface{}) (CollectorResult, error)
}

type Collectors []*Collector

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

func GetCollector(collector *troubleshootv1beta2.Collect, bundlePath string, namespace string, clientConfig *rest.Config) (Collector, bool) {
	switch {
	case collector.ClusterInfo != nil:
		return &CollectClusterInfo{collector.ClusterInfo, bundlePath}, true
	case collector.ClusterResources != nil:
		return &CollectClusterResources{collector.ClusterResources, bundlePath, namespace, clientConfig}, true
	/*case collector.Secret != nil:
		return &CollectSecret{collector.Secret, bundlePath}, true
	case collector.ConfigMap != nil:
		return &CollectConfigMap{collector.ConfigMap, bundlePath}, true
	case collector.Logs != nil:
		return &CollectLogs{collector.Logs, bundlePath}, true
	case collector.Run != nil:
		return &CollectRun{collector.Run, bundlePath}, true
	case collector.RunPod != nil:
		return &CollectRunPod{collector.RunPod, bundlePath}, true
	case collector.Exec != nil:
		return &CollectExec{collector.Exec, bundlePath}, true
	case collector.Data != nil:
		return &CollectData{collector.Data, bundlePath}, true
	case collector.Copy != nil:
		return &CollectCopy{collector.Copy, bundlePath}, true
	case collector.CopyFromHost != nil:
		return &CollectCopyFromHost{collector.CopyFromHost, bundlePath}, true
	case collector.HTTP != nil:
		return &CollectHTTP{collector.HTTP, bundlePath}, true
	case collector.Postgres != nil:
		return &CollectPostgres{collector.Postgres, bundlePath}, true
	case collector.Mysql != nil:
		return &CollectMysql{collector.Mysql, bundlePath}, true
	case collector.Redis != nil:
		return &CollectRedis{collector.Redis, bundlePath}, true
	case collector.Collectd != nil:
		return &CollectCollectd{collector.Collectd, bundlePath}, true
	case collector.Longhorn != nil:
		return &CollectLonghorn{collector.Longhorn, bundlePath}, true
	case collector.Registry != nil:
		return &CollectRegistry{collector.Registry, bundlePath}, true
	case collector.Sysctl != nil:
		return &CollectSysctl{collector.Sysctl, bundlePath}, true*/
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

func (cs Collectors) CheckRBAC(ctx context.Context, collector *troubleshootv1beta2.Collect) error {
	for _, c := range cs {
		if err := c.CheckRBAC(ctx, collector); err != nil {
			return errors.Wrap(err, "failed to check RBAC")
		}
	}
	return nil
}
