package generators

import (
	"fmt"
	"sort"
	"strings"
)

// ConflictResolver identifies and resolves conflicts between requirements
type ConflictResolver struct {
	rules []ConflictRule
}

// ConflictRule defines how to identify and resolve conflicts
type ConflictRule struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Pattern     ConflictPattern    `json:"pattern"`
	Resolution  ConflictResolution `json:"resolution"`
	Severity    ConflictSeverity   `json:"severity"`
}

// ConflictPattern defines how to identify conflicts
type ConflictPattern struct {
	Type      ConflictType `json:"type"`
	Paths     []string     `json:"paths"`
	Keywords  []string     `json:"keywords"`
	Condition string       `json:"condition"`
}

// ConflictResolution defines how to resolve conflicts
type ConflictResolution struct {
	Strategy    ResolutionStrategy `json:"strategy"`
	Priority    string             `json:"priority"`
	Action      string             `json:"action"`
	Message     string             `json:"message"`
	Alternative string             `json:"alternative,omitempty"`
}

// Conflict represents a detected conflict between requirements
type Conflict struct {
	ID           string                   `json:"id"`
	Type         ConflictType             `json:"type"`
	Severity     ConflictSeverity         `json:"severity"`
	Description  string                   `json:"description"`
	Requirements []CategorizedRequirement `json:"requirements"`
	Resolution   *ConflictResolution      `json:"resolution,omitempty"`
	Resolved     bool                     `json:"resolved"`
}

// ConflictType defines the type of conflict
type ConflictType string

const (
	ConflictTypeExclusive     ConflictType = "exclusive"     // Requirements are mutually exclusive
	ConflictTypeVersionRange  ConflictType = "version_range" // Version ranges are incompatible
	ConflictTypeResource      ConflictType = "resource"      // Resource requirements conflict
	ConflictTypeCapability    ConflictType = "capability"    // Capability requirements conflict
	ConflictTypeDependency    ConflictType = "dependency"    // Dependency conflicts
	ConflictTypeConfiguration ConflictType = "configuration" // Configuration conflicts
)

// ConflictSeverity defines the severity of conflicts
type ConflictSeverity string

const (
	ConflictSeverityCritical ConflictSeverity = "critical" // Cannot be resolved automatically
	ConflictSeverityMajor    ConflictSeverity = "major"    // Requires manual intervention
	ConflictSeverityMinor    ConflictSeverity = "minor"    // Can be resolved with warnings
	ConflictSeverityInfo     ConflictSeverity = "info"     // Informational conflicts
)

// ResolutionStrategy defines how to resolve conflicts
type ResolutionStrategy string

const (
	StrategyHighestPriority ResolutionStrategy = "highest_priority" // Use highest priority requirement
	StrategyMerge           ResolutionStrategy = "merge"            // Merge requirements
	StrategyOverride        ResolutionStrategy = "override"         // Override with specific value
	StrategyFail            ResolutionStrategy = "fail"             // Fail with error
	StrategyWarn            ResolutionStrategy = "warn"             // Warn and continue
	StrategyAlternative     ResolutionStrategy = "alternative"      // Suggest alternative
)

// NewConflictResolver creates a new conflict resolver with default rules
func NewConflictResolver() *ConflictResolver {
	return &ConflictResolver{
		rules: getDefaultConflictRules(),
	}
}

// DetectConflicts detects conflicts in categorized requirements
func (r *ConflictResolver) DetectConflicts(reqs []CategorizedRequirement) ([]Conflict, error) {
	var conflicts []Conflict

	// Version range conflicts
	versionConflicts := r.detectVersionConflicts(reqs)
	conflicts = append(conflicts, versionConflicts...)

	// Resource conflicts
	resourceConflicts := r.detectResourceConflicts(reqs)
	conflicts = append(conflicts, resourceConflicts...)

	// Exclusive requirement conflicts
	exclusiveConflicts := r.detectExclusiveConflicts(reqs)
	conflicts = append(conflicts, exclusiveConflicts...)

	// Dependency conflicts
	dependencyConflicts := r.detectDependencyConflicts(reqs)
	conflicts = append(conflicts, dependencyConflicts...)

	// Configuration conflicts
	configConflicts := r.detectConfigurationConflicts(reqs)
	conflicts = append(conflicts, configConflicts...)

	// Sort by severity
	sort.Slice(conflicts, func(i, j int) bool {
		return r.getSeverityValue(conflicts[i].Severity) < r.getSeverityValue(conflicts[j].Severity)
	})

	return conflicts, nil
}

// ResolveConflicts resolves conflicts based on resolution strategies
func (r *ConflictResolver) ResolveConflicts(reqs []CategorizedRequirement, conflicts []Conflict) ([]CategorizedRequirement, []Conflict, error) {
	resolved := make([]CategorizedRequirement, 0, len(reqs))
	unresolvedConflicts := make([]Conflict, 0)

	// Create a map for quick lookup
	reqMap := make(map[string]CategorizedRequirement)
	for _, req := range reqs {
		reqMap[req.Path] = req
	}

	// Track which requirements have been processed
	processed := make(map[string]bool)

	for _, conflict := range conflicts {
		if conflict.Resolution == nil {
			// No resolution strategy, add to unresolved
			unresolvedConflicts = append(unresolvedConflicts, conflict)
			continue
		}

		switch conflict.Resolution.Strategy {
		case StrategyHighestPriority:
			resolvedReq, err := r.resolveByPriority(conflict.Requirements)
			if err != nil {
				unresolvedConflicts = append(unresolvedConflicts, conflict)
				continue
			}

			// Mark conflicting requirements as processed
			for _, req := range conflict.Requirements {
				processed[req.Path] = true
			}

			// Add the resolved requirement
			resolved = append(resolved, resolvedReq)
			conflict.Resolved = true

		case StrategyMerge:
			mergedReq, err := r.mergeRequirements(conflict.Requirements)
			if err != nil {
				unresolvedConflicts = append(unresolvedConflicts, conflict)
				continue
			}

			// Mark conflicting requirements as processed
			for _, req := range conflict.Requirements {
				processed[req.Path] = true
			}

			// Add the merged requirement
			resolved = append(resolved, mergedReq)
			conflict.Resolved = true

		case StrategyOverride:
			overriddenReq, err := r.overrideRequirement(conflict.Requirements, conflict.Resolution.Alternative)
			if err != nil {
				unresolvedConflicts = append(unresolvedConflicts, conflict)
				continue
			}

			// Mark conflicting requirements as processed
			for _, req := range conflict.Requirements {
				processed[req.Path] = true
			}

			// Add the overridden requirement
			resolved = append(resolved, overriddenReq)
			conflict.Resolved = true

		case StrategyWarn:
			// Keep all requirements but add warning
			for _, req := range conflict.Requirements {
				if !processed[req.Path] {
					resolved = append(resolved, req)
					processed[req.Path] = true
				}
			}
			conflict.Resolved = true
			unresolvedConflicts = append(unresolvedConflicts, conflict)

		case StrategyFail:
			// Cannot be resolved
			unresolvedConflicts = append(unresolvedConflicts, conflict)

		case StrategyAlternative:
			// Suggest alternative but keep original
			for _, req := range conflict.Requirements {
				if !processed[req.Path] {
					resolved = append(resolved, req)
					processed[req.Path] = true
				}
			}
			conflict.Resolved = true
			unresolvedConflicts = append(unresolvedConflicts, conflict)
		}
	}

	// Add non-conflicting requirements
	for _, req := range reqs {
		if !processed[req.Path] {
			resolved = append(resolved, req)
		}
	}

	return resolved, unresolvedConflicts, nil
}

// detectVersionConflicts detects version range conflicts
func (r *ConflictResolver) detectVersionConflicts(reqs []CategorizedRequirement) []Conflict {
	var conflicts []Conflict
	versionGroups := make(map[string][]CategorizedRequirement)

	// Group requirements by component
	for _, req := range reqs {
		if r.isVersionRequirement(req) {
			component := r.extractComponentName(req.Path)
			versionGroups[component] = append(versionGroups[component], req)
		}
	}

	// Check for conflicts within each group
	for component, group := range versionGroups {
		if len(group) > 1 {
			// Check if version ranges are incompatible
			if r.hasIncompatibleVersions(group) {
				conflicts = append(conflicts, Conflict{
					ID:           fmt.Sprintf("version_conflict_%s", component),
					Type:         ConflictTypeVersionRange,
					Severity:     ConflictSeverityMajor,
					Description:  fmt.Sprintf("Incompatible version requirements for %s", component),
					Requirements: group,
					Resolution: &ConflictResolution{
						Strategy: StrategyHighestPriority,
						Priority: "required",
						Action:   "use_highest_version",
						Message:  fmt.Sprintf("Using highest priority version requirement for %s", component),
					},
				})
			}
		}
	}

	return conflicts
}

// detectResourceConflicts detects resource requirement conflicts
func (r *ConflictResolver) detectResourceConflicts(reqs []CategorizedRequirement) []Conflict {
	var conflicts []Conflict
	resourceGroups := make(map[string][]CategorizedRequirement)

	// Group requirements by resource type
	for _, req := range reqs {
		if req.Category == CategoryResources {
			resourceType := r.extractResourceType(req.Path)
			resourceGroups[resourceType] = append(resourceGroups[resourceType], req)
		}
	}

	// Check for conflicts within each resource type
	for resourceType, group := range resourceGroups {
		if len(group) > 1 {
			// Check for conflicting resource values
			if r.hasConflictingResourceValues(group) {
				severity := ConflictSeverityMinor
				if r.isCriticalResource(resourceType) {
					severity = ConflictSeverityMajor
				}

				conflicts = append(conflicts, Conflict{
					ID:           fmt.Sprintf("resource_conflict_%s", resourceType),
					Type:         ConflictTypeResource,
					Severity:     severity,
					Description:  fmt.Sprintf("Conflicting %s requirements", resourceType),
					Requirements: group,
					Resolution: &ConflictResolution{
						Strategy: StrategyMerge,
						Priority: "required",
						Action:   "use_maximum",
						Message:  fmt.Sprintf("Using maximum %s requirement", resourceType),
					},
				})
			}
		}
	}

	return conflicts
}

// detectExclusiveConflicts detects mutually exclusive requirements
func (r *ConflictResolver) detectExclusiveConflicts(reqs []CategorizedRequirement) []Conflict {
	var conflicts []Conflict

	// Define exclusive requirement patterns
	exclusivePatterns := map[string][]string{
		"storage_class": {"storage.storageClasses"},
		"node_selector": {"resources.nodes.nodeSelectors"},
		"vendor":        {"vendor.aws", "vendor.azure", "vendor.gcp"},
	}

	for conflictName, patterns := range exclusivePatterns {
		var matchingReqs []CategorizedRequirement

		for _, req := range reqs {
			for _, pattern := range patterns {
				if strings.Contains(req.Path, pattern) {
					matchingReqs = append(matchingReqs, req)
					break
				}
			}
		}

		if len(matchingReqs) > 1 && r.areRequirementsExclusive(matchingReqs) {
			conflicts = append(conflicts, Conflict{
				ID:           fmt.Sprintf("exclusive_conflict_%s", conflictName),
				Type:         ConflictTypeExclusive,
				Severity:     ConflictSeverityMajor,
				Description:  fmt.Sprintf("Mutually exclusive %s requirements", conflictName),
				Requirements: matchingReqs,
				Resolution: &ConflictResolution{
					Strategy: StrategyHighestPriority,
					Priority: "required",
					Action:   "choose_one",
					Message:  fmt.Sprintf("Choosing highest priority %s requirement", conflictName),
				},
			})
		}
	}

	return conflicts
}

// detectDependencyConflicts detects dependency conflicts
func (r *ConflictResolver) detectDependencyConflicts(reqs []CategorizedRequirement) []Conflict {
	var conflicts []Conflict

	// Define dependency rules
	dependencies := map[string][]string{
		"kubernetes.features": {"kubernetes.minVersion"},
		"storage.backup":      {"storage.minCapacity"},
		"network.security":    {"security.encryption"},
	}

	for dependent, requirements := range dependencies {
		var dependentReqs []CategorizedRequirement
		var requiredReqs []CategorizedRequirement

		// Find dependent and required requirements
		for _, req := range reqs {
			if strings.Contains(req.Path, dependent) {
				dependentReqs = append(dependentReqs, req)
			}
			for _, required := range requirements {
				if strings.Contains(req.Path, required) {
					requiredReqs = append(requiredReqs, req)
				}
			}
		}

		// Check if dependent requirements exist but required ones are missing
		if len(dependentReqs) > 0 && len(requiredReqs) == 0 {
			conflicts = append(conflicts, Conflict{
				ID:           fmt.Sprintf("dependency_conflict_%s", strings.ReplaceAll(dependent, ".", "_")),
				Type:         ConflictTypeDependency,
				Severity:     ConflictSeverityMajor,
				Description:  fmt.Sprintf("Missing required dependencies for %s", dependent),
				Requirements: dependentReqs,
				Resolution: &ConflictResolution{
					Strategy:    StrategyAlternative,
					Priority:    "required",
					Action:      "add_dependency",
					Message:     fmt.Sprintf("Add required dependencies: %v", requirements),
					Alternative: strings.Join(requirements, ", "),
				},
			})
		}
	}

	return conflicts
}

// detectConfigurationConflicts detects configuration conflicts
func (r *ConflictResolver) detectConfigurationConflicts(reqs []CategorizedRequirement) []Conflict {
	var conflicts []Conflict

	// Define configuration conflict patterns
	configPatterns := map[string][]string{
		"tls":  {"network.security.tls", "security.encryption.inTransit"},
		"rbac": {"security.rbac", "kubernetes.features"},
	}

	for configName, patterns := range configPatterns {
		var matchingReqs []CategorizedRequirement

		for _, req := range reqs {
			for _, pattern := range patterns {
				if strings.Contains(req.Path, pattern) {
					matchingReqs = append(matchingReqs, req)
					break
				}
			}
		}

		if len(matchingReqs) > 1 && r.hasConfigurationConflict(matchingReqs) {
			conflicts = append(conflicts, Conflict{
				ID:           fmt.Sprintf("config_conflict_%s", configName),
				Type:         ConflictTypeConfiguration,
				Severity:     ConflictSeverityMinor,
				Description:  fmt.Sprintf("Configuration conflict in %s settings", configName),
				Requirements: matchingReqs,
				Resolution: &ConflictResolution{
					Strategy: StrategyMerge,
					Priority: "recommended",
					Action:   "merge_config",
					Message:  fmt.Sprintf("Merging %s configuration", configName),
				},
			})
		}
	}

	return conflicts
}

// Helper methods

func (r *ConflictResolver) isVersionRequirement(req CategorizedRequirement) bool {
	return strings.Contains(req.Path, "version") ||
		strings.Contains(req.Path, "Version") ||
		strings.Contains(strings.ToLower(req.Path), "minversion") ||
		strings.Contains(strings.ToLower(req.Path), "maxversion")
}

func (r *ConflictResolver) extractComponentName(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) > 1 {
		return parts[0]
	}
	return path
}

func (r *ConflictResolver) extractResourceType(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) > 2 {
		return parts[1]
	}
	return "unknown"
}

func (r *ConflictResolver) hasIncompatibleVersions(reqs []CategorizedRequirement) bool {
	// Simple heuristic: if we have both min and max versions, check compatibility
	var minVersion, maxVersion string

	for _, req := range reqs {
		if strings.Contains(strings.ToLower(req.Path), "min") {
			if v, ok := req.Value.(string); ok {
				minVersion = v
			}
		}
		if strings.Contains(strings.ToLower(req.Path), "max") {
			if v, ok := req.Value.(string); ok {
				maxVersion = v
			}
		}
	}

	// If we have both min and max versions, they might be incompatible
	return minVersion != "" && maxVersion != "" && minVersion != maxVersion
}

func (r *ConflictResolver) hasConflictingResourceValues(reqs []CategorizedRequirement) bool {
	// Check if requirements have different values for the same resource
	values := make(map[interface{}]int)
	for _, req := range reqs {
		if req.Value != nil {
			values[req.Value]++
		}
	}
	return len(values) > 1
}

func (r *ConflictResolver) isCriticalResource(resourceType string) bool {
	criticalResources := []string{"cpu", "memory", "nodes"}
	for _, critical := range criticalResources {
		if strings.Contains(strings.ToLower(resourceType), critical) {
			return true
		}
	}
	return false
}

func (r *ConflictResolver) areRequirementsExclusive(reqs []CategorizedRequirement) bool {
	// Simple heuristic: requirements are exclusive if they have different priorities
	// and are in the same category
	if len(reqs) < 2 {
		return false
	}

	category := reqs[0].Category
	for _, req := range reqs[1:] {
		if req.Category != category {
			return true // Different categories might be exclusive
		}
	}
	return false
}

func (r *ConflictResolver) hasConfigurationConflict(reqs []CategorizedRequirement) bool {
	// Check if requirements have conflicting configuration values
	for i, req1 := range reqs {
		for j, req2 := range reqs[i+1:] {
			_ = j // avoid unused variable error
			if r.areConfigurationsConflicting(req1, req2) {
				return true
			}
		}
	}
	return false
}

func (r *ConflictResolver) areConfigurationsConflicting(req1, req2 CategorizedRequirement) bool {
	// Simple heuristic: if both requirements have boolean values that are different
	if v1, ok1 := req1.Value.(bool); ok1 {
		if v2, ok2 := req2.Value.(bool); ok2 {
			return v1 != v2
		}
	}
	return false
}

func (r *ConflictResolver) resolveByPriority(reqs []CategorizedRequirement) (CategorizedRequirement, error) {
	if len(reqs) == 0 {
		return CategorizedRequirement{}, fmt.Errorf("no requirements to resolve")
	}

	// Sort by priority (required > recommended > optional)
	sort.Slice(reqs, func(i, j int) bool {
		return r.getPriorityValue(reqs[i].Priority) < r.getPriorityValue(reqs[j].Priority)
	})

	return reqs[0], nil
}

func (r *ConflictResolver) mergeRequirements(reqs []CategorizedRequirement) (CategorizedRequirement, error) {
	if len(reqs) == 0 {
		return CategorizedRequirement{}, fmt.Errorf("no requirements to merge")
	}

	merged := reqs[0]
	merged.Path = "merged_" + merged.Path

	// Merge tags and keywords
	allTags := make(map[string]bool)
	allKeywords := make(map[string]bool)

	for _, req := range reqs {
		for _, tag := range req.Tags {
			allTags[tag] = true
		}
		for _, keyword := range req.Keywords {
			allKeywords[keyword] = true
		}
	}

	// Convert maps to slices
	merged.Tags = make([]string, 0, len(allTags))
	for tag := range allTags {
		merged.Tags = append(merged.Tags, tag)
	}

	merged.Keywords = make([]string, 0, len(allKeywords))
	for keyword := range allKeywords {
		merged.Keywords = append(merged.Keywords, keyword)
	}

	return merged, nil
}

func (r *ConflictResolver) overrideRequirement(reqs []CategorizedRequirement, override string) (CategorizedRequirement, error) {
	if len(reqs) == 0 {
		return CategorizedRequirement{}, fmt.Errorf("no requirements to override")
	}

	overridden := reqs[0]
	overridden.Path = "overridden_" + overridden.Path
	overridden.Value = override

	return overridden, nil
}

func (r *ConflictResolver) getPriorityValue(priority RequirementPriority) int {
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

func (r *ConflictResolver) getSeverityValue(severity ConflictSeverity) int {
	switch severity {
	case ConflictSeverityCritical:
		return 1
	case ConflictSeverityMajor:
		return 2
	case ConflictSeverityMinor:
		return 3
	case ConflictSeverityInfo:
		return 4
	default:
		return 3
	}
}

// getDefaultConflictRules returns default conflict resolution rules
func getDefaultConflictRules() []ConflictRule {
	return []ConflictRule{
		{
			Name:        "kubernetes_version_conflict",
			Description: "Resolve Kubernetes version range conflicts",
			Pattern: ConflictPattern{
				Type:     ConflictTypeVersionRange,
				Paths:    []string{"kubernetes.minVersion", "kubernetes.maxVersion"},
				Keywords: []string{"version", "kubernetes"},
			},
			Resolution: ConflictResolution{
				Strategy: StrategyHighestPriority,
				Priority: "required",
				Action:   "use_intersection",
				Message:  "Using compatible version range",
			},
			Severity: ConflictSeverityMajor,
		},
		{
			Name:        "resource_maximum_conflict",
			Description: "Resolve resource requirement conflicts by using maximum",
			Pattern: ConflictPattern{
				Type:     ConflictTypeResource,
				Paths:    []string{"resources.cpu", "resources.memory"},
				Keywords: []string{"resources", "cpu", "memory"},
			},
			Resolution: ConflictResolution{
				Strategy: StrategyMerge,
				Priority: "required",
				Action:   "use_maximum",
				Message:  "Using maximum resource requirement",
			},
			Severity: ConflictSeverityMinor,
		},
		{
			Name:        "exclusive_vendor_conflict",
			Description: "Resolve mutually exclusive vendor requirements",
			Pattern: ConflictPattern{
				Type:     ConflictTypeExclusive,
				Paths:    []string{"vendor.aws", "vendor.azure", "vendor.gcp"},
				Keywords: []string{"vendor", "cloud", "provider"},
			},
			Resolution: ConflictResolution{
				Strategy: StrategyHighestPriority,
				Priority: "required",
				Action:   "choose_one",
				Message:  "Choosing highest priority vendor",
			},
			Severity: ConflictSeverityMajor,
		},
	}
}
