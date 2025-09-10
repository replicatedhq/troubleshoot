package autodiscovery

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// Discoverer implements the AutoCollector interface
type Discoverer struct {
	clientConfig *rest.Config
	client       kubernetes.Interface
	rbacChecker  *RBACChecker
	expander     *ResourceExpander
}

// NewDiscoverer creates a new autodiscovery discoverer
func NewDiscoverer(clientConfig *rest.Config, client kubernetes.Interface) (*Discoverer, error) {
	if clientConfig == nil {
		return nil, errors.New("client config is required")
	}
	if client == nil {
		return nil, errors.New("kubernetes client is required")
	}

	rbacChecker, err := NewRBACChecker(client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create RBAC checker")
	}

	expander := NewResourceExpander()

	return &Discoverer{
		clientConfig: clientConfig,
		client:       client,
		rbacChecker:  rbacChecker,
		expander:     expander,
	}, nil
}

// DiscoverFoundational discovers foundational collectors based on cluster state (Path 1)
func (d *Discoverer) DiscoverFoundational(ctx context.Context, opts DiscoveryOptions) ([]CollectorSpec, error) {
	klog.V(2).Infof("Starting foundational discovery for namespaces: %v", opts.Namespaces)

	// Set default timeout if not provided
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	// Create context with timeout
	discoveryCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	// Get target namespaces
	namespaces, err := d.getTargetNamespaces(discoveryCtx, opts.Namespaces)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get target namespaces")
	}

	// Generate foundational collectors
	foundationalCollectors := d.generateFoundationalCollectors(namespaces, opts)

	// Apply RBAC filtering if enabled
	if opts.RBACCheck {
		filteredCollectors, err := d.applyRBACFiltering(discoveryCtx, foundationalCollectors)
		if err != nil {
			klog.Warningf("RBAC filtering failed, proceeding without: %v", err)
		} else {
			foundationalCollectors = filteredCollectors
		}
	}

	klog.V(2).Infof("Discovered %d foundational collectors", len(foundationalCollectors))
	return foundationalCollectors, nil
}

// AugmentWithFoundational augments existing YAML collectors with foundational collectors (Path 2)
func (d *Discoverer) AugmentWithFoundational(ctx context.Context, yamlCollectors []CollectorSpec, opts DiscoveryOptions) ([]CollectorSpec, error) {
	klog.V(2).Infof("Augmenting %d YAML collectors with foundational collectors", len(yamlCollectors))

	// First, get foundational collectors
	foundationalCollectors, err := d.DiscoverFoundational(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to discover foundational collectors")
	}

	// Convert YAML collectors to CollectorSpec format if needed
	yamlSpecs := make([]CollectorSpec, len(yamlCollectors))
	for i, collector := range yamlCollectors {
		collector.Source = SourceYAML
		yamlSpecs[i] = collector
	}

	// Merge and deduplicate collectors
	mergedCollectors := d.mergeAndDeduplicateCollectors(yamlSpecs, foundationalCollectors)

	klog.V(2).Infof("Merged result: %d total collectors (%d YAML + %d foundational, deduplicated)", 
		len(mergedCollectors), len(yamlSpecs), len(foundationalCollectors))

	return mergedCollectors, nil
}

// ValidatePermissions validates RBAC permissions for discovered resources
func (d *Discoverer) ValidatePermissions(ctx context.Context, resources []Resource) ([]Resource, error) {
	if d.rbacChecker == nil {
		return resources, nil
	}

	return d.rbacChecker.FilterByPermissions(ctx, resources)
}

// getTargetNamespaces returns the list of namespaces to target for discovery
func (d *Discoverer) getTargetNamespaces(ctx context.Context, requestedNamespaces []string) ([]string, error) {
	if len(requestedNamespaces) > 0 {
		return requestedNamespaces, nil
	}

	// If no namespaces specified, get all accessible namespaces
	namespaceList, err := d.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		// Fall back to default namespace if we can't list namespaces
		klog.Warningf("Could not list namespaces, falling back to 'default': %v", err)
		return []string{"default"}, nil
	}

	var namespaces []string
	for _, ns := range namespaceList.Items {
		namespaces = append(namespaces, ns.Name)
	}

	return namespaces, nil
}

// generateFoundationalCollectors creates the standard set of foundational collectors
func (d *Discoverer) generateFoundationalCollectors(namespaces []string, opts DiscoveryOptions) []CollectorSpec {
	var collectors []CollectorSpec

	// Always include cluster-level info
	collectors = append(collectors, d.generateClusterInfoCollectors()...)

	// Add namespace-scoped collectors for each target namespace
	for _, namespace := range namespaces {
		collectors = append(collectors, d.generateNamespacedCollectors(namespace, opts)...)
	}

	return collectors
}

// generateClusterInfoCollectors creates cluster-level collectors
func (d *Discoverer) generateClusterInfoCollectors() []CollectorSpec {
	return []CollectorSpec{
		{
			Type:     CollectorTypeClusterInfo,
			Name:     "cluster-info",
			Spec:     &troubleshootv1beta2.ClusterInfo{},
			Priority: 100,
			Source:   SourceFoundational,
		},
		{
			Type: CollectorTypeClusterResources,
			Name: "cluster-resources",
			Spec: &troubleshootv1beta2.ClusterResources{},
			Priority: 100,
			Source:   SourceFoundational,
		},
	}
}

// generateNamespacedCollectors creates namespace-specific collectors
func (d *Discoverer) generateNamespacedCollectors(namespace string, opts DiscoveryOptions) []CollectorSpec {
	var collectors []CollectorSpec

	// Pod logs collector
	collectors = append(collectors, CollectorSpec{
		Type:      CollectorTypeLogs,
		Name:      fmt.Sprintf("logs-%s", namespace),
		Namespace: namespace,
		Spec: &troubleshootv1beta2.Logs{
			CollectorMeta: troubleshootv1beta2.CollectorMeta{
				CollectorName: fmt.Sprintf("logs/%s", namespace),
			},
			Namespace: namespace,
			Selector:  []string{}, // Empty selector to collect all pods
		},
		Priority: 90,
		Source:   SourceFoundational,
	})

	// ConfigMaps collector
	collectors = append(collectors, CollectorSpec{
		Type:      CollectorTypeConfigMaps,
		Name:      fmt.Sprintf("configmaps-%s", namespace),
		Namespace: namespace,
		Spec: &troubleshootv1beta2.ConfigMap{
			CollectorMeta: troubleshootv1beta2.CollectorMeta{
				CollectorName: fmt.Sprintf("configmaps/%s", namespace),
			},
			Namespace:      namespace,
			Selector:       []string{"*"}, // Select all configmaps in namespace
			IncludeAllData: true,
		},
		Priority: 80,
		Source:   SourceFoundational,
	})

	// Secrets collector (metadata only by default)
	collectors = append(collectors, CollectorSpec{
		Type:      CollectorTypeSecrets,
		Name:      fmt.Sprintf("secrets-%s", namespace),
		Namespace: namespace,
			Spec: &troubleshootv1beta2.Secret{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: fmt.Sprintf("secrets/%s", namespace),
				},
				Namespace:      namespace,
				Selector:       []string{"*"}, // Select all secrets in namespace
				IncludeValue:   false,      // Don't include secret values by default
				IncludeAllData: false,      // Don't include secret data by default for security
			},
		Priority: 70,
		Source:   SourceFoundational,
	})

		// Add image facts collector if requested
		if opts.IncludeImages {
			collectors = append(collectors, CollectorSpec{
				Type:      CollectorTypeImageFacts,
				Name:      fmt.Sprintf("image-facts-%s", namespace),
				Namespace: namespace,
				Spec: &troubleshootv1beta2.Data{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: fmt.Sprintf("image-facts/%s", namespace),
					},
					Name: fmt.Sprintf("image-facts-%s", namespace),
					Data: fmt.Sprintf("Image facts collection for namespace %s", namespace),
				},
				Priority: 60,
				Source:   SourceFoundational,
			})
		}

	return collectors
}

// applyRBACFiltering filters collectors based on RBAC permissions
func (d *Discoverer) applyRBACFiltering(ctx context.Context, collectors []CollectorSpec) ([]CollectorSpec, error) {
	// Convert collectors to resources for RBAC checking
	var resources []Resource
	for _, collector := range collectors {
		resource := d.collectorToResource(collector)
		if resource != nil {
			resources = append(resources, *resource)
		}
	}

	// Filter by permissions
	allowedResources, err := d.rbacChecker.FilterByPermissions(ctx, resources)
	if err != nil {
		return nil, errors.Wrap(err, "failed to filter by permissions")
	}

	// Create map of allowed resource keys
	allowedKeys := make(map[string]bool)
	for _, resource := range allowedResources {
		key := fmt.Sprintf("%s/%s/%s/%s", resource.APIVersion, resource.Kind, resource.Namespace, resource.Name)
		allowedKeys[key] = true
	}

	// Filter collectors based on allowed resources
	var filteredCollectors []CollectorSpec
	for _, collector := range collectors {
		resource := d.collectorToResource(collector)
		if resource == nil {
			// If we can't convert to resource, include it (might be cluster-level)
			filteredCollectors = append(filteredCollectors, collector)
			continue
		}

		key := fmt.Sprintf("%s/%s/%s/%s", resource.APIVersion, resource.Kind, resource.Namespace, resource.Name)
		if allowedKeys[key] {
			filteredCollectors = append(filteredCollectors, collector)
		} else {
			klog.V(3).Infof("Filtered out collector %s due to RBAC permissions", collector.Name)
		}
	}

	return filteredCollectors, nil
}

// collectorToResource converts a CollectorSpec to a Resource for RBAC checking
func (d *Discoverer) collectorToResource(collector CollectorSpec) *Resource {
	switch collector.Type {
	case CollectorTypeLogs:
		return &Resource{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  collector.Namespace,
			Name:       "*",
		}
	case CollectorTypeConfigMaps:
		return &Resource{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Namespace:  collector.Namespace,
			Name:       "*",
		}
	case CollectorTypeSecrets:
		return &Resource{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  collector.Namespace,
			Name:       "*",
		}
	case CollectorTypeClusterInfo, CollectorTypeClusterResources:
		return &Resource{
			APIVersion: "v1",
			Kind:       "Node",
			Namespace:  "",
			Name:       "*",
		}
	default:
		return nil
	}
}

// mergeAndDeduplicateCollectors merges YAML and foundational collectors, removing duplicates
func (d *Discoverer) mergeAndDeduplicateCollectors(yamlCollectors, foundationalCollectors []CollectorSpec) []CollectorSpec {
	collectorMap := make(map[string]CollectorSpec)

	// Add foundational collectors first (lower priority)
	for _, collector := range foundationalCollectors {
		key := collector.GetUniqueKey()
		collectorMap[key] = collector
	}

	// Add YAML collectors (higher priority, will override foundational)
	for _, collector := range yamlCollectors {
		key := collector.GetUniqueKey()
		if existing, exists := collectorMap[key]; exists {
			klog.V(3).Infof("YAML collector %s overriding foundational collector %s", collector.Name, existing.Name)
		}
		collectorMap[key] = collector
	}

	// Convert map back to slice
	var result []CollectorSpec
	for _, collector := range collectorMap {
		result = append(result, collector)
	}

	return result
}

