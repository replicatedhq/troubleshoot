package v1beta2

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CollectorMeta struct {
	CollectorName string `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	// +optional
	Exclude *multitype.BoolOrString `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

type ClusterInfo struct {
	CollectorMeta `json:",inline" yaml:",inline"`
}

type ClusterResources struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	Namespaces    []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	IgnoreRBAC    bool     `json:"ignoreRBAC,omitempty" yaml:"ignoreRBAC"`
}

// MetricRequest the details of the MetricValuesList to be retrieved
type MetricRequest struct {
	// Namespace for which to collect the metric values, empty for non-namespaces resources.
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// ObjectName for which to collect metric values, all resources when empty.
	// Note that for namespaced resources a Namespace has to be supplied regardless.
	ObjectName string `json:"objectName,omitempty" yaml:"objectName,omitempty"`
	// ResourceMetricName name of the MetricValueList as per the APIResourceList from
	// custom.metrics.k8s.io/v1beta1
	ResourceMetricName string `json:"resourceMetricName" yaml:"resourceMetricName"`
}

type CustomMetrics struct {
	CollectorMeta  `json:",inline" yaml:",inline"`
	MetricRequests []MetricRequest `json:"metricRequests,omitempty" yaml:"metricRequests,omitempty"`
}

type Secret struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	Name          string   `json:"name,omitempty" yaml:"name,omitempty"`
	Selector      []string `json:"selector,omitempty" yaml:"selector,omitempty"`
	Namespace     string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Key           string   `json:"key,omitempty" yaml:"key,omitempty"`
	IncludeValue  bool     `json:"includeValue,omitempty" yaml:"includeValue,omitempty"`
}

type ConfigMap struct {
	CollectorMeta  `json:",inline" yaml:",inline"`
	Name           string   `json:"name,omitempty" yaml:"name,omitempty"`
	Selector       []string `json:"selector,omitempty" yaml:"selector,omitempty"`
	Namespace      string   `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Key            string   `json:"key,omitempty" yaml:"key,omitempty"`
	IncludeValue   bool     `json:"includeValue,omitempty" yaml:"includeValue,omitempty"`
	IncludeAllData bool     `json:"includeAllData,omitempty" yaml:"includeAllData,omitempty"`
}

type LogLimits struct {
	MaxAge    string      `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
	MaxLines  int64       `json:"maxLines,omitempty" yaml:"maxLines,omitempty"`
	SinceTime metav1.Time `json:"sinceTime,omitempty" yaml:"sinceTime,omitempty"`
	MaxBytes  int64       `json:"maxBytes,omitempty" yaml:"maxBytes,omitempty"`
}

type Logs struct {
	CollectorMeta  `json:",inline" yaml:",inline"`
	Name           string     `json:"name,omitempty" yaml:"name,omitempty"`
	Selector       []string   `json:"selector" yaml:"selector"`
	Namespace      string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	ContainerNames []string   `json:"containerNames,omitempty" yaml:"containerNames,omitempty"`
	Limits         *LogLimits `json:"limits,omitempty" yaml:"limits,omitempty"`
}

type Data struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	Data          string `json:"data" yaml:"data"`
}

type Run struct {
	CollectorMeta      `json:",inline" yaml:",inline"`
	Name               string            `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace          string            `json:"namespace" yaml:"namespace"`
	Image              string            `json:"image" yaml:"image"`
	Command            []string          `json:"command,omitempty" yaml:"command,omitempty"`
	Args               []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Timeout            string            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	ImagePullPolicy    string            `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	ImagePullSecret    *ImagePullSecrets `json:"imagePullSecret,omitempty" yaml:"imagePullSecret,omitempty"`
	ServiceAccountName string            `json:"serviceAccountName,omitempty" yaml:"serviceAccountName,omitempty"`
}

type RunPod struct {
	CollectorMeta   `json:",inline" yaml:",inline"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace       string            `json:"namespace" yaml:"namespace"`
	Timeout         string            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	ImagePullSecret *ImagePullSecrets `json:"imagePullSecret,omitempty" yaml:"imagePullSecret,omitempty"`
	PodSpec         corev1.PodSpec    `json:"podSpec,omitempty" yaml:"podSpec,omitempty"`
}

type ImagePullSecrets struct {
	Name       string            `json:"name,omitempty" yaml:"name,omitempty"`
	Data       map[string]string `json:"data,omitempty" yaml:"data,omitempty"`
	SecretType string            `json:"type,omitempty" yaml:"type,omitempty"`
}

type Exec struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	Name          string   `json:"name,omitempty" yaml:"name,omitempty"`
	Selector      []string `json:"selector" yaml:"selector"`
	Namespace     string   `json:"namespace" yaml:"namespace"`
	ContainerName string   `json:"containerName,omitempty" yaml:"containerName,omitempty"`
	Command       []string `json:"command,omitempty" yaml:"command,omitempty"`
	Args          []string `json:"args,omitempty" yaml:"args,omitempty"`
	Timeout       string   `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type Copy struct {
	CollectorMeta  `json:",inline" yaml:",inline"`
	Name           string   `json:"name,omitempty" yaml:"name,omitempty"`
	Selector       []string `json:"selector" yaml:"selector"`
	Namespace      string   `json:"namespace" yaml:"namespace"`
	ContainerPath  string   `json:"containerPath" yaml:"containerPath"`
	ContainerName  string   `json:"containerName,omitempty" yaml:"containerName,omitempty"`
	ExtractArchive bool     `json:"extractArchive,omitempty" yaml:"extractArchive,omitempty"`
}

type CopyFromHost struct {
	CollectorMeta   `json:",inline" yaml:",inline"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace       string            `json:"namespace" yaml:"namespace"`
	Image           string            `json:"image" yaml:"image"`
	ImagePullPolicy string            `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	ImagePullSecret *ImagePullSecrets `json:"imagePullSecret,omitempty" yaml:"imagePullSecret,omitempty"`
	Timeout         string            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	HostPath        string            `json:"hostPath" yaml:"hostPath"`
	ExtractArchive  bool              `json:"extractArchive,omitempty" yaml:"extractArchive,omitempty"`
}

type Sysctl struct {
	CollectorMeta   `json:",inline" yaml:",inline"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace       string            `json:"namespace" yaml:"namespace"`
	Image           string            `json:"image" yaml:"image"`
	ImagePullPolicy string            `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	ImagePullSecret *ImagePullSecrets `json:"imagePullSecret,omitempty" yaml:"imagePullSecret,omitempty"`
	Timeout         string            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type HTTP struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	Get           *Get   `json:"get,omitempty" yaml:"get,omitempty"`
	Post          *Post  `json:"post,omitempty" yaml:"post,omitempty"`
	Put           *Put   `json:"put,omitempty" yaml:"put,omitempty"`
}

type Get struct {
	URL                string            `json:"url" yaml:"url"`
	InsecureSkipVerify bool              `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	Headers            map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Timeout            time.Duration     `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type Post struct {
	URL                string            `json:"url" yaml:"url"`
	InsecureSkipVerify bool              `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	Headers            map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body               string            `json:"body,omitempty" yaml:"body,omitempty"`
	Timeout            time.Duration     `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type Put struct {
	URL                string            `json:"url" yaml:"url"`
	InsecureSkipVerify bool              `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	Headers            map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body               string            `json:"body,omitempty" yaml:"body,omitempty"`
	Timeout            time.Duration     `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type Database struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	URI           string     `json:"uri" yaml:"uri"`
	Parameters    []string   `json:"parameters,omitempty"`
	TLS           *TLSParams `json:"tls,omitempty" yaml:"tls,omitempty"`
}

type TLSParams struct {
	SkipVerify bool       `json:"skipVerify,omitempty" yaml:"skipVerify,omitempty"`
	Secret     *TLSSecret `json:"secret,omitempty" yaml:"secret,omitempty"`
	CACert     string     `json:"cacert,omitempty" yaml:"cacert,omitempty"`
	ClientCert string     `json:"clientCert,omitempty" yaml:"clientCert,omitempty"`
	ClientKey  string     `json:"clientKey,omitempty" yaml:"clientKey,omitempty"`
}

type TLSSecret struct {
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

type Collectd struct {
	CollectorMeta   `json:",inline" yaml:",inline"`
	Namespace       string            `json:"namespace" yaml:"namespace"`
	Image           string            `json:"image" yaml:"image"`
	ImagePullPolicy string            `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	ImagePullSecret *ImagePullSecrets `json:"imagePullSecret,omitempty" yaml:"imagePullSecret,omitempty"`
	Timeout         string            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	HostPath        string            `json:"hostPath" yaml:"hostPath"`
}

type Ceph struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	Namespace     string `json:"namespace" yaml:"namespace"`
	Timeout       string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type Longhorn struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	Namespace     string `json:"namespace" yaml:"namespace"`
	Timeout       string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

type RegistryImages struct {
	CollectorMeta    `json:",inline" yaml:",inline"`
	Images           []string          `json:"images" yaml:"images"`
	Namespace        string            `json:"namespace" yaml:"namespace"`
	ImagePullSecrets *ImagePullSecrets `json:"imagePullSecret,omitempty" yaml:"imagePullSecret,omitempty"`
}

type Certificates struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	Secrets       []CertificateSource `json:"secrets,omitempty" yaml:"secrets,omitempty"`
	ConfigMaps    []CertificateSource `json:"configMaps,omitempty" yaml:"configMaps,omitempty"`
}

type CertificateSource struct {
	Name       string   `json:"name,omitempty" yaml:"name,omitempty"`
	Namespaces []string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
}

type Helm struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	Namespace     string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	ReleaseName   string `json:"releaseName,omitempty" yaml:"releaseName,omitempty"`
}

type Collect struct {
	ClusterInfo      *ClusterInfo      `json:"clusterInfo,omitempty" yaml:"clusterInfo,omitempty"`
	ClusterResources *ClusterResources `json:"clusterResources,omitempty" yaml:"clusterResources,omitempty"`
	Secret           *Secret           `json:"secret,omitempty" yaml:"secret,omitempty"`
	CustomMetrics    *CustomMetrics    `json:"customMetrics,omitempty" yaml:"customMetrics,omitempty"`
	ConfigMap        *ConfigMap        `json:"configMap,omitempty" yaml:"configMap,omitempty"`
	Logs             *Logs             `json:"logs,omitempty" yaml:"logs,omitempty"`
	Run              *Run              `json:"run,omitempty" yaml:"run,omitempty"`
	RunPod           *RunPod           `json:"runPod,omitempty" yaml:"runPod,omitempty"`
	Exec             *Exec             `json:"exec,omitempty" yaml:"exec,omitempty"`
	Data             *Data             `json:"data,omitempty" yaml:"data,omitempty"`
	Copy             *Copy             `json:"copy,omitempty" yaml:"copy,omitempty"`
	CopyFromHost     *CopyFromHost     `json:"copyFromHost,omitempty" yaml:"copyFromHost,omitempty"`
	HTTP             *HTTP             `json:"http,omitempty" yaml:"http,omitempty"`
	Postgres         *Database         `json:"postgres,omitempty" yaml:"postgres,omitempty"`
	Mssql            *Database         `json:"mssql,omitempty" yaml:"mssql,omitempty"`
	Mysql            *Database         `json:"mysql,omitempty" yaml:"mysql,omitempty"`
	Redis            *Database         `json:"redis,omitempty" yaml:"redis,omitempty"`
	Collectd         *Collectd         `json:"collectd,omitempty" yaml:"collectd,omitempty"`
	Ceph             *Ceph             `json:"ceph,omitempty" yaml:"ceph,omitempty"`
	Longhorn         *Longhorn         `json:"longhorn,omitempty" yaml:"longhorn,omitempty"`
	RegistryImages   *RegistryImages   `json:"registryImages,omitempty" yaml:"registryImages,omitempty"`
	Sysctl           *Sysctl           `json:"sysctl,omitempty" yaml:"sysctl,omitempty"`
	Certificates     *Certificates     `json:"certificates,omitempty" yaml:"certificates,omitempty"`
	Helm             *Helm             `json:"helm,omitempty" yaml:"helm,omitempty"`
}

func (c *Collect) AccessReviewSpecs(overrideNS string) []authorizationv1.SelfSubjectAccessReviewSpec {
	result := make([]authorizationv1.SelfSubjectAccessReviewSpec, 0)

	if c.ClusterInfo != nil {
		// NOOP
	} else if c.ClusterResources != nil {
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   "",
				Verb:        "list",
				Group:       "",
				Version:     "",
				Resource:    "namespaces",
				Subresource: "",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   "",
				Verb:        "list",
				Group:       "",
				Version:     "",
				Resource:    "nodes",
				Subresource: "",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   "",
				Verb:        "list",
				Group:       "apiextensions.k8s.io",
				Version:     "",
				Resource:    "customresourcedefinitions",
				Subresource: "",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   "",
				Verb:        "list",
				Group:       "storage.k8s.io",
				Version:     "",
				Resource:    "storageclasses",
				Subresource: "",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
	} else if c.Secret != nil {
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   pickNamespaceOrDefault(c.Secret.Namespace, overrideNS),
				Verb:        "get",
				Group:       "",
				Version:     "",
				Resource:    "secrets",
				Subresource: "",
				Name:        c.Secret.Name,
			},
			NonResourceAttributes: nil,
		})
	} else if c.ConfigMap != nil {
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   pickNamespaceOrDefault(c.ConfigMap.Namespace, overrideNS),
				Verb:        "get",
				Group:       "",
				Version:     "",
				Resource:    "configmaps",
				Subresource: "",
				Name:        c.ConfigMap.Name,
			},
			NonResourceAttributes: nil,
		})
	} else if c.Logs != nil {
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   pickNamespaceOrDefault(c.Logs.Namespace, overrideNS),
				Verb:        "list",
				Group:       "",
				Version:     "",
				Resource:    "pods",
				Subresource: "",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   pickNamespaceOrDefault(c.Logs.Namespace, overrideNS),
				Verb:        "get",
				Group:       "",
				Version:     "",
				Resource:    "pods",
				Subresource: "log",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
	} else if c.Run != nil {
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   pickNamespaceOrDefault(c.Run.Namespace, overrideNS),
				Verb:        "create",
				Group:       "",
				Version:     "",
				Resource:    "pods",
				Subresource: "",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
	} else if c.RunPod != nil {
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   pickNamespaceOrDefault(c.RunPod.Namespace, overrideNS),
				Verb:        "create",
				Group:       "",
				Version:     "",
				Resource:    "pods",
				Subresource: "",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
	} else if c.Exec != nil {
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   pickNamespaceOrDefault(c.Exec.Namespace, overrideNS),
				Verb:        "list",
				Group:       "",
				Version:     "",
				Resource:    "pods",
				Subresource: "",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   pickNamespaceOrDefault(c.Exec.Namespace, overrideNS),
				Verb:        "get",
				Group:       "",
				Version:     "",
				Resource:    "pods",
				Subresource: "exec",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
	} else if c.Copy != nil {
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   pickNamespaceOrDefault(c.Copy.Namespace, overrideNS),
				Verb:        "list",
				Group:       "",
				Version:     "",
				Resource:    "pods",
				Subresource: "",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   pickNamespaceOrDefault(c.Copy.Namespace, overrideNS),
				Verb:        "get",
				Group:       "",
				Version:     "",
				Resource:    "pods",
				Subresource: "exec",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
	} else if c.CopyFromHost != nil {
		// TODO
	} else if c.Collectd != nil {
		// TODO
	} else if c.HTTP != nil {
		// NOOP
	} else if c.RegistryImages != nil &&
		c.RegistryImages.ImagePullSecrets != nil &&
		c.RegistryImages.ImagePullSecrets.Data == nil {
		result = append(result, authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Namespace:   pickNamespaceOrDefault(c.RegistryImages.Namespace, overrideNS),
				Verb:        "get",
				Group:       "",
				Version:     "",
				Resource:    "secrets",
				Subresource: "",
				Name:        c.RegistryImages.ImagePullSecrets.Name,
			},
			NonResourceAttributes: nil,
		})
	} else if c.Sysctl != nil {
		// TODO
	}

	return result
}

func (c *Collect) GetName() string {
	// TODO: Is this used anywhere? Should we just remove it?
	var collector, name, selector string
	if c.ClusterInfo != nil {
		collector = "cluster-info"
	}
	if c.ClusterResources != nil {
		collector = "cluster-resources"
	}
	if c.Secret != nil {
		collector = "secret"
		name = c.Secret.CollectorName
		selector = strings.Join(c.Secret.Selector, ",")
	}
	if c.ConfigMap != nil {
		collector = "configmap"
		name = c.ConfigMap.CollectorName
		selector = strings.Join(c.ConfigMap.Selector, ",")
	}
	if c.Logs != nil {
		collector = "logs"
		name = c.Logs.CollectorName
		selector = strings.Join(c.Logs.Selector, ",")
	}
	if c.Run != nil {
		collector = "run"
		name = c.Run.CollectorName
	}
	if c.RunPod != nil {
		collector = "run-pod"
		name = c.RunPod.CollectorName
	}
	if c.Exec != nil {
		collector = "exec"
		name = c.Exec.CollectorName
		selector = strings.Join(c.Exec.Selector, ",")
	}
	if c.Data != nil {
		collector = "data"
		name = c.Data.CollectorName
	}
	if c.Copy != nil {
		collector = "copy"
		name = c.Copy.CollectorName
		selector = strings.Join(c.Copy.Selector, ",")
	}
	if c.CopyFromHost != nil {
		collector = "copy-from-host"
		name = c.CopyFromHost.CollectorName
	}
	if c.HTTP != nil {
		collector = "http"
		name = c.HTTP.CollectorName
	}
	if c.Postgres != nil {
		collector = "postgres"
		name = c.Postgres.CollectorName
	}
	if c.Mssql != nil {
		collector = "mssql"
		name = c.Mssql.CollectorName
	}
	if c.Mysql != nil {
		collector = "mysql"
		name = c.Mysql.CollectorName
	}
	if c.Redis != nil {
		collector = "redis"
		name = c.Redis.CollectorName
	}
	if c.Collectd != nil {
		collector = "collectd"
		name = c.Collectd.CollectorName
	}
	if c.Ceph != nil {
		collector = "ceph"
		name = c.Ceph.CollectorName
	}
	if c.Longhorn != nil {
		collector = "longhorn"
		name = c.Longhorn.CollectorName
	}
	if c.RegistryImages != nil {
		collector = "registry-images"
		name = c.RegistryImages.CollectorName
	}
	if c.Sysctl != nil {
		collector = "sysctl"
		name = c.Sysctl.Name
	}
	if c.Certificates != nil {
		collector = "certificates"
		name = c.Certificates.CollectorName
	}

	if collector == "" {
		return "<none>"
	}
	if name != "" {
		return fmt.Sprintf("%s/%s", collector, name)
	}
	if selector != "" {
		return fmt.Sprintf("%s/%s", collector, selector)
	}
	return collector
}

func pickNamespaceOrDefault(collectorNS string, overrideNS string) string {
	if overrideNS != "" {
		return overrideNS
	}
	if collectorNS != "" {
		return collectorNS
	}
	return "default"
}

func GetCollector(collector *Collect) interface{} {
	if collector == nil {
		return nil
	}

	reflected := reflect.ValueOf(collector).Elem()
	for i := 0; i < reflected.NumField(); i++ {
		if reflected.Field(i).IsNil() {
			continue
		}

		return reflect.Indirect(reflected.Field(i)).Addr().Interface()
	}

	return nil
}
