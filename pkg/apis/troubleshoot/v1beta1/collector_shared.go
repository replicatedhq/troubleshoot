package v1beta1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Meta struct {
	Name   string            `json:"name,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
}

type Scrub struct {
	Regex   string `json:"regex"`
	Replace string `json:"replace"`
}

type SpecShared struct {
	Description    string `json:"description,omitempty"`
	OutputDir      string `json:"output_dir,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	Scrub          *Scrub `json:"scrub,omitempty"`
	IncludeEmpty   bool   `json:"include_empty,omitempty"`
	Meta           *Meta  `json:"meta,omitempty"`
	Defer          bool   `json:"defer,omitempty"`
}

type KubernetesAPIVersionsOptions struct {
	SpecShared `json:",inline,omitempty"`
}

type KubernetesClusterInfoOptions struct {
	SpecShared `json:",inline,omitempty"`
}

type KubernetesVersionOptions struct {
	SpecShared `json:",inline,omitempty"`
}

type KubernetesLogsOptions struct {
	SpecShared    `json:",inline,omitempty"`
	Pod           string              `json:"pod,omitempty"`
	Namespace     string              `json:"namespace,omitempty"`
	PodLogOptions *v1.PodLogOptions   `json:"pod_log_options,omitempty"`
	ListOptions   *metav1.ListOptions `json:"list_options,omitempty"`
}

type KubernetesContainerCpOptions struct {
	SpecShared     `json:",inline,omitempty"`
	Pod            string              `json:"pod,omitempty"`
	PodListOptions *metav1.ListOptions `json:"pod_list_options,omitempty"`
	Container      string              `json:"container,omitempty"`
	Namespace      string              `json:"namespace,omitempty"`
	SrcPath        string              `json:"src_path,omitempty"`
}

type KubernetesResourceListOptions struct {
	SpecShared   `json:",inline,omitempty"`
	Kind         string              `json:"kind"`
	GroupVersion string              `json:"group_version,omitempty"`
	Namespace    string              `json:"namespace,omitempty"`
	ListOptions  *metav1.ListOptions `json:"resource_list_options,omitempty"`
}

type Collect struct {
	KubernetesAPIVersions  *KubernetesAPIVersionsOptions  `json:"kubernetes.api-versions,omitempty"`
	KubernetesClusterInfo  *KubernetesClusterInfoOptions  `json:"kubernetes.cluster-info,omitempty"`
	KubernetesContainerCp  *KubernetesContainerCpOptions  `json:"kubernetes.container-cp,omitempty"`
	KubernetesLogs         *KubernetesLogsOptions         `json:"kubernetes.logs,omitempty"`
	KubernetesResourceList *KubernetesResourceListOptions `json:"kubernetes.resource-list,omitempty"`
	KubernetesVersion      *KubernetesVersionOptions      `json:"kubernetes.version,omitempty"`
}
