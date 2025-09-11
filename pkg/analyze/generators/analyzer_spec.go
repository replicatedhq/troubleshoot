package generators

import (
	"fmt"
	"strings"
)

// AnalyzerSpec defines the specification for generating an analyzer
type AnalyzerSpec struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description"`
	Category     RequirementCategory      `json:"category"`
	Type         AnalyzerType             `json:"type"`
	Requirements []CategorizedRequirement `json:"requirements"`
	Tags         []string                 `json:"tags"`
	Priority     RequirementPriority      `json:"priority"`
	Rules        []GenerationRule         `json:"rules"`
	Template     string                   `json:"template"`
	Variables    map[string]interface{}   `json:"variables"`
	Tests        []TestSpec               `json:"tests"`
}

// GenerationRule defines rules for analyzer generation
type GenerationRule struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Condition   string                 `json:"condition"`
	Action      GenerationAction       `json:"action"`
	Parameters  map[string]interface{} `json:"parameters"`
	Priority    int                    `json:"priority"`
}

// GenerationAction defines the type of action to take during generation
type GenerationAction string

const (
	ActionIncludeField  GenerationAction = "include_field"
	ActionExcludeField  GenerationAction = "exclude_field"
	ActionSetVariable   GenerationAction = "set_variable"
	ActionAddImport     GenerationAction = "add_import"
	ActionAddMethod     GenerationAction = "add_method"
	ActionAddValidation GenerationAction = "add_validation"
	ActionSetTemplate   GenerationAction = "set_template"
	ActionAddTest       GenerationAction = "add_test"
)

// TestSpec defines test specifications for generated analyzers
type TestSpec struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Input       map[string]interface{} `json:"input"`
	Expected    TestExpectation        `json:"expected"`
	Setup       []string               `json:"setup"`
	Teardown    []string               `json:"teardown"`
}

// TestExpectation defines what to expect from a test
type TestExpectation struct {
	Result     string                 `json:"result"` // "pass", "fail", "warn"
	Message    string                 `json:"message"`
	Properties map[string]interface{} `json:"properties"`
	ErrorCount int                    `json:"error_count"`
	WarnCount  int                    `json:"warn_count"`
}

// RuleEngine applies rules to determine analyzer generation strategy
type RuleEngine struct {
	rules []GenerationRule
}

// NewRuleEngine creates a new rule engine
func NewRuleEngine() *RuleEngine {
	return &RuleEngine{
		rules: getDefaultGenerationRules(),
	}
}

// ApplyRules applies generation rules to create analyzer specifications
func (r *RuleEngine) ApplyRules(category RequirementCategory, requirements []CategorizedRequirement) []AnalyzerSpec {
	var specs []AnalyzerSpec

	// Group requirements by logical analyzer
	analyzerGroups := r.groupRequirementsForAnalyzers(category, requirements)

	// Create analyzer spec for each group
	for _, group := range analyzerGroups {
		spec := r.createAnalyzerSpec(category, group)
		specs = append(specs, spec)
	}

	return specs
}

// AddRule adds a custom generation rule
func (r *RuleEngine) AddRule(rule GenerationRule) {
	r.rules = append(r.rules, rule)
}

// groupRequirementsForAnalyzers groups requirements that should be in the same analyzer
func (r *RuleEngine) groupRequirementsForAnalyzers(category RequirementCategory, requirements []CategorizedRequirement) [][]CategorizedRequirement {
	var groups [][]CategorizedRequirement

	switch category {
	case CategoryKubernetes:
		groups = r.groupKubernetesRequirements(requirements)
	case CategoryResources:
		groups = r.groupResourceRequirements(requirements)
	case CategoryStorage:
		groups = r.groupStorageRequirements(requirements)
	case CategoryNetwork:
		groups = r.groupNetworkRequirements(requirements)
	case CategorySecurity:
		groups = r.groupSecurityRequirements(requirements)
	default:
		// For other categories, create one analyzer per requirement
		for _, req := range requirements {
			groups = append(groups, []CategorizedRequirement{req})
		}
	}

	return groups
}

// groupKubernetesRequirements groups Kubernetes requirements logically
func (r *RuleEngine) groupKubernetesRequirements(requirements []CategorizedRequirement) [][]CategorizedRequirement {
	var groups [][]CategorizedRequirement

	versionGroup := []CategorizedRequirement{}
	apiGroup := []CategorizedRequirement{}
	nodeGroup := []CategorizedRequirement{}

	for _, req := range requirements {
		if containsAny(req.Path, []string{"version", "Version"}) {
			versionGroup = append(versionGroup, req)
		} else if containsAny(req.Path, []string{"api", "API", "apis"}) {
			apiGroup = append(apiGroup, req)
		} else if containsAny(req.Path, []string{"node", "Node", "nodes"}) {
			nodeGroup = append(nodeGroup, req)
		}
	}

	if len(versionGroup) > 0 {
		groups = append(groups, versionGroup)
	}
	if len(apiGroup) > 0 {
		groups = append(groups, apiGroup)
	}
	if len(nodeGroup) > 0 {
		groups = append(groups, nodeGroup)
	}

	return groups
}

// groupResourceRequirements groups resource requirements logically
func (r *RuleEngine) groupResourceRequirements(requirements []CategorizedRequirement) [][]CategorizedRequirement {
	var groups [][]CategorizedRequirement

	cpuGroup := []CategorizedRequirement{}
	memoryGroup := []CategorizedRequirement{}
	nodeGroup := []CategorizedRequirement{}

	for _, req := range requirements {
		if containsAny(req.Path, []string{"cpu", "CPU", "cores"}) {
			cpuGroup = append(cpuGroup, req)
		} else if containsAny(req.Path, []string{"memory", "Memory", "ram", "bytes"}) {
			memoryGroup = append(memoryGroup, req)
		} else if containsAny(req.Path, []string{"node", "Node", "nodes"}) {
			nodeGroup = append(nodeGroup, req)
		}
	}

	if len(cpuGroup) > 0 {
		groups = append(groups, cpuGroup)
	}
	if len(memoryGroup) > 0 {
		groups = append(groups, memoryGroup)
	}
	if len(nodeGroup) > 0 {
		groups = append(groups, nodeGroup)
	}

	return groups
}

// groupStorageRequirements groups storage requirements logically
func (r *RuleEngine) groupStorageRequirements(requirements []CategorizedRequirement) [][]CategorizedRequirement {
	var groups [][]CategorizedRequirement

	capacityGroup := []CategorizedRequirement{}
	classGroup := []CategorizedRequirement{}
	performanceGroup := []CategorizedRequirement{}

	for _, req := range requirements {
		if containsAny(req.Path, []string{"capacity", "Capacity", "size"}) {
			capacityGroup = append(capacityGroup, req)
		} else if containsAny(req.Path, []string{"class", "Class", "storageClass"}) {
			classGroup = append(classGroup, req)
		} else if containsAny(req.Path, []string{"performance", "Performance", "iops", "throughput"}) {
			performanceGroup = append(performanceGroup, req)
		}
	}

	if len(capacityGroup) > 0 {
		groups = append(groups, capacityGroup)
	}
	if len(classGroup) > 0 {
		groups = append(groups, classGroup)
	}
	if len(performanceGroup) > 0 {
		groups = append(groups, performanceGroup)
	}

	return groups
}

// groupNetworkRequirements groups network requirements logically
func (r *RuleEngine) groupNetworkRequirements(requirements []CategorizedRequirement) [][]CategorizedRequirement {
	var groups [][]CategorizedRequirement

	bandwidthGroup := []CategorizedRequirement{}
	connectivityGroup := []CategorizedRequirement{}
	securityGroup := []CategorizedRequirement{}

	for _, req := range requirements {
		if containsAny(req.Path, []string{"bandwidth", "Bandwidth"}) {
			bandwidthGroup = append(bandwidthGroup, req)
		} else if containsAny(req.Path, []string{"connectivity", "Connectivity", "endpoint"}) {
			connectivityGroup = append(connectivityGroup, req)
		} else if containsAny(req.Path, []string{"security", "Security", "tls", "ssl"}) {
			securityGroup = append(securityGroup, req)
		}
	}

	if len(bandwidthGroup) > 0 {
		groups = append(groups, bandwidthGroup)
	}
	if len(connectivityGroup) > 0 {
		groups = append(groups, connectivityGroup)
	}
	if len(securityGroup) > 0 {
		groups = append(groups, securityGroup)
	}

	return groups
}

// groupSecurityRequirements groups security requirements logically
func (r *RuleEngine) groupSecurityRequirements(requirements []CategorizedRequirement) [][]CategorizedRequirement {
	var groups [][]CategorizedRequirement

	rbacGroup := []CategorizedRequirement{}
	podSecurityGroup := []CategorizedRequirement{}
	encryptionGroup := []CategorizedRequirement{}

	for _, req := range requirements {
		if containsAny(req.Path, []string{"rbac", "RBAC", "role", "Role"}) {
			rbacGroup = append(rbacGroup, req)
		} else if containsAny(req.Path, []string{"pod", "Pod", "podSecurity"}) {
			podSecurityGroup = append(podSecurityGroup, req)
		} else if containsAny(req.Path, []string{"encryption", "Encryption", "tls", "ssl"}) {
			encryptionGroup = append(encryptionGroup, req)
		}
	}

	if len(rbacGroup) > 0 {
		groups = append(groups, rbacGroup)
	}
	if len(podSecurityGroup) > 0 {
		groups = append(groups, podSecurityGroup)
	}
	if len(encryptionGroup) > 0 {
		groups = append(groups, encryptionGroup)
	}

	return groups
}

// createAnalyzerSpec creates an analyzer specification from a group of requirements
func (r *RuleEngine) createAnalyzerSpec(category RequirementCategory, requirements []CategorizedRequirement) AnalyzerSpec {
	// Determine analyzer name based on requirements
	name := r.generateAnalyzerName(category, requirements)

	// Determine analyzer description
	description := r.generateAnalyzerDescription(category, requirements)

	// Determine priority (highest priority in the group)
	priority := r.determineHighestPriority(requirements)

	// Generate tags
	tags := r.generateTags(category, requirements)

	// Create test specifications
	tests := r.generateTestSpecs(category, requirements)

	return AnalyzerSpec{
		Name:         name,
		Description:  description,
		Category:     category,
		Type:         AnalyzerType(category),
		Requirements: requirements,
		Priority:     priority,
		Tags:         tags,
		Rules:        r.getApplicableRules(category),
		Variables:    r.generateVariables(requirements),
		Tests:        tests,
	}
}

// generateAnalyzerName generates a name for the analyzer
func (r *RuleEngine) generateAnalyzerName(category RequirementCategory, requirements []CategorizedRequirement) string {
	base := string(category)

	// Try to be more specific based on requirements
	if len(requirements) > 0 {
		first := requirements[0]
		parts := strings.Split(first.Path, ".")
		if len(parts) > 1 {
			return fmt.Sprintf("%s-%s", base, parts[1])
		}
	}

	return fmt.Sprintf("%s-analyzer", base)
}

// generateAnalyzerDescription generates a description for the analyzer
func (r *RuleEngine) generateAnalyzerDescription(category RequirementCategory, requirements []CategorizedRequirement) string {
	baseDescription := fmt.Sprintf("Validates %s requirements", category)

	if len(requirements) == 1 {
		return fmt.Sprintf("%s for %s", baseDescription, requirements[0].Path)
	} else if len(requirements) > 1 {
		return fmt.Sprintf("%s including %s and %d other requirements",
			baseDescription, requirements[0].Path, len(requirements)-1)
	}

	return baseDescription
}

// determineHighestPriority determines the highest priority in a group
func (r *RuleEngine) determineHighestPriority(requirements []CategorizedRequirement) RequirementPriority {
	highest := PriorityOptional

	for _, req := range requirements {
		if req.Priority == PriorityRequired {
			return PriorityRequired
		} else if req.Priority == PriorityRecommended && highest == PriorityOptional {
			highest = PriorityRecommended
		}
	}

	return highest
}

// generateTags generates tags for the analyzer
func (r *RuleEngine) generateTags(category RequirementCategory, requirements []CategorizedRequirement) []string {
	tags := []string{string(category)}

	// Add tags based on requirement keywords
	for _, req := range requirements {
		tags = append(tags, req.Tags...)
	}

	// Remove duplicates
	return removeDuplicateStrings(tags)
}

// generateTestSpecs generates test specifications
func (r *RuleEngine) generateTestSpecs(category RequirementCategory, requirements []CategorizedRequirement) []TestSpec {
	var tests []TestSpec

	for _, req := range requirements {
		// Generate basic pass/fail tests for each requirement
		tests = append(tests, TestSpec{
			Name:        fmt.Sprintf("Test_%s_Pass", strings.ReplaceAll(req.Path, ".", "_")),
			Description: fmt.Sprintf("Test that %s requirement passes", req.Path),
			Expected: TestExpectation{
				Result:  "pass",
				Message: fmt.Sprintf("%s requirement is satisfied", req.Path),
			},
		})

		tests = append(tests, TestSpec{
			Name:        fmt.Sprintf("Test_%s_Fail", strings.ReplaceAll(req.Path, ".", "_")),
			Description: fmt.Sprintf("Test that %s requirement fails", req.Path),
			Expected: TestExpectation{
				Result:  "fail",
				Message: fmt.Sprintf("%s requirement is not satisfied", req.Path),
			},
		})
	}

	return tests
}

// generateVariables generates template variables from requirements
func (r *RuleEngine) generateVariables(requirements []CategorizedRequirement) map[string]interface{} {
	vars := make(map[string]interface{})

	for _, req := range requirements {
		// Convert requirement to template variables
		key := strings.ReplaceAll(req.Path, ".", "_")
		vars[key] = req.Value
		vars[key+"_Priority"] = string(req.Priority)
		vars[key+"_Tags"] = req.Tags
	}

	return vars
}

// getApplicableRules gets rules applicable to a category
func (r *RuleEngine) getApplicableRules(category RequirementCategory) []GenerationRule {
	var applicable []GenerationRule

	for _, rule := range r.rules {
		// Simple matching - in practice would be more sophisticated
		if strings.Contains(rule.Condition, string(category)) {
			applicable = append(applicable, rule)
		}
	}

	return applicable
}

// Helper functions

func containsAny(str string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(str, sub) {
			return true
		}
	}
	return false
}

func removeDuplicateStrings(slice []string) []string {
	seen := make(map[string]bool)
	result := []string{}

	for _, item := range slice {
		if !seen[item] {
			seen[item] = true
			result = append(result, item)
		}
	}

	return result
}

// getDefaultGenerationRules returns default generation rules
func getDefaultGenerationRules() []GenerationRule {
	return []GenerationRule{
		{
			Name:        "kubernetes_version_check",
			Description: "Add version checking for Kubernetes requirements",
			Condition:   "category == 'kubernetes' && hasVersionRequirement",
			Action:      ActionAddMethod,
			Parameters: map[string]interface{}{
				"method_name": "checkKubernetesVersion",
				"template":    "kubernetes_version_check_template",
			},
			Priority: 10,
		},
		{
			Name:        "resource_validation",
			Description: "Add resource validation for resource requirements",
			Condition:   "category == 'resources'",
			Action:      ActionAddValidation,
			Parameters: map[string]interface{}{
				"validation_type": "resource",
				"template":        "resource_validation_template",
			},
			Priority: 8,
		},
		{
			Name:        "storage_class_check",
			Description: "Add storage class checking for storage requirements",
			Condition:   "category == 'storage'",
			Action:      ActionAddMethod,
			Parameters: map[string]interface{}{
				"method_name": "checkStorageClass",
				"template":    "storage_class_check_template",
			},
			Priority: 7,
		},
	}
}
