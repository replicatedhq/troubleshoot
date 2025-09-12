package autodiscovery

import (
	"context"
	"fmt"
	"sort"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

// ResourceExpander handles converting discovered resources to collector specifications
type ResourceExpander struct {
	expansionRules map[CollectorType]ExpansionRule
}

// ExpansionRule defines how a resource type should be expanded into collectors
type ExpansionRule struct {
	// CollectorType is the type of collector this rule creates
	CollectorType CollectorType
	// Priority determines the order of collectors (higher = more important)
	Priority int
	// RequiredPermissions lists the RBAC permissions needed
	RequiredPermissions []ResourcePermission
	// ExpansionFunc creates the actual collector spec
	ExpansionFunc func(context.Context, ExpansionContext) ([]CollectorSpec, error)
	// Dependencies lists other collector types this depends on
	Dependencies []CollectorType
}

// ResourcePermission represents a required RBAC permission
type ResourcePermission struct {
	APIVersion string
	Kind       string
	Verbs      []string // get, list, watch, etc.
}

// ExpansionContext provides context for resource expansion
type ExpansionContext struct {
	Namespace string
	Options   DiscoveryOptions
	Resources []Resource
	Metadata  map[string]interface{}
}

// NewResourceExpander creates a new resource expander with default rules
func NewResourceExpander() *ResourceExpander {
	expander := &ResourceExpander{
		expansionRules: make(map[CollectorType]ExpansionRule),
	}

	// Register default expansion rules
	expander.registerDefaultRules()

	return expander
}

// registerDefaultRules registers the standard set of expansion rules
func (re *ResourceExpander) registerDefaultRules() {
	// Cluster info collector rule
	re.RegisterRule(CollectorTypeClusterInfo, ExpansionRule{
		CollectorType: CollectorTypeClusterInfo,
		Priority:      100,
		RequiredPermissions: []ResourcePermission{
			{APIVersion: "v1", Kind: "Node", Verbs: []string{"list"}},
		},
		ExpansionFunc: re.expandClusterInfo,
	})

	// Cluster resources collector rule
	re.RegisterRule(CollectorTypeClusterResources, ExpansionRule{
		CollectorType: CollectorTypeClusterResources,
		Priority:      95,
		RequiredPermissions: []ResourcePermission{
			{APIVersion: "v1", Kind: "Node", Verbs: []string{"list"}},
			{APIVersion: "v1", Kind: "Namespace", Verbs: []string{"list"}},
		},
		ExpansionFunc: re.expandClusterResources,
	})

	// Pod logs collector rule
	re.RegisterRule(CollectorTypeLogs, ExpansionRule{
		CollectorType: CollectorTypeLogs,
		Priority:      90,
		RequiredPermissions: []ResourcePermission{
			{APIVersion: "v1", Kind: "Pod", Verbs: []string{"list", "get"}},
		},
		ExpansionFunc: re.expandPodLogs,
	})

	// ConfigMaps collector rule
	re.RegisterRule(CollectorTypeConfigMaps, ExpansionRule{
		CollectorType: CollectorTypeConfigMaps,
		Priority:      80,
		RequiredPermissions: []ResourcePermission{
			{APIVersion: "v1", Kind: "ConfigMap", Verbs: []string{"list", "get"}},
		},
		ExpansionFunc: re.expandConfigMaps,
	})

	// Secrets collector rule
	re.RegisterRule(CollectorTypeSecrets, ExpansionRule{
		CollectorType: CollectorTypeSecrets,
		Priority:      75,
		RequiredPermissions: []ResourcePermission{
			{APIVersion: "v1", Kind: "Secret", Verbs: []string{"list", "get"}},
		},
		ExpansionFunc: re.expandSecrets,
	})

	// Events collector rule
	re.RegisterRule(CollectorTypeEvents, ExpansionRule{
		CollectorType: CollectorTypeEvents,
		Priority:      70,
		RequiredPermissions: []ResourcePermission{
			{APIVersion: "v1", Kind: "Event", Verbs: []string{"list"}},
		},
		ExpansionFunc: re.expandEvents,
	})

	// Image facts collector rule
	re.RegisterRule(CollectorTypeImageFacts, ExpansionRule{
		CollectorType: CollectorTypeImageFacts,
		Priority:      60,
		RequiredPermissions: []ResourcePermission{
			{APIVersion: "v1", Kind: "Pod", Verbs: []string{"list", "get"}},
		},
		ExpansionFunc: re.expandImageFacts,
	})
}

// RegisterRule registers a new expansion rule
func (re *ResourceExpander) RegisterRule(collectorType CollectorType, rule ExpansionRule) {
	re.expansionRules[collectorType] = rule
	klog.V(4).Infof("Registered expansion rule for collector type: %s", collectorType)
}

// ExpandToCollectors converts discovered resources to collector specifications
func (re *ResourceExpander) ExpandToCollectors(ctx context.Context, namespaces []string, opts DiscoveryOptions) ([]CollectorSpec, error) {
	klog.V(3).Infof("Expanding resources to collectors for %d namespaces", len(namespaces))

	var allCollectors []CollectorSpec

	// Generate cluster-level collectors first
	clusterCollectors, err := re.generateClusterLevelCollectors(ctx, opts)
	if err != nil {
		klog.Warningf("Failed to generate cluster-level collectors: %v", err)
	} else {
		allCollectors = append(allCollectors, clusterCollectors...)
	}

	// Generate namespace-scoped collectors
	for _, namespace := range namespaces {
		namespaceCollectors, err := re.generateNamespaceCollectors(ctx, namespace, opts)
		if err != nil {
			klog.Warningf("Failed to generate collectors for namespace %s: %v", namespace, err)
			continue
		}
		allCollectors = append(allCollectors, namespaceCollectors...)
	}

	// Sort collectors by priority (higher first)
	sort.Slice(allCollectors, func(i, j int) bool {
		return allCollectors[i].Priority > allCollectors[j].Priority
	})

	klog.V(3).Infof("Resource expansion complete: generated %d collectors", len(allCollectors))
	return allCollectors, nil
}

// generateClusterLevelCollectors creates cluster-scoped collectors
func (re *ResourceExpander) generateClusterLevelCollectors(ctx context.Context, opts DiscoveryOptions) ([]CollectorSpec, error) {
	var collectors []CollectorSpec

	context := ExpansionContext{
		Namespace: "",
		Options:   opts,
		Metadata:  make(map[string]interface{}),
	}

	// Generate cluster info collector
	if rule, exists := re.expansionRules[CollectorTypeClusterInfo]; exists {
		clusterCollectors, err := rule.ExpansionFunc(ctx, context)
		if err != nil {
			klog.Warningf("Failed to expand cluster info collectors: %v", err)
		} else {
			collectors = append(collectors, clusterCollectors...)
		}
	}

	// Generate cluster resources collector
	if rule, exists := re.expansionRules[CollectorTypeClusterResources]; exists {
		resourceCollectors, err := rule.ExpansionFunc(ctx, context)
		if err != nil {
			klog.Warningf("Failed to expand cluster resources collectors: %v", err)
		} else {
			collectors = append(collectors, resourceCollectors...)
		}
	}

	return collectors, nil
}

// generateNamespaceCollectors creates namespace-scoped collectors
func (re *ResourceExpander) generateNamespaceCollectors(ctx context.Context, namespace string, opts DiscoveryOptions) ([]CollectorSpec, error) {
	var collectors []CollectorSpec

	context := ExpansionContext{
		Namespace: namespace,
		Options:   opts,
		Metadata:  make(map[string]interface{}),
	}

	// Generate collectors for each type
	collectorTypes := []CollectorType{
		CollectorTypeLogs,
		CollectorTypeConfigMaps,
		CollectorTypeSecrets,
		CollectorTypeEvents,
	}

	// Add image facts if requested
	if opts.IncludeImages {
		collectorTypes = append(collectorTypes, CollectorTypeImageFacts)
	}

	for _, collectorType := range collectorTypes {
		if rule, exists := re.expansionRules[collectorType]; exists {
			typeCollectors, err := rule.ExpansionFunc(ctx, context)
			if err != nil {
				klog.Warningf("Failed to expand %s collectors for namespace %s: %v", collectorType, namespace, err)
				continue
			}
			collectors = append(collectors, typeCollectors...)
		}
	}

	return collectors, nil
}

// Expansion functions for different collector types

func (re *ResourceExpander) expandClusterInfo(ctx context.Context, context ExpansionContext) ([]CollectorSpec, error) {
	return []CollectorSpec{
		{
			Type:     CollectorTypeClusterInfo,
			Name:     "cluster-info",
			Spec:     &troubleshootv1beta2.ClusterInfo{},
			Priority: 100,
			Source:   SourceFoundational,
		},
	}, nil
}

func (re *ResourceExpander) expandClusterResources(ctx context.Context, context ExpansionContext) ([]CollectorSpec, error) {
	return []CollectorSpec{
		{
			Type:     CollectorTypeClusterResources,
			Name:     "cluster-resources",
			Spec:     &troubleshootv1beta2.ClusterResources{},
			Priority: 95,
			Source:   SourceFoundational,
		},
	}, nil
}

func (re *ResourceExpander) expandPodLogs(ctx context.Context, context ExpansionContext) ([]CollectorSpec, error) {
	if context.Namespace == "" {
		return nil, fmt.Errorf("namespace required for pod logs collector")
	}

	return []CollectorSpec{
		{
			Type:      CollectorTypeLogs,
			Name:      fmt.Sprintf("logs-%s", context.Namespace),
			Namespace: context.Namespace,
			Spec: &troubleshootv1beta2.Logs{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: fmt.Sprintf("logs/%s", context.Namespace),
				},
				Namespace: context.Namespace,
				Selector:  []string{}, // Empty selector to collect all pods
				Limits: &troubleshootv1beta2.LogLimits{
					MaxLines: 10000, // Limit to prevent excessive log collection
				},
			},
			Priority: 90,
			Source:   SourceFoundational,
		},
	}, nil
}

func (re *ResourceExpander) expandConfigMaps(ctx context.Context, context ExpansionContext) ([]CollectorSpec, error) {
	if context.Namespace == "" {
		return nil, fmt.Errorf("namespace required for configmaps collector")
	}

	return []CollectorSpec{
		{
			Type:      CollectorTypeConfigMaps,
			Name:      fmt.Sprintf("configmaps-%s", context.Namespace),
			Namespace: context.Namespace,
			Spec: &troubleshootv1beta2.ConfigMap{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: fmt.Sprintf("configmaps/%s", context.Namespace),
				},
				Namespace:      context.Namespace,
				Selector:       []string{"*"}, // Select all configmaps in namespace
				IncludeAllData: true,
			},
			Priority: 80,
			Source:   SourceFoundational,
		},
	}, nil
}

func (re *ResourceExpander) expandSecrets(ctx context.Context, context ExpansionContext) ([]CollectorSpec, error) {
	if context.Namespace == "" {
		return nil, fmt.Errorf("namespace required for secrets collector")
	}

	return []CollectorSpec{
		{
			Type:      CollectorTypeSecrets,
			Name:      fmt.Sprintf("secrets-%s", context.Namespace),
			Namespace: context.Namespace,
			Spec: &troubleshootv1beta2.Secret{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: fmt.Sprintf("secrets/%s", context.Namespace),
				},
				Namespace:      context.Namespace,
				Selector:       []string{"*"}, // Select all secrets in namespace
				IncludeValue:   false,         // Don't include secret values by default for security
				IncludeAllData: false,         // Don't include secret data by default for security
			},
			Priority: 75,
			Source:   SourceFoundational,
		},
	}, nil
}

func (re *ResourceExpander) expandEvents(ctx context.Context, context ExpansionContext) ([]CollectorSpec, error) {
	if context.Namespace == "" {
		return nil, fmt.Errorf("namespace required for events collector")
	}

	// Create a custom events collector using the Data collector type
	// since there's no specific Events collector in troubleshoot
	eventsCollectorName := fmt.Sprintf("events-%s", context.Namespace)

	return []CollectorSpec{
		{
			Type:      CollectorTypeEvents,
			Name:      eventsCollectorName,
			Namespace: context.Namespace,
			Spec: &troubleshootv1beta2.Data{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: fmt.Sprintf("events/%s", context.Namespace),
				},
				Name: eventsCollectorName,
				Data: fmt.Sprintf("Events in namespace %s", context.Namespace),
			},
			Priority: 70,
			Source:   SourceFoundational,
		},
	}, nil
}

func (re *ResourceExpander) expandImageFacts(ctx context.Context, context ExpansionContext) ([]CollectorSpec, error) {
	if context.Namespace == "" {
		return nil, fmt.Errorf("namespace required for image facts collector")
	}

	// Create placeholder data that indicates this will contain image facts JSON
	placeholderData := fmt.Sprintf(`{"namespace": "%s", "description": "Container image facts and metadata", "type": "image-facts"}`, context.Namespace)

	return []CollectorSpec{
		{
			Type:      CollectorTypeImageFacts,
			Name:      fmt.Sprintf("image-facts-%s", context.Namespace),
			Namespace: context.Namespace,
			Spec: &troubleshootv1beta2.Data{
				CollectorMeta: troubleshootv1beta2.CollectorMeta{
					CollectorName: fmt.Sprintf("image-facts/%s", context.Namespace),
				},
				Name: fmt.Sprintf("image-facts-%s", context.Namespace),
				Data: placeholderData,
			},
			Priority: 60,
			Source:   SourceFoundational,
		},
	}, nil
}

// GetRequiredPermissions returns the RBAC permissions required for a collector type
func (re *ResourceExpander) GetRequiredPermissions(collectorType CollectorType) []ResourcePermission {
	if rule, exists := re.expansionRules[collectorType]; exists {
		return rule.RequiredPermissions
	}
	return nil
}

// ValidateCollectorDependencies ensures all collector dependencies are satisfied
func (re *ResourceExpander) ValidateCollectorDependencies(collectors []CollectorSpec) error {
	collectorTypes := make(map[CollectorType]bool)
	for _, collector := range collectors {
		collectorTypes[collector.Type] = true
	}

	// Check dependencies
	for _, collector := range collectors {
		if rule, exists := re.expansionRules[collector.Type]; exists {
			for _, dependency := range rule.Dependencies {
				if !collectorTypes[dependency] {
					return fmt.Errorf("collector %s requires dependency %s which is not present",
						collector.Type, dependency)
				}
			}
		}
	}

	return nil
}

// GetCollectorPriority returns the priority for a collector type
func (re *ResourceExpander) GetCollectorPriority(collectorType CollectorType) int {
	if rule, exists := re.expansionRules[collectorType]; exists {
		return rule.Priority
	}
	return 0
}

// DeduplicateCollectors removes duplicate collectors based on their unique key
func (re *ResourceExpander) DeduplicateCollectors(collectors []CollectorSpec) []CollectorSpec {
	seen := make(map[string]bool)
	var deduplicated []CollectorSpec

	for _, collector := range collectors {
		key := collector.GetUniqueKey()
		if !seen[key] {
			seen[key] = true
			deduplicated = append(deduplicated, collector)
		} else {
			klog.V(4).Infof("Duplicate collector filtered: %s", key)
		}
	}

	return deduplicated
}

// FilterCollectorsByNamespace filters collectors to only include those for specified namespaces
func (re *ResourceExpander) FilterCollectorsByNamespace(collectors []CollectorSpec, targetNamespaces []string) []CollectorSpec {
	if len(targetNamespaces) == 0 {
		return collectors
	}

	namespaceSet := make(map[string]bool)
	for _, ns := range targetNamespaces {
		namespaceSet[ns] = true
	}

	var filtered []CollectorSpec
	for _, collector := range collectors {
		// Include cluster-scoped collectors (empty namespace)
		if collector.Namespace == "" {
			filtered = append(filtered, collector)
			continue
		}

		// Include namespace-scoped collectors for target namespaces
		if namespaceSet[collector.Namespace] {
			filtered = append(filtered, collector)
		}
	}

	return filtered
}

// GetCollectorTypesForNamespace returns the collector types that should be generated for a namespace
func (re *ResourceExpander) GetCollectorTypesForNamespace(namespace string, opts DiscoveryOptions) []CollectorType {
	var types []CollectorType

	// Standard namespace-scoped collectors
	types = append(types,
		CollectorTypeLogs,
		CollectorTypeConfigMaps,
		CollectorTypeSecrets,
		CollectorTypeEvents,
	)

	// Optional collectors based on options
	if opts.IncludeImages {
		types = append(types, CollectorTypeImageFacts)
	}

	return types
}
