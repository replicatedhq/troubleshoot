package collect

import (
	"context"
	"fmt"
	"strconv"
<<<<<<< HEAD
	"strings"
=======
>>>>>>> 7581ee864f788e3af453371e62a9a4af8d3dcd21
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

func GetCollector(collector *troubleshootv1beta2.Collect, bundlePath string, namespace string, clientConfig *rest.Config, client kubernetes.Interface, sinceTime *time.Time) (interface{}, bool) {

	ctx := context.TODO() //empty context defined prior

	//ctx background() deployed to support context timeout implementation
	const timeout = 5 //placeholder will add timout field to Logs struct.
	ctxBackground, cancel := context.WithTimeout(context.Background(), timeout*time.Second)
	defer cancel()

<<<<<<< HEAD
	var RBACErrors []error
=======
	if c.Collect.ClusterInfo != nil {
		result, err = ClusterInfo(c)
	} else if c.Collect.ClusterResources != nil {
		result, err = ClusterResources(c, c.Collect.ClusterResources)
	} else if c.Collect.Secret != nil {
		result, err = Secret(ctx, c, c.Collect.Secret, client)
	} else if c.Collect.ConfigMap != nil {
		result, err = ConfigMap(ctx, c, c.Collect.ConfigMap, client)
	} else if c.Collect.Logs != nil {
		result, err = Logs(ctxBackground, c, c.Collect.Logs)
	} else if c.Collect.Run != nil {
		result, err = Run(c, c.Collect.Run)
	} else if c.Collect.RunPod != nil {
		result, err = RunPod(c, c.Collect.RunPod)
	} else if c.Collect.Exec != nil {
		result, err = Exec(c, c.Collect.Exec)
	} else if c.Collect.Data != nil {
		result, err = Data(c, c.Collect.Data)
	} else if c.Collect.Copy != nil {
		result, err = Copy(c, c.Collect.Copy)
	} else if c.Collect.CopyFromHost != nil {
		namespace := c.Collect.CopyFromHost.Namespace
		if namespace == "" && c.Namespace == "" {
			kubeconfig := k8sutil.GetKubeconfig()
			namespace, _, _ = kubeconfig.Namespace()
		} else if namespace == "" {
			namespace = c.Namespace
		}
		result, err = CopyFromHost(ctx, c, c.Collect.CopyFromHost, namespace, clientConfig, client)
	} else if c.Collect.HTTP != nil {
		result, err = HTTP(c, c.Collect.HTTP)
	} else if c.Collect.Postgres != nil {
		result, err = Postgres(c, c.Collect.Postgres)
	} else if c.Collect.Mysql != nil {
		result, err = Mysql(c, c.Collect.Mysql)
	} else if c.Collect.Redis != nil {
		result, err = Redis(c, c.Collect.Redis)
	} else if c.Collect.Collectd != nil {
		// TODO: see if redaction breaks these
		namespace := c.Collect.Collectd.Namespace
		if namespace == "" && c.Namespace == "" {
			kubeconfig := k8sutil.GetKubeconfig()
			namespace, _, _ = kubeconfig.Namespace()
		} else if namespace == "" {
			namespace = c.Namespace
		}
		result, err = Collectd(ctx, c, c.Collect.Collectd, namespace, clientConfig, client)
	} else if c.Collect.Ceph != nil {
		result, err = Ceph(c, c.Collect.Ceph)
	} else if c.Collect.Longhorn != nil {
		result, err = Longhorn(c, c.Collect.Longhorn)
	} else if c.Collect.RegistryImages != nil {
		result, err = Registry(c, c.Collect.RegistryImages)
	} else if c.Collect.Sysctl != nil {
		if c.Collect.Sysctl.Namespace == "" {
			c.Collect.Sysctl.Namespace = c.Namespace
		}
		if c.Collect.Sysctl.Namespace == "" {
			kubeconfig := k8sutil.GetKubeconfig()
			namespace, _, _ := kubeconfig.Namespace()
			c.Collect.Sysctl.Namespace = namespace
		}
		result, err = Sysctl(ctx, c, client, c.Collect.Sysctl)
	} else {
		err = errors.New("no spec found to run")
		return
	}
	if err != nil {
		return
	}
>>>>>>> 7581ee864f788e3af453371e62a9a4af8d3dcd21

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

func getCollectorName(c interface{}) string {
	var collector, name, selector string

	switch v := c.(type) {
	case *CollectClusterInfo:
		collector = "cluster-info"
	case *CollectClusterResources:
		collector = "cluster-resources"
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
