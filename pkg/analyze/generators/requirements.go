package generators

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// RequirementSpec represents a specification for generating analyzers
type RequirementSpec struct {
	APIVersion string                 `json:"apiVersion" yaml:"apiVersion"`
	Kind       string                 `json:"kind" yaml:"kind"`
	Metadata   RequirementMetadata    `json:"metadata" yaml:"metadata"`
	Spec       RequirementSpecDetails `json:"spec" yaml:"spec"`
}

// RequirementMetadata contains metadata about the requirement specification
type RequirementMetadata struct {
	Name        string            `json:"name" yaml:"name"`
	Description string            `json:"description" yaml:"description"`
	Version     string            `json:"version" yaml:"version"`
	Vendor      string            `json:"vendor" yaml:"vendor"`
	Tags        []string          `json:"tags" yaml:"tags"`
	Labels      map[string]string `json:"labels" yaml:"labels"`
	Annotations map[string]string `json:"annotations" yaml:"annotations"`
}

// RequirementSpecDetails contains the detailed requirements
type RequirementSpecDetails struct {
	Kubernetes KubernetesRequirements `json:"kubernetes" yaml:"kubernetes"`
	Resources  ResourceRequirements   `json:"resources" yaml:"resources"`
	Storage    StorageRequirements    `json:"storage" yaml:"storage"`
	Network    NetworkRequirements    `json:"network" yaml:"network"`
	Security   SecurityRequirements   `json:"security" yaml:"security"`
	Custom     []CustomRequirement    `json:"custom" yaml:"custom"`
	Vendor     VendorRequirements     `json:"vendor" yaml:"vendor"`
	Replicated ReplicatedRequirements `json:"replicated" yaml:"replicated"`
}

// KubernetesRequirements specifies Kubernetes-related requirements
type KubernetesRequirements struct {
	MinVersion    string                    `json:"minVersion" yaml:"minVersion"`
	MaxVersion    string                    `json:"maxVersion" yaml:"maxVersion"`
	Features      []string                  `json:"features" yaml:"features"`
	APIs          []APIRequirement          `json:"apis" yaml:"apis"`
	Distributions []DistributionRequirement `json:"distributions" yaml:"distributions"`
	NodeCount     NodeCountRequirement      `json:"nodeCount" yaml:"nodeCount"`
}

// APIRequirement specifies required Kubernetes APIs
type APIRequirement struct {
	Group      string `json:"group" yaml:"group"`
	Version    string `json:"version" yaml:"version"`
	Kind       string `json:"kind" yaml:"kind"`
	Required   bool   `json:"required" yaml:"required"`
	MinVersion string `json:"minVersion" yaml:"minVersion"`
}

// DistributionRequirement specifies requirements for Kubernetes distributions
type DistributionRequirement struct {
	Name       string   `json:"name" yaml:"name"`
	Versions   []string `json:"versions" yaml:"versions"`
	Supported  bool     `json:"supported" yaml:"supported"`
	Deprecated bool     `json:"deprecated" yaml:"deprecated"`
}

// NodeCountRequirement specifies node count requirements
type NodeCountRequirement struct {
	Min         int      `json:"min" yaml:"min"`
	Max         int      `json:"max" yaml:"max"`
	Recommended int      `json:"recommended" yaml:"recommended"`
	NodeTypes   []string `json:"nodeTypes" yaml:"nodeTypes"`
}

// ResourceRequirements specifies resource-related requirements
type ResourceRequirements struct {
	CPU    CPURequirement    `json:"cpu" yaml:"cpu"`
	Memory MemoryRequirement `json:"memory" yaml:"memory"`
	Nodes  NodeRequirements  `json:"nodes" yaml:"nodes"`
}

// CPURequirement specifies CPU requirements
type CPURequirement struct {
	MinCores         float64  `json:"minCores" yaml:"minCores"`
	MaxUtilization   float64  `json:"maxUtilization" yaml:"maxUtilization"`
	RequiredFeatures []string `json:"requiredFeatures" yaml:"requiredFeatures"`
	Architecture     []string `json:"architecture" yaml:"architecture"`
}

// MemoryRequirement specifies memory requirements
type MemoryRequirement struct {
	MinBytes       int64   `json:"minBytes" yaml:"minBytes"`
	MaxUtilization float64 `json:"maxUtilization" yaml:"maxUtilization"`
	SwapAllowed    bool    `json:"swapAllowed" yaml:"swapAllowed"`
}

// NodeRequirements specifies node-level requirements
type NodeRequirements struct {
	MinNodes      int                `json:"minNodes" yaml:"minNodes"`
	MaxNodes      int                `json:"maxNodes" yaml:"maxNodes"`
	NodeSelectors map[string]string  `json:"nodeSelectors" yaml:"nodeSelectors"`
	Taints        []TaintRequirement `json:"taints" yaml:"taints"`
	Labels        []LabelRequirement `json:"labels" yaml:"labels"`
}

// TaintRequirement specifies node taint requirements
type TaintRequirement struct {
	Key      string `json:"key" yaml:"key"`
	Value    string `json:"value" yaml:"value"`
	Effect   string `json:"effect" yaml:"effect"`
	Required bool   `json:"required" yaml:"required"`
	Operator string `json:"operator" yaml:"operator"`
}

// LabelRequirement specifies node label requirements
type LabelRequirement struct {
	Key      string   `json:"key" yaml:"key"`
	Values   []string `json:"values" yaml:"values"`
	Required bool     `json:"required" yaml:"required"`
	Operator string   `json:"operator" yaml:"operator"`
}

// StorageRequirements specifies storage-related requirements
type StorageRequirements struct {
	MinCapacity    int64                     `json:"minCapacity" yaml:"minCapacity"`
	StorageClasses []StorageClassRequirement `json:"storageClasses" yaml:"storageClasses"`
	VolumeTypes    []VolumeTypeRequirement   `json:"volumeTypes" yaml:"volumeTypes"`
	Performance    StoragePerformance        `json:"performance" yaml:"performance"`
	Backup         BackupRequirement         `json:"backup" yaml:"backup"`
}

// StorageClassRequirement specifies storage class requirements
type StorageClassRequirement struct {
	Name        string            `json:"name" yaml:"name"`
	Provisioner string            `json:"provisioner" yaml:"provisioner"`
	Parameters  map[string]string `json:"parameters" yaml:"parameters"`
	Required    bool              `json:"required" yaml:"required"`
	Default     bool              `json:"default" yaml:"default"`
}

// VolumeTypeRequirement specifies volume type requirements
type VolumeTypeRequirement struct {
	Type     string `json:"type" yaml:"type"`
	Required bool   `json:"required" yaml:"required"`
	MinSize  int64  `json:"minSize" yaml:"minSize"`
	MaxSize  int64  `json:"maxSize" yaml:"maxSize"`
}

// StoragePerformance specifies storage performance requirements
type StoragePerformance struct {
	MinIOPS       int64 `json:"minIOPS" yaml:"minIOPS"`
	MinThroughput int64 `json:"minThroughput" yaml:"minThroughput"`
	Latency       int64 `json:"latency" yaml:"latency"`
}

// BackupRequirement specifies backup requirements
type BackupRequirement struct {
	Required     bool          `json:"required" yaml:"required"`
	Frequency    time.Duration `json:"frequency" yaml:"frequency"`
	Retention    time.Duration `json:"retention" yaml:"retention"`
	Destinations []string      `json:"destinations" yaml:"destinations"`
	Encryption   bool          `json:"encryption" yaml:"encryption"`
	Compression  bool          `json:"compression" yaml:"compression"`
	Validation   bool          `json:"validation" yaml:"validation"`
}

// NetworkRequirements specifies network-related requirements
type NetworkRequirements struct {
	Connectivity []ConnectivityRequirement  `json:"connectivity" yaml:"connectivity"`
	Bandwidth    BandwidthRequirement       `json:"bandwidth" yaml:"bandwidth"`
	Latency      LatencyRequirement         `json:"latency" yaml:"latency"`
	Security     NetworkSecurityRequirement `json:"security" yaml:"security"`
	DNS          DNSRequirement             `json:"dns" yaml:"dns"`
	Proxy        ProxyRequirement           `json:"proxy" yaml:"proxy"`
}

// ConnectivityRequirement specifies connectivity requirements
type ConnectivityRequirement struct {
	Type        string   `json:"type" yaml:"type"`
	Endpoints   []string `json:"endpoints" yaml:"endpoints"`
	Ports       []int    `json:"ports" yaml:"ports"`
	Protocols   []string `json:"protocols" yaml:"protocols"`
	Required    bool     `json:"required" yaml:"required"`
	Direction   string   `json:"direction" yaml:"direction"`
	Description string   `json:"description" yaml:"description"`
}

// BandwidthRequirement specifies bandwidth requirements
type BandwidthRequirement struct {
	MinUpload   int64 `json:"minUpload" yaml:"minUpload"`
	MinDownload int64 `json:"minDownload" yaml:"minDownload"`
	Burst       int64 `json:"burst" yaml:"burst"`
}

// LatencyRequirement specifies latency requirements
type LatencyRequirement struct {
	MaxRTT        time.Duration `json:"maxRTT" yaml:"maxRTT"`
	MaxJitter     time.Duration `json:"maxJitter" yaml:"maxJitter"`
	TestEndpoints []string      `json:"testEndpoints" yaml:"testEndpoints"`
}

// NetworkSecurityRequirement specifies network security requirements
type NetworkSecurityRequirement struct {
	TLSRequired     bool     `json:"tlsRequired" yaml:"tlsRequired"`
	MinTLSVersion   string   `json:"minTLSVersion" yaml:"minTLSVersion"`
	AllowedCiphers  []string `json:"allowedCiphers" yaml:"allowedCiphers"`
	CertificateAuth bool     `json:"certificateAuth" yaml:"certificateAuth"`
	NetworkPolicies bool     `json:"networkPolicies" yaml:"networkPolicies"`
}

// DNSRequirement specifies DNS requirements
type DNSRequirement struct {
	Required      bool     `json:"required" yaml:"required"`
	Servers       []string `json:"servers" yaml:"servers"`
	SearchDomains []string `json:"searchDomains" yaml:"searchDomains"`
	Resolution    int64    `json:"resolution" yaml:"resolution"`
}

// ProxyRequirement specifies proxy requirements
type ProxyRequirement struct {
	Required bool     `json:"required" yaml:"required"`
	HTTP     string   `json:"http" yaml:"http"`
	HTTPS    string   `json:"https" yaml:"https"`
	NoProxy  []string `json:"noProxy" yaml:"noProxy"`
	Auth     bool     `json:"auth" yaml:"auth"`
}

// SecurityRequirements specifies security-related requirements
type SecurityRequirements struct {
	RBAC          RBACRequirement          `json:"rbac" yaml:"rbac"`
	PodSecurity   PodSecurityRequirement   `json:"podSecurity" yaml:"podSecurity"`
	NetworkPolicy NetworkPolicyRequirement `json:"networkPolicy" yaml:"networkPolicy"`
	Encryption    EncryptionRequirement    `json:"encryption" yaml:"encryption"`
	Admission     AdmissionRequirement     `json:"admission" yaml:"admission"`
	Compliance    ComplianceRequirement    `json:"compliance" yaml:"compliance"`
}

// RBACRequirement specifies RBAC requirements
type RBACRequirement struct {
	Required       bool              `json:"required" yaml:"required"`
	Roles          []RoleRequirement `json:"roles" yaml:"roles"`
	ClusterRole    bool              `json:"clusterRole" yaml:"clusterRole"`
	ServiceAccount bool              `json:"serviceAccount" yaml:"serviceAccount"`
}

// RoleRequirement specifies role requirements
type RoleRequirement struct {
	Name     string               `json:"name" yaml:"name"`
	Rules    []RuleRequirement    `json:"rules" yaml:"rules"`
	Subjects []SubjectRequirement `json:"subjects" yaml:"subjects"`
	Required bool                 `json:"required" yaml:"required"`
}

// RuleRequirement specifies RBAC rule requirements
type RuleRequirement struct {
	APIGroups []string `json:"apiGroups" yaml:"apiGroups"`
	Resources []string `json:"resources" yaml:"resources"`
	Verbs     []string `json:"verbs" yaml:"verbs"`
	Names     []string `json:"resourceNames" yaml:"resourceNames"`
}

// SubjectRequirement specifies RBAC subject requirements
type SubjectRequirement struct {
	Kind      string `json:"kind" yaml:"kind"`
	Name      string `json:"name" yaml:"name"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

// PodSecurityRequirement specifies pod security requirements
type PodSecurityRequirement struct {
	Standards       []string `json:"standards" yaml:"standards"`
	RunAsNonRoot    bool     `json:"runAsNonRoot" yaml:"runAsNonRoot"`
	ReadOnlyRoot    bool     `json:"readOnlyRoot" yaml:"readOnlyRoot"`
	AllowPrivileged bool     `json:"allowPrivileged" yaml:"allowPrivileged"`
	Capabilities    []string `json:"capabilities" yaml:"capabilities"`
	SELinux         bool     `json:"seLinux" yaml:"seLinux"`
	AppArmor        bool     `json:"appArmor" yaml:"appArmor"`
	Seccomp         bool     `json:"seccomp" yaml:"seccomp"`
}

// NetworkPolicyRequirement specifies network policy requirements
type NetworkPolicyRequirement struct {
	Required           bool     `json:"required" yaml:"required"`
	DefaultDeny        bool     `json:"defaultDeny" yaml:"defaultDeny"`
	IngressPolicies    []string `json:"ingressPolicies" yaml:"ingressPolicies"`
	EgressPolicies     []string `json:"egressPolicies" yaml:"egressPolicies"`
	NamespaceIsolation bool     `json:"namespaceIsolation" yaml:"namespaceIsolation"`
}

// EncryptionRequirement specifies encryption requirements
type EncryptionRequirement struct {
	AtRest        EncryptionAtRest         `json:"atRest" yaml:"atRest"`
	InTransit     EncryptionInTransit      `json:"inTransit" yaml:"inTransit"`
	KeyManagement KeyManagementRequirement `json:"keyManagement" yaml:"keyManagement"`
}

// EncryptionAtRest specifies encryption at rest requirements
type EncryptionAtRest struct {
	Required  bool     `json:"required" yaml:"required"`
	Algorithm string   `json:"algorithm" yaml:"algorithm"`
	KeySize   int      `json:"keySize" yaml:"keySize"`
	Providers []string `json:"providers" yaml:"providers"`
}

// EncryptionInTransit specifies encryption in transit requirements
type EncryptionInTransit struct {
	Required      bool     `json:"required" yaml:"required"`
	MinTLSVersion string   `json:"minTLSVersion" yaml:"minTLSVersion"`
	Protocols     []string `json:"protocols" yaml:"protocols"`
	CipherSuites  []string `json:"cipherSuites" yaml:"cipherSuites"`
}

// KeyManagementRequirement specifies key management requirements
type KeyManagementRequirement struct {
	Provider string        `json:"provider" yaml:"provider"`
	Rotation time.Duration `json:"rotation" yaml:"rotation"`
	Backup   bool          `json:"backup" yaml:"backup"`
	HSM      bool          `json:"hsm" yaml:"hsm"`
	Escrow   bool          `json:"escrow" yaml:"escrow"`
}

// AdmissionRequirement specifies admission controller requirements
type AdmissionRequirement struct {
	Controllers []AdmissionController `json:"controllers" yaml:"controllers"`
	Webhooks    []WebhookRequirement  `json:"webhooks" yaml:"webhooks"`
	Policies    []PolicyRequirement   `json:"policies" yaml:"policies"`
}

// AdmissionController specifies admission controller requirements
type AdmissionController struct {
	Name     string                 `json:"name" yaml:"name"`
	Required bool                   `json:"required" yaml:"required"`
	Version  string                 `json:"version" yaml:"version"`
	Config   map[string]interface{} `json:"config" yaml:"config"`
}

// WebhookRequirement specifies admission webhook requirements
type WebhookRequirement struct {
	Name          string   `json:"name" yaml:"name"`
	Type          string   `json:"type" yaml:"type"`
	Operations    []string `json:"operations" yaml:"operations"`
	Resources     []string `json:"resources" yaml:"resources"`
	FailurePolicy string   `json:"failurePolicy" yaml:"failurePolicy"`
	Required      bool     `json:"required" yaml:"required"`
}

// PolicyRequirement specifies policy requirements
type PolicyRequirement struct {
	Type     string                 `json:"type" yaml:"type"`
	Name     string                 `json:"name" yaml:"name"`
	Rules    []interface{}          `json:"rules" yaml:"rules"`
	Config   map[string]interface{} `json:"config" yaml:"config"`
	Required bool                   `json:"required" yaml:"required"`
}

// ComplianceRequirement specifies compliance requirements
type ComplianceRequirement struct {
	Standards   []string `json:"standards" yaml:"standards"`
	Frameworks  []string `json:"frameworks" yaml:"frameworks"`
	Benchmarks  []string `json:"benchmarks" yaml:"benchmarks"`
	Audit       bool     `json:"audit" yaml:"audit"`
	Reporting   bool     `json:"reporting" yaml:"reporting"`
	Remediation bool     `json:"remediation" yaml:"remediation"`
}

// CustomRequirement specifies custom requirements
type CustomRequirement struct {
	Name        string                 `json:"name" yaml:"name"`
	Type        string                 `json:"type" yaml:"type"`
	Description string                 `json:"description" yaml:"description"`
	Check       CustomCheck            `json:"check" yaml:"check"`
	Config      map[string]interface{} `json:"config" yaml:"config"`
	Priority    int                    `json:"priority" yaml:"priority"`
	Required    bool                   `json:"required" yaml:"required"`
}

// CustomCheck specifies custom check logic
type CustomCheck struct {
	Script   string            `json:"script" yaml:"script"`
	Command  []string          `json:"command" yaml:"command"`
	Expected interface{}       `json:"expected" yaml:"expected"`
	Operator string            `json:"operator" yaml:"operator"`
	Timeout  time.Duration     `json:"timeout" yaml:"timeout"`
	Retries  int               `json:"retries" yaml:"retries"`
	Env      map[string]string `json:"env" yaml:"env"`
}

// VendorRequirements specifies vendor-specific requirements
type VendorRequirements struct {
	AWS       AWSRequirements       `json:"aws" yaml:"aws"`
	Azure     AzureRequirements     `json:"azure" yaml:"azure"`
	GCP       GCPRequirements       `json:"gcp" yaml:"gcp"`
	VMware    VMwareRequirements    `json:"vmware" yaml:"vmware"`
	OpenShift OpenShiftRequirements `json:"openshift" yaml:"openshift"`
	Rancher   RancherRequirements   `json:"rancher" yaml:"rancher"`
}

// AWSRequirements specifies AWS-specific requirements
type AWSRequirements struct {
	EKS     EKSRequirement `json:"eks" yaml:"eks"`
	IAM     IAMRequirement `json:"iam" yaml:"iam"`
	VPC     VPCRequirement `json:"vpc" yaml:"vpc"`
	EBS     EBSRequirement `json:"ebs" yaml:"ebs"`
	ELB     ELBRequirement `json:"elb" yaml:"elb"`
	Regions []string       `json:"regions" yaml:"regions"`
	Zones   []string       `json:"zones" yaml:"zones"`
}

// EKSRequirement specifies EKS requirements
type EKSRequirement struct {
	Version      string   `json:"version" yaml:"version"`
	NodeGroups   int      `json:"nodeGroups" yaml:"nodeGroups"`
	ManagedNodes bool     `json:"managedNodes" yaml:"managedNodes"`
	Fargate      bool     `json:"fargate" yaml:"fargate"`
	Addons       []string `json:"addons" yaml:"addons"`
}

// IAMRequirement specifies IAM requirements
type IAMRequirement struct {
	Roles          []string `json:"roles" yaml:"roles"`
	Policies       []string `json:"policies" yaml:"policies"`
	ServiceAccount bool     `json:"serviceAccount" yaml:"serviceAccount"`
	OIDC           bool     `json:"oidc" yaml:"oidc"`
}

// VPCRequirement specifies VPC requirements
type VPCRequirement struct {
	Subnets         int      `json:"subnets" yaml:"subnets"`
	PrivateSubnets  int      `json:"privateSubnets" yaml:"privateSubnets"`
	PublicSubnets   int      `json:"publicSubnets" yaml:"publicSubnets"`
	NATGateway      bool     `json:"natGateway" yaml:"natGateway"`
	InternetGateway bool     `json:"internetGateway" yaml:"internetGateway"`
	CIDRBlocks      []string `json:"cidrBlocks" yaml:"cidrBlocks"`
}

// EBSRequirement specifies EBS requirements
type EBSRequirement struct {
	VolumeTypes []string `json:"volumeTypes" yaml:"volumeTypes"`
	Encryption  bool     `json:"encryption" yaml:"encryption"`
	Snapshots   bool     `json:"snapshots" yaml:"snapshots"`
	CSI         bool     `json:"csi" yaml:"csi"`
}

// ELBRequirement specifies ELB requirements
type ELBRequirement struct {
	Type        string `json:"type" yaml:"type"`
	Scheme      string `json:"scheme" yaml:"scheme"`
	Certificate bool   `json:"certificate" yaml:"certificate"`
	HealthCheck bool   `json:"healthCheck" yaml:"healthCheck"`
	CrossZone   bool   `json:"crossZone" yaml:"crossZone"`
}

// AzureRequirements specifies Azure-specific requirements
type AzureRequirements struct {
	AKS      AKSRequirement           `json:"aks" yaml:"aks"`
	Identity AzureIdentityRequirement `json:"identity" yaml:"identity"`
	Network  AzureNetworkRequirement  `json:"network" yaml:"network"`
	Storage  AzureStorageRequirement  `json:"storage" yaml:"storage"`
	Regions  []string                 `json:"regions" yaml:"regions"`
	Zones    []string                 `json:"zones" yaml:"zones"`
}

// AKSRequirement specifies AKS requirements
type AKSRequirement struct {
	Version       string   `json:"version" yaml:"version"`
	NodePools     int      `json:"nodePools" yaml:"nodePools"`
	AutoScaling   bool     `json:"autoScaling" yaml:"autoScaling"`
	RBAC          bool     `json:"rbac" yaml:"rbac"`
	NetworkPolicy bool     `json:"networkPolicy" yaml:"networkPolicy"`
	Addons        []string `json:"addons" yaml:"addons"`
}

// AzureIdentityRequirement specifies Azure identity requirements
type AzureIdentityRequirement struct {
	ManagedIdentity  bool     `json:"managedIdentity" yaml:"managedIdentity"`
	ServicePrincipal bool     `json:"servicePrincipal" yaml:"servicePrincipal"`
	KeyVault         bool     `json:"keyVault" yaml:"keyVault"`
	Roles            []string `json:"roles" yaml:"roles"`
}

// AzureNetworkRequirement specifies Azure network requirements
type AzureNetworkRequirement struct {
	VNet         bool   `json:"vnet" yaml:"vnet"`
	Subnets      int    `json:"subnets" yaml:"subnets"`
	NSG          bool   `json:"nsg" yaml:"nsg"`
	LoadBalancer string `json:"loadBalancer" yaml:"loadBalancer"`
	Gateway      bool   `json:"gateway" yaml:"gateway"`
}

// AzureStorageRequirement specifies Azure storage requirements
type AzureStorageRequirement struct {
	StorageAccount bool     `json:"storageAccount" yaml:"storageAccount"`
	ManagedDisks   bool     `json:"managedDisks" yaml:"managedDisks"`
	FileShare      bool     `json:"fileShare" yaml:"fileShare"`
	BlobStorage    bool     `json:"blobStorage" yaml:"blobStorage"`
	Encryption     bool     `json:"encryption" yaml:"encryption"`
	Tiers          []string `json:"tiers" yaml:"tiers"`
}

// GCPRequirements specifies GCP-specific requirements
type GCPRequirements struct {
	GKE     GKERequirement        `json:"gke" yaml:"gke"`
	IAM     GCPIAMRequirement     `json:"iam" yaml:"iam"`
	Network GCPNetworkRequirement `json:"network" yaml:"network"`
	Storage GCPStorageRequirement `json:"storage" yaml:"storage"`
	Regions []string              `json:"regions" yaml:"regions"`
	Zones   []string              `json:"zones" yaml:"zones"`
}

// GKERequirement specifies GKE requirements
type GKERequirement struct {
	Version       string   `json:"version" yaml:"version"`
	NodePools     int      `json:"nodePools" yaml:"nodePools"`
	AutoScaling   bool     `json:"autoScaling" yaml:"autoScaling"`
	Autopilot     bool     `json:"autopilot" yaml:"autopilot"`
	NetworkPolicy bool     `json:"networkPolicy" yaml:"networkPolicy"`
	Addons        []string `json:"addons" yaml:"addons"`
}

// GCPIAMRequirement specifies GCP IAM requirements
type GCPIAMRequirement struct {
	ServiceAccount   bool     `json:"serviceAccount" yaml:"serviceAccount"`
	WorkloadIdentity bool     `json:"workloadIdentity" yaml:"workloadIdentity"`
	Roles            []string `json:"roles" yaml:"roles"`
	Bindings         []string `json:"bindings" yaml:"bindings"`
}

// GCPNetworkRequirement specifies GCP network requirements
type GCPNetworkRequirement struct {
	VPC          bool   `json:"vpc" yaml:"vpc"`
	Subnets      int    `json:"subnets" yaml:"subnets"`
	Firewall     bool   `json:"firewall" yaml:"firewall"`
	LoadBalancer string `json:"loadBalancer" yaml:"loadBalancer"`
	CloudNAT     bool   `json:"cloudNAT" yaml:"cloudNAT"`
}

// GCPStorageRequirement specifies GCP storage requirements
type GCPStorageRequirement struct {
	PersistentDisk bool     `json:"persistentDisk" yaml:"persistentDisk"`
	CloudStorage   bool     `json:"cloudStorage" yaml:"cloudStorage"`
	Filestore      bool     `json:"filestore" yaml:"filestore"`
	Encryption     bool     `json:"encryption" yaml:"encryption"`
	Classes        []string `json:"classes" yaml:"classes"`
}

// VMwareRequirements specifies VMware-specific requirements
type VMwareRequirements struct {
	VSphere VSphereRequirement `json:"vsphere" yaml:"vsphere"`
	VSAN    VSANRequirement    `json:"vsan" yaml:"vsan"`
	NSX     NSXRequirement     `json:"nsx" yaml:"nsx"`
	Tanzu   TanzuRequirement   `json:"tanzu" yaml:"tanzu"`
}

// VSphereRequirement specifies vSphere requirements
type VSphereRequirement struct {
	Version    string   `json:"version" yaml:"version"`
	Datacenter string   `json:"datacenter" yaml:"datacenter"`
	Clusters   []string `json:"clusters" yaml:"clusters"`
	Hosts      int      `json:"hosts" yaml:"hosts"`
	DRS        bool     `json:"drs" yaml:"drs"`
	HA         bool     `json:"ha" yaml:"ha"`
	VMotion    bool     `json:"vmotion" yaml:"vmotion"`
}

// VSANRequirement specifies vSAN requirements
type VSANRequirement struct {
	Version     string `json:"version" yaml:"version"`
	Datastore   bool   `json:"datastore" yaml:"datastore"`
	Encryption  bool   `json:"encryption" yaml:"encryption"`
	Compression bool   `json:"compression" yaml:"compression"`
	Dedup       bool   `json:"dedup" yaml:"dedup"`
}

// NSXRequirement specifies NSX requirements
type NSXRequirement struct {
	Version       string `json:"version" yaml:"version"`
	LoadBalancer  bool   `json:"loadBalancer" yaml:"loadBalancer"`
	Firewall      bool   `json:"firewall" yaml:"firewall"`
	MicroSeg      bool   `json:"microsegmentation" yaml:"microsegmentation"`
	NetworkPolicy bool   `json:"networkPolicy" yaml:"networkPolicy"`
}

// TanzuRequirement specifies Tanzu requirements
type TanzuRequirement struct {
	Version            string   `json:"version" yaml:"version"`
	Grid               bool     `json:"grid" yaml:"grid"`
	Mission            bool     `json:"mission" yaml:"mission"`
	ApplicationCatalog bool     `json:"applicationCatalog" yaml:"applicationCatalog"`
	Addons             []string `json:"addons" yaml:"addons"`
}

// OpenShiftRequirements specifies OpenShift-specific requirements
type OpenShiftRequirements struct {
	Version      string                       `json:"version" yaml:"version"`
	Edition      string                       `json:"edition" yaml:"edition"`
	Installation OpenShiftInstallRequirement  `json:"installation" yaml:"installation"`
	Security     OpenShiftSecurityRequirement `json:"security" yaml:"security"`
	Networking   OpenShiftNetworkRequirement  `json:"networking" yaml:"networking"`
	Storage      OpenShiftStorageRequirement  `json:"storage" yaml:"storage"`
	Operators    []OperatorRequirement        `json:"operators" yaml:"operators"`
}

// OpenShiftInstallRequirement specifies OpenShift installation requirements
type OpenShiftInstallRequirement struct {
	Platform string `json:"platform" yaml:"platform"`
	Masters  int    `json:"masters" yaml:"masters"`
	Workers  int    `json:"workers" yaml:"workers"`
	Etcd     bool   `json:"etcd" yaml:"etcd"`
	Registry bool   `json:"registry" yaml:"registry"`
	Router   bool   `json:"router" yaml:"router"`
	Console  bool   `json:"console" yaml:"console"`
}

// OpenShiftSecurityRequirement specifies OpenShift security requirements
type OpenShiftSecurityRequirement struct {
	SCC          bool     `json:"scc" yaml:"scc"`
	OAuth        bool     `json:"oauth" yaml:"oauth"`
	Certificates bool     `json:"certificates" yaml:"certificates"`
	ServiceMesh  bool     `json:"serviceMesh" yaml:"serviceMesh"`
	Compliance   []string `json:"compliance" yaml:"compliance"`
}

// OpenShiftNetworkRequirement specifies OpenShift networking requirements
type OpenShiftNetworkRequirement struct {
	SDN         string `json:"sdn" yaml:"sdn"`
	CNI         string `json:"cni" yaml:"cni"`
	ServiceMesh bool   `json:"serviceMesh" yaml:"serviceMesh"`
	Ingress     string `json:"ingress" yaml:"ingress"`
	Routes      bool   `json:"routes" yaml:"routes"`
}

// OpenShiftStorageRequirement specifies OpenShift storage requirements
type OpenShiftStorageRequirement struct {
	OCS       bool     `json:"ocs" yaml:"ocs"`
	ODF       bool     `json:"odf" yaml:"odf"`
	Registry  string   `json:"registry" yaml:"registry"`
	Classes   []string `json:"classes" yaml:"classes"`
	Snapshots bool     `json:"snapshots" yaml:"snapshots"`
}

// OperatorRequirement specifies operator requirements
type OperatorRequirement struct {
	Name      string                 `json:"name" yaml:"name"`
	Namespace string                 `json:"namespace" yaml:"namespace"`
	Version   string                 `json:"version" yaml:"version"`
	Channel   string                 `json:"channel" yaml:"channel"`
	Source    string                 `json:"source" yaml:"source"`
	Config    map[string]interface{} `json:"config" yaml:"config"`
	Required  bool                   `json:"required" yaml:"required"`
}

// RancherRequirements specifies Rancher-specific requirements
type RancherRequirements struct {
	Version    string                       `json:"version" yaml:"version"`
	Edition    string                       `json:"edition" yaml:"edition"`
	Management RancherManagementRequirement `json:"management" yaml:"management"`
	Downstream RancherDownstreamRequirement `json:"downstream" yaml:"downstream"`
	Apps       RancherAppsRequirement       `json:"apps" yaml:"apps"`
	Security   RancherSecurityRequirement   `json:"security" yaml:"security"`
}

// RancherManagementRequirement specifies Rancher management requirements
type RancherManagementRequirement struct {
	HA         bool     `json:"ha" yaml:"ha"`
	Backup     bool     `json:"backup" yaml:"backup"`
	Monitoring bool     `json:"monitoring" yaml:"monitoring"`
	Logging    bool     `json:"logging" yaml:"logging"`
	Alerting   bool     `json:"alerting" yaml:"alerting"`
	Projects   []string `json:"projects" yaml:"projects"`
}

// RancherDownstreamRequirement specifies Rancher downstream requirements
type RancherDownstreamRequirement struct {
	Clusters    int      `json:"clusters" yaml:"clusters"`
	Imported    bool     `json:"imported" yaml:"imported"`
	Provisioned bool     `json:"provisioned" yaml:"provisioned"`
	Templates   []string `json:"templates" yaml:"templates"`
	Versions    []string `json:"versions" yaml:"versions"`
}

// RancherAppsRequirement specifies Rancher apps requirements
type RancherAppsRequirement struct {
	Catalog     []string `json:"catalog" yaml:"catalog"`
	Fleet       bool     `json:"fleet" yaml:"fleet"`
	Continuous  bool     `json:"continuous" yaml:"continuous"`
	Marketplace bool     `json:"marketplace" yaml:"marketplace"`
	Apps        []string `json:"apps" yaml:"apps"`
}

// RancherSecurityRequirement specifies Rancher security requirements
type RancherSecurityRequirement struct {
	CIS        bool     `json:"cis" yaml:"cis"`
	Scanning   bool     `json:"scanning" yaml:"scanning"`
	Policies   []string `json:"policies" yaml:"policies"`
	Benchmark  bool     `json:"benchmark" yaml:"benchmark"`
	Compliance []string `json:"compliance" yaml:"compliance"`
}

// ReplicatedRequirements specifies Replicated-specific requirements
type ReplicatedRequirements struct {
	KOTS          KOTSRequirement             `json:"kots" yaml:"kots"`
	Registry      RegistryRequirement         `json:"registry" yaml:"registry"`
	License       LicenseRequirement          `json:"license" yaml:"license"`
	Preflight     PreflightRequirement        `json:"preflight" yaml:"preflight"`
	SupportBundle SupportBundleRequirement    `json:"supportBundle" yaml:"supportBundle"`
	Config        ConfigRequirement           `json:"config" yaml:"config"`
	Backup        ReplicatedBackupRequirement `json:"backup" yaml:"backup"`
}

// KOTSRequirement specifies KOTS requirements
type KOTSRequirement struct {
	Version      string   `json:"version" yaml:"version"`
	MinVersion   string   `json:"minVersion" yaml:"minVersion"`
	AdminConsole bool     `json:"adminConsole" yaml:"adminConsole"`
	Airgap       bool     `json:"airgap" yaml:"airgap"`
	Snapshots    bool     `json:"snapshots" yaml:"snapshots"`
	Identity     bool     `json:"identity" yaml:"identity"`
	Applications []string `json:"applications" yaml:"applications"`
}

// RegistryRequirement specifies registry requirements
type RegistryRequirement struct {
	Type        string `json:"type" yaml:"type"`
	Endpoint    string `json:"endpoint" yaml:"endpoint"`
	Namespace   string `json:"namespace" yaml:"namespace"`
	Auth        bool   `json:"auth" yaml:"auth"`
	TLS         bool   `json:"tls" yaml:"tls"`
	Replication bool   `json:"replication" yaml:"replication"`
	Scanning    bool   `json:"scanning" yaml:"scanning"`
}

// LicenseRequirement specifies license requirements
type LicenseRequirement struct {
	Type         string                 `json:"type" yaml:"type"`
	Entitlements map[string]interface{} `json:"entitlements" yaml:"entitlements"`
	Expiration   bool                   `json:"expiration" yaml:"expiration"`
	Validation   bool                   `json:"validation" yaml:"validation"`
	Verification bool                   `json:"verification" yaml:"verification"`
}

// PreflightRequirement specifies preflight requirements
type PreflightRequirement struct {
	Required     bool     `json:"required" yaml:"required"`
	Analyzers    []string `json:"analyzers" yaml:"analyzers"`
	Collectors   []string `json:"collectors" yaml:"collectors"`
	HostChecks   bool     `json:"hostChecks" yaml:"hostChecks"`
	RemoteChecks bool     `json:"remoteChecks" yaml:"remoteChecks"`
	Strict       bool     `json:"strict" yaml:"strict"`
}

// SupportBundleRequirement specifies support bundle requirements
type SupportBundleRequirement struct {
	Required   bool          `json:"required" yaml:"required"`
	Collectors []string      `json:"collectors" yaml:"collectors"`
	Analyzers  []string      `json:"analyzers" yaml:"analyzers"`
	Redactors  []string      `json:"redactors" yaml:"redactors"`
	Size       int64         `json:"size" yaml:"size"`
	Retention  time.Duration `json:"retention" yaml:"retention"`
}

// ConfigRequirement specifies config requirements
type ConfigRequirement struct {
	Items       []ConfigItemRequirement  `json:"items" yaml:"items"`
	Groups      []ConfigGroupRequirement `json:"groups" yaml:"groups"`
	Validation  bool                     `json:"validation" yaml:"validation"`
	Templates   bool                     `json:"templates" yaml:"templates"`
	Conditional bool                     `json:"conditional" yaml:"conditional"`
}

// ConfigItemRequirement specifies config item requirements
type ConfigItemRequirement struct {
	Name       string                  `json:"name" yaml:"name"`
	Type       string                  `json:"type" yaml:"type"`
	Required   bool                    `json:"required" yaml:"required"`
	Default    interface{}             `json:"default" yaml:"default"`
	Validation ConfigValidation        `json:"validation" yaml:"validation"`
	Items      []ConfigItemRequirement `json:"items" yaml:"items"`
}

// ConfigGroupRequirement specifies config group requirements
type ConfigGroupRequirement struct {
	Name        string                  `json:"name" yaml:"name"`
	Title       string                  `json:"title" yaml:"title"`
	Description string                  `json:"description" yaml:"description"`
	Items       []ConfigItemRequirement `json:"items" yaml:"items"`
	When        string                  `json:"when" yaml:"when"`
}

// ConfigValidation specifies config validation requirements
type ConfigValidation struct {
	Regex    string      `json:"regex" yaml:"regex"`
	Message  string      `json:"message" yaml:"message"`
	Required bool        `json:"required" yaml:"required"`
	Min      interface{} `json:"min" yaml:"min"`
	Max      interface{} `json:"max" yaml:"max"`
	Options  []string    `json:"options" yaml:"options"`
}

// ReplicatedBackupRequirement specifies backup requirements for Replicated
type ReplicatedBackupRequirement struct {
	Velero       VeleroRequirement   `json:"velero" yaml:"velero"`
	Snapshots    bool                `json:"snapshots" yaml:"snapshots"`
	Schedule     string              `json:"schedule" yaml:"schedule"`
	Retention    time.Duration       `json:"retention" yaml:"retention"`
	Destinations []BackupDestination `json:"destinations" yaml:"destinations"`
	Validation   bool                `json:"validation" yaml:"validation"`
}

// VeleroRequirement specifies Velero requirements
type VeleroRequirement struct {
	Version    string   `json:"version" yaml:"version"`
	Plugins    []string `json:"plugins" yaml:"plugins"`
	CSI        bool     `json:"csi" yaml:"csi"`
	Restic     bool     `json:"restic" yaml:"restic"`
	Encryption bool     `json:"encryption" yaml:"encryption"`
}

// BackupDestination specifies backup destination requirements
type BackupDestination struct {
	Type        string            `json:"type" yaml:"type"`
	Endpoint    string            `json:"endpoint" yaml:"endpoint"`
	Bucket      string            `json:"bucket" yaml:"bucket"`
	Path        string            `json:"path" yaml:"path"`
	Credentials map[string]string `json:"credentials" yaml:"credentials"`
	Encryption  bool              `json:"encryption" yaml:"encryption"`
}

// RequirementCategory represents different categories of requirements
type RequirementCategory string

const (
	CategoryKubernetes RequirementCategory = "kubernetes"
	CategoryResources  RequirementCategory = "resources"
	CategoryStorage    RequirementCategory = "storage"
	CategoryNetwork    RequirementCategory = "network"
	CategorySecurity   RequirementCategory = "security"
	CategoryCustom     RequirementCategory = "custom"
	CategoryVendor     RequirementCategory = "vendor"
	CategoryReplicated RequirementCategory = "replicated"
)

// RequirementPriority represents the priority level of requirements
type RequirementPriority string

const (
	PriorityRequired    RequirementPriority = "required"
	PriorityRecommended RequirementPriority = "recommended"
	PriorityOptional    RequirementPriority = "optional"
	PriorityDeprecated  RequirementPriority = "deprecated"
)

// RequirementConflict represents a conflict between requirements
type RequirementConflict struct {
	Type         string   `json:"type"`
	Description  string   `json:"description"`
	Requirements []string `json:"requirements"`
	Resolution   string   `json:"resolution"`
	Severity     string   `json:"severity"`
}

// RequirementValidation represents validation rules for requirements
type RequirementValidation struct {
	Rules    []ValidationRule `json:"rules"`
	Errors   []string         `json:"errors"`
	Warnings []string         `json:"warnings"`
	Valid    bool             `json:"valid"`
}

// GetCategory returns the category for a given requirement field path
func GetCategory(fieldPath string) RequirementCategory {
	parts := strings.Split(fieldPath, ".")
	if len(parts) == 0 {
		return CategoryCustom
	}

	switch parts[0] {
	case "kubernetes":
		return CategoryKubernetes
	case "resources":
		return CategoryResources
	case "storage":
		return CategoryStorage
	case "network":
		return CategoryNetwork
	case "security":
		return CategorySecurity
	case "vendor":
		return CategoryVendor
	case "replicated":
		return CategoryReplicated
	default:
		return CategoryCustom
	}
}

// ValidateRequirementName validates requirement names
func ValidateRequirementName(name string) error {
	if name == "" {
		return fmt.Errorf("requirement name cannot be empty")
	}

	// Check for valid characters (alphanumeric, dash, underscore)
	nameRegex := regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("requirement name '%s' contains invalid characters (only alphanumeric, dash, underscore allowed)", name)
	}

	// Check length
	if len(name) > 255 {
		return fmt.Errorf("requirement name '%s' is too long (max 255 characters)", name)
	}

	return nil
}

// ValidateVersion validates version strings
func ValidateVersion(version string) error {
	if version == "" {
		return nil // Version is optional
	}

	// Basic semantic version validation
	versionRegex := regexp.MustCompile(`^v?[0-9]+\.[0-9]+(\.[0-9]+)?(-[a-zA-Z0-9.-]+)?(\+[a-zA-Z0-9.-]+)?$`)
	if !versionRegex.MatchString(version) {
		return fmt.Errorf("version '%s' is not a valid semantic version", version)
	}

	return nil
}

// ValidateAPIVersion validates API version strings
func ValidateAPIVersion(apiVersion string) error {
	if apiVersion == "" {
		return fmt.Errorf("API version cannot be empty")
	}

	// Check for valid API version format (group/version or just version)
	apiVersionRegex := regexp.MustCompile(`^[a-zA-Z0-9.-]+(/v[0-9]+(alpha|beta)?[0-9]*)?$`)
	if !apiVersionRegex.MatchString(apiVersion) {
		return fmt.Errorf("API version '%s' is not valid", apiVersion)
	}

	return nil
}
