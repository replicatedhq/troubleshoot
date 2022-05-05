package v1beta2

import (
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

type ClusterVersion struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type StorageClass struct {
	AnalyzeMeta      `json:",inline" yaml:",inline"`
	Outcomes         []*Outcome `json:"outcomes" yaml:"outcomes"`
	StorageClassName string     `json:"storageClassName" yaml:"storageClassName"`
}

type CustomResourceDefinition struct {
	AnalyzeMeta                  `json:",inline" yaml:",inline"`
	Outcomes                     []*Outcome `json:"outcomes" yaml:"outcomes"`
	CustomResourceDefinitionName string     `json:"customResourceDefinitionName" yaml:"customResourceDefinitionName"`
}

type Ingress struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
	IngressName string     `json:"ingressName" yaml:"ingressName"`
	Namespace   string     `json:"namespace" yaml:"namespace"`
}

type AnalyzeSecret struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
	SecretName  string     `json:"secretName" yaml:"secretName"`
	Namespace   string     `json:"namespace" yaml:"namespace"`
	Key         string     `json:"key,omitempty" yaml:"key,omitempty"`
}

type AnalyzeConfigMap struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
	ConfigMapName string     `json:"configMapName" yaml:"configMapName"`
	Namespace     string     `json:"namespace" yaml:"namespace"`
	Key           string     `json:"key,omitempty" yaml:"key,omitempty"`
}

type ImagePullSecret struct {
	AnalyzeMeta  `json:",inline" yaml:",inline"`
	Outcomes     []*Outcome `json:"outcomes" yaml:"outcomes"`
	RegistryName string     `json:"registryName" yaml:"registryName"`
}

type DeploymentStatus struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
	Namespace   string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Namespaces  []string   `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	Name        string     `json:"name" yaml:"name"`
}

type StatefulsetStatus struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
	Namespace   string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Namespaces  []string   `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	Name        string     `json:"name" yaml:"name"`
}

type JobStatus struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
	Namespace   string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Namespaces  []string   `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	Name        string     `json:"name" yaml:"name"`
}

type ReplicaSetStatus struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
	Namespace   string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Namespaces  []string   `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	Name        string     `json:"name" yaml:"name"`
	Selector    []string   `json:"selector" yaml:"selector"`
}

type ClusterPodStatuses struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
	Namespaces  []string   `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
}

type ContainerRuntime struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type Distribution struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type NodeResources struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome           `json:"outcomes" yaml:"outcomes"`
	Filters     *NodeResourceFilters `json:"filters,omitempty" yaml:"filters,omitempty"`
}

type NodeResourceFilters struct {
	CPUCapacity                 string                 `json:"cpuCapacity,omitempty" yaml:"cpuCapacity,omitempty"`
	CPUAllocatable              string                 `json:"cpuAllocatable,omitempty" yaml:"cpuAllocatable,omitempty"`
	MemoryCapacity              string                 `json:"memoryCapacity,omitempty" yaml:"memoryCapacity,omitempty"`
	MemoryAllocatable           string                 `json:"memoryAllocatable,omitempty" yaml:"memoryAllocatable,omitempty"`
	PodCapacity                 string                 `json:"podCapacity,omitempty" yaml:"podCapacity,omitempty"`
	PodAllocatable              string                 `json:"podAllocatable,omitempty" yaml:"podAllocatable,omitempty"`
	EphemeralStorageCapacity    string                 `json:"ephemeralStorageCapacity,omitempty" yaml:"ephemeralStorageCapacity,omitempty"`
	EphemeralStorageAllocatable string                 `json:"ephemeralStorageAllocatable,omitempty" yaml:"ephemeralStorageAllocatable,omitempty"`
	Selector                    *NodeResourceSelectors `json:"selector,omitempty" yaml:"selector,omitempty"`
}

type NodeResourceSelectors struct {
	MatchLabel map[string]string `json:"matchLabel,omitempty" yaml:"matchLabel,omitempty"`
}

type TextAnalyze struct {
	AnalyzeMeta     `json:",inline" yaml:",inline"`
	CollectorName   string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	FileName        string     `json:"fileName,omitempty" yaml:"fileName,omitempty"`
	RegexPattern    string     `json:"regex,omitempty" yaml:"regex,omitempty"`
	RegexGroups     string     `json:"regexGroups,omitempty" yaml:"regexGroups,omitempty"`
	IgnoreIfNoFiles bool       `json:"ignoreIfNoFiles,omitempty" yaml:"ignoreIfNoFiles,omitempty"`
	Outcomes        []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type DatabaseAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
	CollectorName string     `json:"collectorName" yaml:"collectorName"`
	FileName      string     `json:"fileName,omitempty" yaml:"fileName,omitempty"`
}

type CollectdAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
	CollectorName string     `json:"collectorName" yaml:"collectorName"`
}

type CephStatusAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Namespace     string     `json:"namespace" yaml:"namespace"`
}

type LonghornAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Namespace     string     `json:"namespace" yaml:"namespace"`
}

type WeaveReportAnalyze struct {
	AnalyzeMeta    `json:",inline" yaml:",inline"`
	ReportFileGlob string `json:"reportFileGlob" yaml:"reportFileGlob,omitempty"`
}

type RegistryImagesAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
	CollectorName string     `json:"collectorName" yaml:"collectorName"`
}

type SysctlAnalyze struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type AnalyzeMeta struct {
	CheckName string                  `json:"checkName,omitempty" yaml:"checkName,omitempty"`
	Exclude   *multitype.BoolOrString `json:"exclude,omitempty" yaml:"exclude,omitempty"`
	Strict    *multitype.BoolOrString `json:"strict,omitempty" yaml:"strict,omitempty"`
}

type Analyze struct {
	ClusterVersion           *ClusterVersion           `json:"clusterVersion,omitempty" yaml:"clusterVersion,omitempty"`
	StorageClass             *StorageClass             `json:"storageClass,omitempty" yaml:"storageClass,omitempty"`
	CustomResourceDefinition *CustomResourceDefinition `json:"customResourceDefinition,omitempty" yaml:"customResourceDefinition,omitempty"`
	Ingress                  *Ingress                  `json:"ingress,omitempty" yaml:"ingress,omitempty"`
	Secret                   *AnalyzeSecret            `json:"secret,omitempty" yaml:"secret,omitempty"`
	ConfigMap                *AnalyzeConfigMap         `json:"configMap,omitempty" yaml:"configMap,omitempty"`
	ImagePullSecret          *ImagePullSecret          `json:"imagePullSecret,omitempty" yaml:"imagePullSecret,omitempty"`
	DeploymentStatus         *DeploymentStatus         `json:"deploymentStatus,omitempty" yaml:"deploymentStatus,omitempty"`
	StatefulsetStatus        *StatefulsetStatus        `json:"statefulsetStatus,omitempty" yaml:"statefulsetStatus,omitempty"`
	JobStatus                *JobStatus                `json:"jobStatus,omitempty" yaml:"jobStatus,omitempty"`
	ReplicaSetStatus         *ReplicaSetStatus         `json:"replicasetStatus,omitempty" yaml:"replicasetStatus,omitempty"`
	ClusterPodStatuses       *ClusterPodStatuses       `json:"clusterPodStatuses,omitempty" yaml:"clusterPodStatuses,omitempty"`
	ContainerRuntime         *ContainerRuntime         `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`
	Distribution             *Distribution             `json:"distribution,omitempty" yaml:"distribution,omitempty"`
	NodeResources            *NodeResources            `json:"nodeResources,omitempty" yaml:"nodeResources,omitempty"`
	TextAnalyze              *TextAnalyze              `json:"textAnalyze,omitempty" yaml:"textAnalyze,omitempty"`
	Postgres                 *DatabaseAnalyze          `json:"postgres,omitempty" yaml:"postgres,omitempty"`
	Mysql                    *DatabaseAnalyze          `json:"mysql,omitempty" yaml:"mysql,omitempty"`
	Redis                    *DatabaseAnalyze          `json:"redis,omitempty" yaml:"redis,omitempty"`
	CephStatus               *CephStatusAnalyze        `json:"cephStatus,omitempty" yaml:"cephStatus,omitempty"`
	Longhorn                 *LonghornAnalyze          `json:"longhorn,omitempty" yaml:"longhorn,omitempty"`
	RegistryImages           *RegistryImagesAnalyze    `json:"registryImages,omitempty" yaml:"registryImages,omitempty"`
	WeaveReport              *WeaveReportAnalyze       `json:"weaveReport,omitempty" yaml:"weaveReport,omitempty"`
	Sysctl                   *SysctlAnalyze            `json:"sysctl,omitempty" yaml:"sysctl,omitempty"`
}
