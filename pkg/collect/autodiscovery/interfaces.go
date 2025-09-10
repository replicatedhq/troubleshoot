package autodiscovery

import (
	"context"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

// AutoCollector defines the interface for automatic collector discovery
type AutoCollector interface {
	// DiscoverFoundational discovers foundational collectors based on cluster state (Path 1)
	DiscoverFoundational(ctx context.Context, opts DiscoveryOptions) ([]CollectorSpec, error)
	// AugmentWithFoundational augments existing YAML collectors with foundational collectors (Path 2)
	AugmentWithFoundational(ctx context.Context, yamlCollectors []CollectorSpec, opts DiscoveryOptions) ([]CollectorSpec, error)
	// ValidatePermissions validates RBAC permissions for discovered resources
	ValidatePermissions(ctx context.Context, resources []Resource) ([]Resource, error)
}

// DiscoveryOptions configures the autodiscovery behavior
type DiscoveryOptions struct {
	// Target namespaces for discovery (empty = all accessible namespaces)
	Namespaces []string
	// Include container image metadata collection
	IncludeImages bool
	// Perform RBAC permission checking
	RBACCheck bool
	// Maximum discovery depth for resource relationships
	MaxDepth int
	// Path 1: Only collect foundational data
	FoundationalOnly bool
	// Path 2: Add foundational to existing YAML specs
	AugmentMode bool
	// Timeout for discovery operations
	Timeout time.Duration
}

// CollectorSpec represents a collector specification that can be converted to troubleshootv1beta2.Collect
type CollectorSpec struct {
	// Type of collector (logs, clusterResources, secret, etc.)
	Type CollectorType
	// Name of the collector for identification
	Name string
	// Namespace for namespaced resources
	Namespace string
	// Spec contains the actual collector configuration
	Spec interface{}
	// Priority for deduplication (higher wins)
	Priority int
	// Source indicates where this collector came from (foundational, yaml, etc.)
	Source CollectorSource
}

// CollectorType represents the type of data being collected
type CollectorType string

const (
	CollectorTypePods            CollectorType = "pods"
	CollectorTypeDeployments     CollectorType = "deployments"
	CollectorTypeServices        CollectorType = "services"
	CollectorTypeConfigMaps      CollectorType = "configmaps"
	CollectorTypeSecrets         CollectorType = "secrets"
	CollectorTypeEvents          CollectorType = "events"
	CollectorTypeLogs            CollectorType = "logs"
	CollectorTypeClusterInfo     CollectorType = "clusterInfo"
	CollectorTypeClusterResources CollectorType = "clusterResources"
	CollectorTypeImageFacts      CollectorType = "imageFacts"
)

// CollectorSource indicates the origin of a collector
type CollectorSource string

const (
	SourceFoundational CollectorSource = "foundational"
	SourceYAML         CollectorSource = "yaml"
	SourceAugmented    CollectorSource = "augmented"
)

// Resource represents a Kubernetes resource for RBAC checking
type Resource struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

// FoundationalCollectors represents the set of collectors that are always included
type FoundationalCollectors struct {
	// Core Kubernetes resources always collected
	Pods           []CollectorSpec
	Deployments    []CollectorSpec
	Services       []CollectorSpec
	ConfigMaps     []CollectorSpec
	Secrets        []CollectorSpec
	Events         []CollectorSpec
	Logs           []CollectorSpec
	ClusterInfo    []CollectorSpec
	ClusterResources []CollectorSpec
	// Container image metadata
	ImageFacts     []CollectorSpec
}

// ToTroubleshootCollect converts a CollectorSpec to a troubleshootv1beta2.Collect
func (c CollectorSpec) ToTroubleshootCollect() (*troubleshootv1beta2.Collect, error) {
	collect := &troubleshootv1beta2.Collect{}
	
	switch c.Type {
	case CollectorTypeLogs:
		if logs, ok := c.Spec.(*troubleshootv1beta2.Logs); ok {
			collect.Logs = logs
		}
	case CollectorTypeClusterInfo:
		if clusterInfo, ok := c.Spec.(*troubleshootv1beta2.ClusterInfo); ok {
			collect.ClusterInfo = clusterInfo
		}
	case CollectorTypeClusterResources:
		if clusterResources, ok := c.Spec.(*troubleshootv1beta2.ClusterResources); ok {
			collect.ClusterResources = clusterResources
		}
	case CollectorTypeSecrets:
		if secret, ok := c.Spec.(*troubleshootv1beta2.Secret); ok {
			collect.Secret = secret
		}
	case CollectorTypeConfigMaps:
		if configMap, ok := c.Spec.(*troubleshootv1beta2.ConfigMap); ok {
			collect.ConfigMap = configMap
		}
	case CollectorTypeImageFacts:
		if data, ok := c.Spec.(*troubleshootv1beta2.Data); ok {
			collect.Data = data
		}
	// Add more cases as needed for other collector types
	}
	
	return collect, nil
}

// GetUniqueKey returns a unique identifier for deduplication
func (c CollectorSpec) GetUniqueKey() string {
	if c.Namespace != "" {
		return string(c.Type) + "/" + c.Namespace + "/" + c.Name
	}
	return string(c.Type) + "/" + c.Name
}
