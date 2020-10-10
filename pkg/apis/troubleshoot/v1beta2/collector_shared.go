package v1beta2

import (
	"fmt"
	"strings"

	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	authorizationv1 "k8s.io/api/authorization/v1"
)

type CollectorMeta struct {
	CollectorName string `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	// +optional
	Exclude multitype.BoolOrString `json:"exclude,omitempty" yaml:"exclude,omitempty"`
}

type ClusterInfo struct {
	CollectorMeta `json:",inline" yaml:",inline"`
}

type ClusterResources struct {
	CollectorMeta `json:",inline" yaml:",inline"`
}

type Secret struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	SecretName    string `json:"name" yaml:"name"`
	Namespace     string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Key           string `json:"key,omitempty" yaml:"key,omitempty"`
	IncludeValue  bool   `json:"includeValue,omitempty" yaml:"includeValue,omitempty"`
}

type LogLimits struct {
	MaxAge   string `json:"maxAge,omitempty" yaml:"maxAge,omitempty"`
	MaxLines int64  `json:"maxLines,omitempty" yaml:"maxLines,omitempty"`
}

type Logs struct {
	CollectorMeta  `json:",inline" yaml:",inline"`
	Name           string     `json:"name,omitempty" yaml:"name,omitempty"`
	Selector       []string   `json:"selector" yaml:"selector"`
	Namespace      string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	ContainerNames []string   `json:"containerNames,omitempty" yaml:"containerNames,omitempty"`
	Limits         *LogLimits `json:"limits,omitempty" yaml:"omitempty"`
}

type Data struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	Data          string `json:"data" yaml:"data"`
}

type Run struct {
	CollectorMeta   `json:",inline" yaml:",inline"`
	Name            string            `json:"name,omitempty" yaml:"name,omitempty"`
	Namespace       string            `json:"namespace" yaml:"namespace"`
	Image           string            `json:"image" yaml:"image"`
	Command         []string          `json:"command,omitempty" yaml:"command,omitempty"`
	Args            []string          `json:"args,omitempty" yaml:"args,omitempty"`
	Timeout         string            `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	ImagePullPolicy string            `json:"imagePullPolicy,omitempty" yaml:"imagePullPolicy,omitempty"`
	ImagePullSecret *ImagePullSecrets `json:"imagePullSecret,omitempty" yaml:"imagePullSecret,omitempty"`
    HostNetwork     bool              `json:"hostNetwork,omitempty" yaml:"hostNetwork,omitempty"`
    HostPID         bool              `json:"hostPID,omitempty" yaml:"hostPID,omitempty"`
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
	CollectorMeta `json:",inline" yaml:",inline"`
	Name          string   `json:"name,omitempty" yaml:"name,omitempty"`
	Selector      []string `json:"selector" yaml:"selector"`
	Namespace     string   `json:"namespace" yaml:"namespace"`
	ContainerPath string   `json:"containerPath" yaml:"containerPath"`
	ContainerName string   `json:"containerName,omitempty" yaml:"containerName,omitempty"`
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
}

type Post struct {
	URL                string            `json:"url" yaml:"url"`
	InsecureSkipVerify bool              `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	Headers            map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body               string            `json:"body,omitempty" yaml:"body,omitempty"`
}

type Put struct {
	URL                string            `json:"url" yaml:"url"`
	InsecureSkipVerify bool              `json:"insecureSkipVerify,omitempty" yaml:"insecureSkipVerify,omitempty"`
	Headers            map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body               string            `json:"body,omitempty" yaml:"body,omitempty"`
}

type Database struct {
	CollectorMeta `json:",inline" yaml:",inline"`
	URI           string `json:"uri" yaml:"uri"`
}

type Collect struct {
	ClusterInfo      *ClusterInfo      `json:"clusterInfo,omitempty" yaml:"clusterInfo,omitempty"`
	ClusterResources *ClusterResources `json:"clusterResources,omitempty" yaml:"clusterResources,omitempty"`
	Secret           *Secret           `json:"secret,omitempty" yaml:"secret,omitempty"`
	Logs             *Logs             `json:"logs,omitempty" yaml:"logs,omitempty"`
	Run              *Run              `json:"run,omitempty" yaml:"run,omitempty"`
	Exec             *Exec             `json:"exec,omitempty" yaml:"exec,omitempty"`
	Data             *Data             `json:"data,omitempty" yaml:"data,omitempty"`
	Copy             *Copy             `json:"copy,omitempty" yaml:"copy,omitempty"`
	HTTP             *HTTP             `json:"http,omitempty" yaml:"http,omitempty"`
	Postgres         *Database         `json:"postgres,omitempty" yaml:"postgres,omitempty"`
	Mysql            *Database         `json:"mysql,omitempty" yaml:"mysql,omitempty"`
	Redis            *Database         `json:"redis,omitempty" yaml:"redis,omitempty"`
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
				Resource:    "Namespace",
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
				Resource:    "Node",
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
				Resource:    "CustomResourceDefinition",
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
				Resource:    "StorageClasses",
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
				Resource:    "Secret",
				Subresource: "",
				Name:        c.Secret.SecretName,
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
				Resource:    "Pod",
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
				Resource:    "Pod",
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
				Resource:    "Pod",
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
				Resource:    "Pod",
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
				Resource:    "Pod",
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
				Resource:    "Pod",
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
				Resource:    "Pod",
				Subresource: "exec",
				Name:        "",
			},
			NonResourceAttributes: nil,
		})
	} else if c.HTTP != nil {
		// NOOP
	}

	return result
}

func (c *Collect) GetName() string {
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
	if c.Exec != nil {
		collector = "exec"
		name = c.Exec.CollectorName
		selector = strings.Join(c.Exec.Selector, ",")
	}
	if c.Copy != nil {
		collector = "copy"
		name = c.Copy.CollectorName
		selector = strings.Join(c.Copy.Selector, ",")
	}
	if c.HTTP != nil {
		collector = "http"
		name = c.HTTP.CollectorName
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
