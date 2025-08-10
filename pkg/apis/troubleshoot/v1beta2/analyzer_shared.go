package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/replicatedhq/troubleshoot/pkg/multitype"
)

type ClusterVersion struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type StorageClass struct {
	AnalyzeMeta      `json:",inline" yaml:",inline"`
	Outcomes         []*Outcome `json:"outcomes" yaml:"outcomes"`
	StorageClassName string     `json:"storageClassName,omitempty" yaml:"storageClassName,omitempty"`
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

type ClusterResource struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
	Kind          string     `json:"kind" yaml:"kind"`
	ClusterScoped bool       `json:"clusterScoped" yaml:"clusterScoped"`
	Namespace     string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Name          string     `json:"name" yaml:"name"`
	YamlPath      string     `json:"yamlPath" yaml:"yamlPath"`
	ExpectedValue string     `json:"expectedValue,omitempty" yaml:"expectedValue,omitempty"`
	RegexPattern  string     `json:"regex,omitempty" yaml:"regex,omitempty"`
	RegexGroups   string     `json:"regexGroups,omitempty" yaml:"regexGroups,omitempty"`
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

type ClusterContainerStatuses struct {
	AnalyzeMeta  `json:",inline" yaml:",inline"`
	Outcomes     []*Outcome `json:"outcomes" yaml:"outcomes"`
	Namespaces   []string   `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	RestartCount int32      `json:"restartCount" yaml:"restartCount"`
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
	CPUArchitecture             string                 `json:"cpuArchitecture,omitempty" yaml:"cpuArchitecture,omitempty"`
	CPUCapacity                 string                 `json:"cpuCapacity,omitempty" yaml:"cpuCapacity,omitempty"`
	CPUAllocatable              string                 `json:"cpuAllocatable,omitempty" yaml:"cpuAllocatable,omitempty"`
	MemoryCapacity              string                 `json:"memoryCapacity,omitempty" yaml:"memoryCapacity,omitempty"`
	MemoryAllocatable           string                 `json:"memoryAllocatable,omitempty" yaml:"memoryAllocatable,omitempty"`
	PodCapacity                 string                 `json:"podCapacity,omitempty" yaml:"podCapacity,omitempty"`
	PodAllocatable              string                 `json:"podAllocatable,omitempty" yaml:"podAllocatable,omitempty"`
	EphemeralStorageCapacity    string                 `json:"ephemeralStorageCapacity,omitempty" yaml:"ephemeralStorageCapacity,omitempty"`
	EphemeralStorageAllocatable string                 `json:"ephemeralStorageAllocatable,omitempty" yaml:"ephemeralStorageAllocatable,omitempty"`
	Selector                    *NodeResourceSelectors `json:"selector,omitempty" yaml:"selector,omitempty"`
	ResourceName                string                 `json:"resourceName,omitempty" yaml:"resourceName,omitempty"`
	ResourceAllocatable         string                 `json:"resourceAllocatable,omitempty" yaml:"resourceAllocatable,omitempty"`
	ResourceCapacity            string                 `json:"resourceCapacity,omitempty" yaml:"resourceCapacity,omitempty"`
}

type NodeResourceSelectors struct {
	MatchLabel       map[string]string                 `json:"matchLabel,omitempty" yaml:"matchLabel,omitempty"`
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty" yaml:"matchExpressions,omitempty"`
}

type TextAnalyze struct {
	AnalyzeMeta     `json:",inline" yaml:",inline"`
	CollectorName   string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	FileName        string     `json:"fileName,omitempty" yaml:"fileName,omitempty"`
	RegexPattern    string     `json:"regex,omitempty" yaml:"regex,omitempty"`
	RegexGroups     string     `json:"regexGroups,omitempty" yaml:"regexGroups,omitempty"`
	IgnoreIfNoFiles bool       `json:"ignoreIfNoFiles,omitempty" yaml:"ignoreIfNoFiles,omitempty"`
	Outcomes        []*Outcome `json:"outcomes" yaml:"outcomes"`
	ExcludeFiles    []string   `json:"excludeFiles,omitempty" yaml:"excludeFiles,omitempty"`
}

type YamlCompare struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	FileName      string     `json:"fileName,omitempty" yaml:"fileName,omitempty"`
	Path          string     `json:"path,omitempty" yaml:"path,omitempty"`
	Value         string     `json:"value,omitempty" yaml:"value,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type JsonCompare struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	FileName      string     `json:"fileName,omitempty" yaml:"fileName,omitempty"`
	Path          string     `json:"path,omitempty" yaml:"path,omitempty"`
	JsonPath      string     `json:"jsonPath,omitempty" yaml:"jsonPath,omitempty"`
	Value         string     `json:"value,omitempty" yaml:"value,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
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

type VeleroAnalyze struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
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
	CheckName   string                  `json:"checkName,omitempty" yaml:"checkName,omitempty"`
	Exclude     *multitype.BoolOrString `json:"exclude,omitempty" yaml:"exclude,omitempty"`
	Strict      *multitype.BoolOrString `json:"strict,omitempty" yaml:"strict,omitempty"`
	Annotations map[string]string       `json:"annotations,omitempty" yaml:"annotations,omitempty"`
}

type CertificatesAnalyze struct {
	AnalyzeMeta `json:",inline" yaml:",inline"`
	Outcomes    []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type GoldpingerAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	Outcomes      []*Outcome `json:"outcomes,omitempty" yaml:"outcomes,omitempty"`
	CollectorName string     `json:"collectorName" yaml:"collectorName"`
	FilePath      string     `json:"filePath,omitempty" yaml:"filePath,omitempty"`
}

type EventAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string     `json:"collectorName" yaml:"collectorName"`
	Namespace     string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Kind          string     `json:"kind,omitempty" yaml:"kind,omitempty"`
	Reason        string     `json:"reason" yaml:"reason"`
	RegexPattern  string     `json:"regex,omitempty" yaml:"regex,omitempty"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
}

type NodeMetricsAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	CollectorName string                    `json:"collectorName" yaml:"collectorName"`
	Filters       NodeMetricsAnalyzeFilters `json:"filters,omitempty" yaml:"filters,omitempty"`
	Outcomes      []*Outcome                `json:"outcomes" yaml:"outcomes"`
}

type NodeMetricsAnalyzeFilters struct {
	PVC *PVCRef `json:"pvc,omitempty" yaml:"pvc,omitempty"`
}

type PVCRef struct {
	NameRegex string `json:"nameRegex,omitempty" yaml:"nameRegex,omitempty"`
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
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
	ClusterContainerStatuses *ClusterContainerStatuses `json:"clusterContainerStatuses,omitempty" yaml:"clusterContainerStatuses,omitempty"`
	ContainerRuntime         *ContainerRuntime         `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`
	Distribution             *Distribution             `json:"distribution,omitempty" yaml:"distribution,omitempty"`
	NodeResources            *NodeResources            `json:"nodeResources,omitempty" yaml:"nodeResources,omitempty"`
	TextAnalyze              *TextAnalyze              `json:"textAnalyze,omitempty" yaml:"textAnalyze,omitempty"`
	YamlCompare              *YamlCompare              `json:"yamlCompare,omitempty" yaml:"yamlCompare,omitempty"`
	JsonCompare              *JsonCompare              `json:"jsonCompare,omitempty" yaml:"jsonCompare,omitempty"`
	Postgres                 *DatabaseAnalyze          `json:"postgres,omitempty" yaml:"postgres,omitempty"`
	Mssql                    *DatabaseAnalyze          `json:"mssql,omitempty" yaml:"mssql,omitempty"`
	Mysql                    *DatabaseAnalyze          `json:"mysql,omitempty" yaml:"mysql,omitempty"`
	Redis                    *DatabaseAnalyze          `json:"redis,omitempty" yaml:"redis,omitempty"`
	CephStatus               *CephStatusAnalyze        `json:"cephStatus,omitempty" yaml:"cephStatus,omitempty"`
	Velero                   *VeleroAnalyze            `json:"velero,omitempty" yaml:"velero,omitempty"`
	Longhorn                 *LonghornAnalyze          `json:"longhorn,omitempty" yaml:"longhorn,omitempty"`
	RegistryImages           *RegistryImagesAnalyze    `json:"registryImages,omitempty" yaml:"registryImages,omitempty"`
	WeaveReport              *WeaveReportAnalyze       `json:"weaveReport,omitempty" yaml:"weaveReport,omitempty"`
	Sysctl                   *SysctlAnalyze            `json:"sysctl,omitempty" yaml:"sysctl,omitempty"`
	ClusterResource          *ClusterResource          `json:"clusterResource,omitempty" yaml:"clusterResource,omitempty"`
	Certificates             *CertificatesAnalyze      `json:"certificates,omitempty" yaml:"certificates,omitempty"`
	Goldpinger               *GoldpingerAnalyze        `json:"goldpinger,omitempty" yaml:"goldpinger,omitempty"`
	Event                    *EventAnalyze             `json:"event,omitempty" yaml:"event,omitempty"`
	NodeMetrics              *NodeMetricsAnalyze       `json:"nodeMetrics,omitempty" yaml:"nodeMetrics,omitempty"`
	HTTP                     *HTTPAnalyze              `json:"http,omitempty" yaml:"http,omitempty"`
	LLM                      *LLMAnalyze               `json:"llm,omitempty" yaml:"llm,omitempty"`
}

type LLMAnalyze struct {
	AnalyzeMeta   `json:",inline" yaml:",inline"`
	Outcomes      []*Outcome `json:"outcomes" yaml:"outcomes"`
	CollectorName string     `json:"collectorName,omitempty" yaml:"collectorName,omitempty"`
	FileName      string     `json:"fileName,omitempty" yaml:"fileName,omitempty"`
	MaxFiles      int        `json:"maxFiles,omitempty" yaml:"maxFiles,omitempty"`
	MaxSize       int        `json:"maxSize,omitempty" yaml:"maxSize,omitempty"`
	Model         string     `json:"model,omitempty" yaml:"model,omitempty"`
}
