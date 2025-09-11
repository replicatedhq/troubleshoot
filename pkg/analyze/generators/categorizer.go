package generators

import (
	"sort"
	"strings"
)

// RequirementCategorizer categorizes requirements into different types
type RequirementCategorizer struct {
	categories map[RequirementCategory]CategoryInfo
}

// CategoryInfo contains information about a requirement category
type CategoryInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Priority    int      `json:"priority"`
	Keywords    []string `json:"keywords"`
	Analyzers   []string `json:"analyzers"`
}

// CategorizedRequirement represents a requirement with its category information
type CategorizedRequirement struct {
	Path     string              `json:"path"`
	Category RequirementCategory `json:"category"`
	Priority RequirementPriority `json:"priority"`
	Tags     []string            `json:"tags"`
	Keywords []string            `json:"keywords"`
	Value    interface{}         `json:"value"`
}

// NewRequirementCategorizer creates a new requirement categorizer
func NewRequirementCategorizer() *RequirementCategorizer {
	return &RequirementCategorizer{
		categories: getDefaultCategories(),
	}
}

// CategorizeSpec categorizes all requirements in a specification
func (c *RequirementCategorizer) CategorizeSpec(spec *RequirementSpec) ([]CategorizedRequirement, error) {
	var categorized []CategorizedRequirement

	// Categorize Kubernetes requirements
	kubernetesReqs := c.categorizeKubernetesRequirements(&spec.Spec.Kubernetes)
	categorized = append(categorized, kubernetesReqs...)

	// Categorize Resource requirements
	resourceReqs := c.categorizeResourceRequirements(&spec.Spec.Resources)
	categorized = append(categorized, resourceReqs...)

	// Categorize Storage requirements
	storageReqs := c.categorizeStorageRequirements(&spec.Spec.Storage)
	categorized = append(categorized, storageReqs...)

	// Categorize Network requirements
	networkReqs := c.categorizeNetworkRequirements(&spec.Spec.Network)
	categorized = append(categorized, networkReqs...)

	// Categorize Security requirements
	securityReqs := c.categorizeSecurityRequirements(&spec.Spec.Security)
	categorized = append(categorized, securityReqs...)

	// Categorize Custom requirements
	customReqs := c.categorizeCustomRequirements(spec.Spec.Custom)
	categorized = append(categorized, customReqs...)

	// Categorize Vendor requirements
	vendorReqs := c.categorizeVendorRequirements(&spec.Spec.Vendor)
	categorized = append(categorized, vendorReqs...)

	// Categorize Replicated requirements
	replicatedReqs := c.categorizeReplicatedRequirements(&spec.Spec.Replicated)
	categorized = append(categorized, replicatedReqs...)

	// Sort by priority
	sort.Slice(categorized, func(i, j int) bool {
		return c.getPriorityValue(categorized[i].Priority) < c.getPriorityValue(categorized[j].Priority)
	})

	return categorized, nil
}

// categorizeKubernetesRequirements categorizes Kubernetes requirements
func (c *RequirementCategorizer) categorizeKubernetesRequirements(k8s *KubernetesRequirements) []CategorizedRequirement {
	var reqs []CategorizedRequirement

	// Version requirements
	if k8s.MinVersion != "" {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "kubernetes.minVersion",
			Category: CategoryKubernetes,
			Priority: PriorityRequired,
			Tags:     []string{"version", "compatibility"},
			Keywords: []string{"kubernetes", "version", "minimum"},
			Value:    k8s.MinVersion,
		})
	}

	if k8s.MaxVersion != "" {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "kubernetes.maxVersion",
			Category: CategoryKubernetes,
			Priority: PriorityRequired,
			Tags:     []string{"version", "compatibility"},
			Keywords: []string{"kubernetes", "version", "maximum"},
			Value:    k8s.MaxVersion,
		})
	}

	// Features
	for i, feature := range k8s.Features {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "kubernetes.features[" + string(rune(i)) + "]",
			Category: CategoryKubernetes,
			Priority: PriorityRecommended,
			Tags:     []string{"feature", "capability"},
			Keywords: []string{"kubernetes", "feature", strings.ToLower(feature)},
			Value:    feature,
		})
	}

	// APIs
	for i, api := range k8s.APIs {
		priority := PriorityOptional
		if api.Required {
			priority = PriorityRequired
		}

		reqs = append(reqs, CategorizedRequirement{
			Path:     "kubernetes.apis[" + string(rune(i)) + "]",
			Category: CategoryKubernetes,
			Priority: priority,
			Tags:     []string{"api", "capability"},
			Keywords: []string{"kubernetes", "api", strings.ToLower(api.Kind)},
			Value:    api,
		})
	}

	// Node count
	if k8s.NodeCount.Min > 0 {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "kubernetes.nodeCount.min",
			Category: CategoryResources,
			Priority: PriorityRequired,
			Tags:     []string{"nodes", "scaling"},
			Keywords: []string{"kubernetes", "nodes", "minimum", "scaling"},
			Value:    k8s.NodeCount.Min,
		})
	}

	return reqs
}

// categorizeResourceRequirements categorizes resource requirements
func (c *RequirementCategorizer) categorizeResourceRequirements(resources *ResourceRequirements) []CategorizedRequirement {
	var reqs []CategorizedRequirement

	// CPU requirements
	if resources.CPU.MinCores > 0 {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "resources.cpu.minCores",
			Category: CategoryResources,
			Priority: PriorityRequired,
			Tags:     []string{"cpu", "compute"},
			Keywords: []string{"cpu", "cores", "compute", "performance"},
			Value:    resources.CPU.MinCores,
		})
	}

	if resources.CPU.MaxUtilization > 0 {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "resources.cpu.maxUtilization",
			Category: CategoryResources,
			Priority: PriorityRecommended,
			Tags:     []string{"cpu", "performance"},
			Keywords: []string{"cpu", "utilization", "performance", "monitoring"},
			Value:    resources.CPU.MaxUtilization,
		})
	}

	// Memory requirements
	if resources.Memory.MinBytes > 0 {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "resources.memory.minBytes",
			Category: CategoryResources,
			Priority: PriorityRequired,
			Tags:     []string{"memory", "compute"},
			Keywords: []string{"memory", "ram", "compute", "performance"},
			Value:    resources.Memory.MinBytes,
		})
	}

	if resources.Memory.MaxUtilization > 0 {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "resources.memory.maxUtilization",
			Category: CategoryResources,
			Priority: PriorityRecommended,
			Tags:     []string{"memory", "performance"},
			Keywords: []string{"memory", "utilization", "performance", "monitoring"},
			Value:    resources.Memory.MaxUtilization,
		})
	}

	// Node requirements
	if resources.Nodes.MinNodes > 0 {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "resources.nodes.minNodes",
			Category: CategoryResources,
			Priority: PriorityRequired,
			Tags:     []string{"nodes", "scaling"},
			Keywords: []string{"nodes", "minimum", "scaling", "cluster"},
			Value:    resources.Nodes.MinNodes,
		})
	}

	// Node selectors
	for key, value := range resources.Nodes.NodeSelectors {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "resources.nodes.nodeSelectors." + key,
			Category: CategoryResources,
			Priority: PriorityRecommended,
			Tags:     []string{"nodes", "scheduling"},
			Keywords: []string{"nodes", "selector", "scheduling", "placement"},
			Value:    value,
		})
	}

	return reqs
}

// categorizeStorageRequirements categorizes storage requirements
func (c *RequirementCategorizer) categorizeStorageRequirements(storage *StorageRequirements) []CategorizedRequirement {
	var reqs []CategorizedRequirement

	// Capacity requirements
	if storage.MinCapacity > 0 {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "storage.minCapacity",
			Category: CategoryStorage,
			Priority: PriorityRequired,
			Tags:     []string{"storage", "capacity"},
			Keywords: []string{"storage", "capacity", "disk", "volume"},
			Value:    storage.MinCapacity,
		})
	}

	// Storage classes
	for i, sc := range storage.StorageClasses {
		priority := PriorityOptional
		if sc.Required {
			priority = PriorityRequired
		}
		if sc.Default {
			priority = PriorityRecommended
		}

		reqs = append(reqs, CategorizedRequirement{
			Path:     "storage.storageClasses[" + string(rune(i)) + "]",
			Category: CategoryStorage,
			Priority: priority,
			Tags:     []string{"storage", "storageclass"},
			Keywords: []string{"storage", "class", "provisioner", sc.Provisioner},
			Value:    sc,
		})
	}

	// Performance requirements
	if storage.Performance.MinIOPS > 0 {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "storage.performance.minIOPS",
			Category: CategoryStorage,
			Priority: PriorityRecommended,
			Tags:     []string{"storage", "performance"},
			Keywords: []string{"storage", "performance", "iops", "throughput"},
			Value:    storage.Performance.MinIOPS,
		})
	}

	// Backup requirements
	if storage.Backup.Required {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "storage.backup",
			Category: CategoryStorage,
			Priority: PriorityRequired,
			Tags:     []string{"storage", "backup"},
			Keywords: []string{"storage", "backup", "disaster", "recovery"},
			Value:    storage.Backup,
		})
	}

	return reqs
}

// categorizeNetworkRequirements categorizes network requirements
func (c *RequirementCategorizer) categorizeNetworkRequirements(network *NetworkRequirements) []CategorizedRequirement {
	var reqs []CategorizedRequirement

	// Connectivity requirements
	for i, conn := range network.Connectivity {
		priority := PriorityOptional
		if conn.Required {
			priority = PriorityRequired
		}

		reqs = append(reqs, CategorizedRequirement{
			Path:     "network.connectivity[" + string(rune(i)) + "]",
			Category: CategoryNetwork,
			Priority: priority,
			Tags:     []string{"network", "connectivity"},
			Keywords: []string{"network", "connectivity", strings.ToLower(conn.Type)},
			Value:    conn,
		})
	}

	// Bandwidth requirements
	if network.Bandwidth.MinUpload > 0 || network.Bandwidth.MinDownload > 0 {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "network.bandwidth",
			Category: CategoryNetwork,
			Priority: PriorityRecommended,
			Tags:     []string{"network", "bandwidth"},
			Keywords: []string{"network", "bandwidth", "throughput", "performance"},
			Value:    network.Bandwidth,
		})
	}

	// Security requirements
	if network.Security.TLSRequired {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "network.security.tls",
			Category: CategorySecurity,
			Priority: PriorityRequired,
			Tags:     []string{"network", "security", "encryption"},
			Keywords: []string{"network", "security", "tls", "encryption"},
			Value:    network.Security,
		})
	}

	// DNS requirements
	if network.DNS.Required {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "network.dns",
			Category: CategoryNetwork,
			Priority: PriorityRequired,
			Tags:     []string{"network", "dns"},
			Keywords: []string{"network", "dns", "resolution", "discovery"},
			Value:    network.DNS,
		})
	}

	return reqs
}

// categorizeSecurityRequirements categorizes security requirements
func (c *RequirementCategorizer) categorizeSecurityRequirements(security *SecurityRequirements) []CategorizedRequirement {
	var reqs []CategorizedRequirement

	// RBAC requirements
	if security.RBAC.Required {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "security.rbac",
			Category: CategorySecurity,
			Priority: PriorityRequired,
			Tags:     []string{"security", "rbac", "authorization"},
			Keywords: []string{"security", "rbac", "authorization", "permissions"},
			Value:    security.RBAC,
		})
	}

	// Pod security requirements
	if len(security.PodSecurity.Standards) > 0 {
		for i, standard := range security.PodSecurity.Standards {
			reqs = append(reqs, CategorizedRequirement{
				Path:     "security.podSecurity.standards[" + string(rune(i)) + "]",
				Category: CategorySecurity,
				Priority: PriorityRequired,
				Tags:     []string{"security", "pods"},
				Keywords: []string{"security", "pod", "standard", standard},
				Value:    standard,
			})
		}
	}

	// Encryption requirements
	if security.Encryption.AtRest.Required {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "security.encryption.atRest",
			Category: CategorySecurity,
			Priority: PriorityRequired,
			Tags:     []string{"security", "encryption"},
			Keywords: []string{"security", "encryption", "rest", "data"},
			Value:    security.Encryption.AtRest,
		})
	}

	if security.Encryption.InTransit.Required {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "security.encryption.inTransit",
			Category: CategorySecurity,
			Priority: PriorityRequired,
			Tags:     []string{"security", "encryption"},
			Keywords: []string{"security", "encryption", "transit", "tls"},
			Value:    security.Encryption.InTransit,
		})
	}

	// Compliance requirements
	if len(security.Compliance.Standards) > 0 {
		for i, standard := range security.Compliance.Standards {
			reqs = append(reqs, CategorizedRequirement{
				Path:     "security.compliance.standards[" + string(rune(i)) + "]",
				Category: CategorySecurity,
				Priority: PriorityRequired,
				Tags:     []string{"security", "compliance"},
				Keywords: []string{"security", "compliance", strings.ToLower(standard)},
				Value:    standard,
			})
		}
	}

	return reqs
}

// categorizeCustomRequirements categorizes custom requirements
func (c *RequirementCategorizer) categorizeCustomRequirements(customs []CustomRequirement) []CategorizedRequirement {
	var reqs []CategorizedRequirement

	for i, custom := range customs {
		priority := PriorityOptional
		if custom.Required {
			priority = PriorityRequired
		}

		// Determine category based on type
		category := CategoryCustom
		if custom.Type != "" {
			switch strings.ToLower(custom.Type) {
			case "kubernetes", "k8s":
				category = CategoryKubernetes
			case "resource", "compute":
				category = CategoryResources
			case "storage", "disk":
				category = CategoryStorage
			case "network", "networking":
				category = CategoryNetwork
			case "security", "auth":
				category = CategorySecurity
			case "vendor":
				category = CategoryVendor
			}
		}

		keywords := []string{"custom", strings.ToLower(custom.Type), strings.ToLower(custom.Name)}

		reqs = append(reqs, CategorizedRequirement{
			Path:     "custom[" + string(rune(i)) + "]",
			Category: category,
			Priority: priority,
			Tags:     []string{"custom", custom.Type},
			Keywords: keywords,
			Value:    custom,
		})
	}

	return reqs
}

// categorizeVendorRequirements categorizes vendor-specific requirements
func (c *RequirementCategorizer) categorizeVendorRequirements(vendor *VendorRequirements) []CategorizedRequirement {
	var reqs []CategorizedRequirement

	// AWS requirements
	if vendor.AWS.EKS.Version != "" {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "vendor.aws.eks",
			Category: CategoryVendor,
			Priority: PriorityRequired,
			Tags:     []string{"vendor", "aws", "eks"},
			Keywords: []string{"vendor", "aws", "eks", "kubernetes"},
			Value:    vendor.AWS.EKS,
		})
	}

	// Azure requirements
	if vendor.Azure.AKS.Version != "" {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "vendor.azure.aks",
			Category: CategoryVendor,
			Priority: PriorityRequired,
			Tags:     []string{"vendor", "azure", "aks"},
			Keywords: []string{"vendor", "azure", "aks", "kubernetes"},
			Value:    vendor.Azure.AKS,
		})
	}

	// GCP requirements
	if vendor.GCP.GKE.Version != "" {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "vendor.gcp.gke",
			Category: CategoryVendor,
			Priority: PriorityRequired,
			Tags:     []string{"vendor", "gcp", "gke"},
			Keywords: []string{"vendor", "gcp", "gke", "kubernetes"},
			Value:    vendor.GCP.GKE,
		})
	}

	// OpenShift requirements
	if vendor.OpenShift.Version != "" {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "vendor.openshift",
			Category: CategoryVendor,
			Priority: PriorityRequired,
			Tags:     []string{"vendor", "redhat", "openshift"},
			Keywords: []string{"vendor", "redhat", "openshift", "kubernetes"},
			Value:    vendor.OpenShift,
		})
	}

	return reqs
}

// categorizeReplicatedRequirements categorizes Replicated-specific requirements
func (c *RequirementCategorizer) categorizeReplicatedRequirements(replicated *ReplicatedRequirements) []CategorizedRequirement {
	var reqs []CategorizedRequirement

	// KOTS requirements
	if replicated.KOTS.Version != "" {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "replicated.kots",
			Category: CategoryReplicated,
			Priority: PriorityRequired,
			Tags:     []string{"replicated", "kots"},
			Keywords: []string{"replicated", "kots", "application", "lifecycle"},
			Value:    replicated.KOTS,
		})
	}

	// Preflight requirements
	if replicated.Preflight.Required {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "replicated.preflight",
			Category: CategoryReplicated,
			Priority: PriorityRequired,
			Tags:     []string{"replicated", "preflight"},
			Keywords: []string{"replicated", "preflight", "validation", "checks"},
			Value:    replicated.Preflight,
		})
	}

	// Support bundle requirements
	if replicated.SupportBundle.Required {
		reqs = append(reqs, CategorizedRequirement{
			Path:     "replicated.supportBundle",
			Category: CategoryReplicated,
			Priority: PriorityRecommended,
			Tags:     []string{"replicated", "supportbundle"},
			Keywords: []string{"replicated", "support", "bundle", "diagnostics"},
			Value:    replicated.SupportBundle,
		})
	}

	return reqs
}

// GetCategoryInfo returns information about a specific category
func (c *RequirementCategorizer) GetCategoryInfo(category RequirementCategory) (CategoryInfo, bool) {
	info, exists := c.categories[category]
	return info, exists
}

// GetAllCategories returns all available categories
func (c *RequirementCategorizer) GetAllCategories() map[RequirementCategory]CategoryInfo {
	return c.categories
}

// FilterByCategory filters requirements by category
func (c *RequirementCategorizer) FilterByCategory(reqs []CategorizedRequirement, category RequirementCategory) []CategorizedRequirement {
	var filtered []CategorizedRequirement

	for _, req := range reqs {
		if req.Category == category {
			filtered = append(filtered, req)
		}
	}

	return filtered
}

// FilterByPriority filters requirements by priority
func (c *RequirementCategorizer) FilterByPriority(reqs []CategorizedRequirement, priority RequirementPriority) []CategorizedRequirement {
	var filtered []CategorizedRequirement

	for _, req := range reqs {
		if req.Priority == priority {
			filtered = append(filtered, req)
		}
	}

	return filtered
}

// FilterByTags filters requirements by tags
func (c *RequirementCategorizer) FilterByTags(reqs []CategorizedRequirement, tags []string) []CategorizedRequirement {
	var filtered []CategorizedRequirement

	for _, req := range reqs {
		if c.hasAnyTag(req.Tags, tags) {
			filtered = append(filtered, req)
		}
	}

	return filtered
}

// SearchByKeywords searches requirements by keywords
func (c *RequirementCategorizer) SearchByKeywords(reqs []CategorizedRequirement, keywords []string) []CategorizedRequirement {
	var matched []CategorizedRequirement

	for _, req := range reqs {
		if c.hasAnyKeyword(req.Keywords, keywords) {
			matched = append(matched, req)
		}
	}

	return matched
}

// hasAnyTag checks if requirement has any of the specified tags
func (c *RequirementCategorizer) hasAnyTag(reqTags, searchTags []string) bool {
	for _, searchTag := range searchTags {
		for _, reqTag := range reqTags {
			if strings.EqualFold(reqTag, searchTag) {
				return true
			}
		}
	}
	return false
}

// hasAnyKeyword checks if requirement has any of the specified keywords
func (c *RequirementCategorizer) hasAnyKeyword(reqKeywords, searchKeywords []string) bool {
	for _, searchKeyword := range searchKeywords {
		for _, reqKeyword := range reqKeywords {
			if strings.Contains(strings.ToLower(reqKeyword), strings.ToLower(searchKeyword)) {
				return true
			}
		}
	}
	return false
}

// getPriorityValue returns numeric value for priority comparison
func (c *RequirementCategorizer) getPriorityValue(priority RequirementPriority) int {
	switch priority {
	case PriorityRequired:
		return 1
	case PriorityRecommended:
		return 2
	case PriorityOptional:
		return 3
	case PriorityDeprecated:
		return 4
	default:
		return 3
	}
}

// getDefaultCategories returns the default category information
func getDefaultCategories() map[RequirementCategory]CategoryInfo {
	return map[RequirementCategory]CategoryInfo{
		CategoryKubernetes: {
			Name:        "Kubernetes",
			Description: "Kubernetes platform and API requirements",
			Priority:    1,
			Keywords:    []string{"kubernetes", "k8s", "api", "version", "feature", "distribution"},
			Analyzers:   []string{"clusterVersion", "api", "distribution", "nodeVersion"},
		},
		CategoryResources: {
			Name:        "Resources",
			Description: "Compute resource requirements (CPU, memory, nodes)",
			Priority:    2,
			Keywords:    []string{"cpu", "memory", "nodes", "compute", "resources", "capacity"},
			Analyzers:   []string{"nodeResources", "clusterResources", "cpuAnalyzer", "memoryAnalyzer"},
		},
		CategoryStorage: {
			Name:        "Storage",
			Description: "Storage and persistence requirements",
			Priority:    3,
			Keywords:    []string{"storage", "disk", "volume", "capacity", "performance", "backup"},
			Analyzers:   []string{"storageClass", "persistentVolume", "volumeSnapshot", "diskUsage"},
		},
		CategoryNetwork: {
			Name:        "Network",
			Description: "Networking and connectivity requirements",
			Priority:    4,
			Keywords:    []string{"network", "connectivity", "bandwidth", "latency", "dns", "proxy"},
			Analyzers:   []string{"clusterPodNetwork", "service", "ingress", "networkPolicy"},
		},
		CategorySecurity: {
			Name:        "Security",
			Description: "Security and compliance requirements",
			Priority:    5,
			Keywords:    []string{"security", "rbac", "encryption", "compliance", "audit", "policy"},
			Analyzers:   []string{"rbac", "podSecurityPolicy", "networkPolicy", "clusterRoleBinding"},
		},
		CategoryCustom: {
			Name:        "Custom",
			Description: "Custom application-specific requirements",
			Priority:    6,
			Keywords:    []string{"custom", "application", "specific", "business", "logic"},
			Analyzers:   []string{"customResourceDefinition", "configMap", "secret"},
		},
		CategoryVendor: {
			Name:        "Vendor",
			Description: "Cloud provider and vendor-specific requirements",
			Priority:    7,
			Keywords:    []string{"vendor", "aws", "azure", "gcp", "cloud", "provider", "managed"},
			Analyzers:   []string{"distribution", "clusterVersion", "nodeVersion"},
		},
		CategoryReplicated: {
			Name:        "Replicated",
			Description: "Replicated platform-specific requirements",
			Priority:    8,
			Keywords:    []string{"replicated", "kots", "preflight", "supportbundle", "license"},
			Analyzers:   []string{"kotsadm", "license", "registry", "imagePullSecret"},
		},
	}
}
