package generators

import (
	"fmt"
	"regexp"
	"strings"
)

// RequirementValidator validates requirement specifications
type RequirementValidator struct {
	rules []ValidationRule
}

// ValidationRule defines a validation rule
type ValidationRule struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Path        string            `json:"path"`
	Required    bool              `json:"required"`
	Pattern     string            `json:"pattern,omitempty"`
	MinValue    interface{}       `json:"min_value,omitempty"`
	MaxValue    interface{}       `json:"max_value,omitempty"`
	AllowedValues []interface{}   `json:"allowed_values,omitempty"`
	Custom      ValidationFunc    `json:"-"`
}

// ValidationFunc is a custom validation function
type ValidationFunc func(value interface{}, spec *RequirementSpec) error

// ValidationError represents a validation error
type ValidationError struct {
	Path        string `json:"path"`
	Field       string `json:"field"`
	Value       interface{} `json:"value"`
	Rule        string `json:"rule"`
	Message     string `json:"message"`
	Severity    ValidationSeverity `json:"severity"`
}

// ValidationWarning represents a validation warning
type ValidationWarning struct {
	Path        string `json:"path"`
	Field       string `json:"field"`
	Value       interface{} `json:"value"`
	Rule        string `json:"rule"`
	Message     string `json:"message"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// ValidationSeverity defines the severity of validation issues
type ValidationSeverity string

const (
	SeverityError   ValidationSeverity = "error"
	SeverityWarning ValidationSeverity = "warning"
	SeverityInfo    ValidationSeverity = "info"
)

// NewRequirementValidator creates a new requirement validator
func NewRequirementValidator() *RequirementValidator {
	return &RequirementValidator{
		rules: getDefaultValidationRules(),
	}
}

// Validate validates a requirement specification (legacy method)
func (v *RequirementValidator) Validate(spec *RequirementSpec) error {
	errors, _, err := v.ValidateSpec(spec)
	if err != nil {
		return err
	}

	if len(errors) > 0 {
		return fmt.Errorf("validation failed with %d errors", len(errors))
	}

	return nil
}

// ValidateSpec validates a requirement specification and returns detailed results
func (v *RequirementValidator) ValidateSpec(spec *RequirementSpec) ([]ValidationError, []ValidationWarning, error) {
	var errors []ValidationError
	var warnings []ValidationWarning

	// Validate basic structure
	if err := v.validateBasicStructure(spec); err != nil {
		errors = append(errors, ValidationError{
			Path:     "root",
			Field:    "structure",
			Rule:     "basic_structure",
			Message:  err.Error(),
			Severity: SeverityError,
		})
	}

	// Validate metadata
	metadataErrors, metadataWarnings := v.validateMetadata(&spec.Metadata)
	errors = append(errors, metadataErrors...)
	warnings = append(warnings, metadataWarnings...)

	// Validate spec details
	specErrors, specWarnings := v.validateSpecDetails(&spec.Spec)
	errors = append(errors, specErrors...)
	warnings = append(warnings, specWarnings...)

	return errors, warnings, nil
}

// AddValidationRule adds a custom validation rule
func (v *RequirementValidator) AddValidationRule(rule ValidationRule) {
	v.rules = append(v.rules, rule)
}

// validateBasicStructure validates the basic structure of the specification
func (v *RequirementValidator) validateBasicStructure(spec *RequirementSpec) error {
	if spec == nil {
		return fmt.Errorf("specification is nil")
	}

	if spec.APIVersion == "" {
		return fmt.Errorf("apiVersion is required")
	}

	if spec.Kind == "" {
		return fmt.Errorf("kind is required")
	}

	// Validate API version format
	if !v.isValidAPIVersion(spec.APIVersion) {
		return fmt.Errorf("invalid apiVersion format: %s", spec.APIVersion)
	}

	// Validate Kind
	if spec.Kind != "RequirementSpec" {
		return fmt.Errorf("invalid kind: %s, expected RequirementSpec", spec.Kind)
	}

	return nil
}

func (v *RequirementValidator) isValidAPIVersion(version string) bool {
	pattern := `^[a-zA-Z][a-zA-Z0-9.-]*\/v\d+(?:alpha\d*|beta\d*)?$`
	matched, _ := regexp.MatchString(pattern, version)
	return matched
}

func (v *RequirementValidator) isValidName(name string) bool {
	pattern := `^[a-z0-9-]+$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched && !strings.HasPrefix(name, "-") && !strings.HasSuffix(name, "-")
}

func (v *RequirementValidator) isValidSemVer(version string) bool {
	pattern := `^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-((?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(?:\.(?:0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*))?(?:\+([0-9a-zA-Z-]+(?:\.[0-9a-zA-Z-]+)*))?$`
	matched, _ := regexp.MatchString(pattern, version)
	return matched
}

func (v *RequirementValidator) isValidTag(tag string) bool {
	pattern := `^[a-zA-Z0-9-_]+$`
	matched, _ := regexp.MatchString(pattern, tag)
	return matched
}

func (v *RequirementValidator) isValidKubernetesVersion(version string) bool {
	pattern := `^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-.*)?$`
	matched, _ := regexp.MatchString(pattern, version)
	return matched
}

func (v *RequirementValidator) isVersionRangeValid(minVersion, maxVersion string) bool {
	return strings.Compare(minVersion, maxVersion) <= 0
}

func (v *RequirementValidator) isValidIPOrHostname(addr string) bool {
	ipPattern := `^(\d{1,3}\.){3}\d{1,3}$`
	hostnamePattern := `^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`
	
	ipMatched, _ := regexp.MatchString(ipPattern, addr)
	hostnameMatched, _ := regexp.MatchString(hostnamePattern, addr)
	
	return ipMatched || hostnameMatched
}

func (v *RequirementValidator) isValidPodSecurityStandard(standard string) bool {
	validStandards := []string{"privileged", "baseline", "restricted"}
	for _, valid := range validStandards {
		if standard == valid {
			return true
		}
	}
	return false
}

func (v *RequirementValidator) isValidTLSVersion(version string) bool {
	validVersions := []string{"1.0", "1.1", "1.2", "1.3"}
	for _, valid := range validVersions {
		if version == valid {
			return true
		}
	}
	return false
}

func (v *RequirementValidator) isValidRegistryEndpoint(endpoint string) bool {
	pattern := `^[a-zA-Z0-9.-]+(?::\d+)?(?:\/.*)?$`
	matched, _ := regexp.MatchString(pattern, endpoint)
	return matched
}

func (v *RequirementValidator) validateMetadata(metadata *RequirementMetadata) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	if metadata.Name == "" {
		errors = append(errors, ValidationError{
			Path:     "metadata.name",
			Field:    "name",
			Rule:     "required",
			Message:  "metadata.name is required",
			Severity: SeverityError,
		})
	} else if !v.isValidName(metadata.Name) {
		errors = append(errors, ValidationError{
			Path:     "metadata.name",
			Field:    "name",
			Value:    metadata.Name,
			Rule:     "format",
			Message:  "metadata.name must contain only lowercase letters, numbers, and hyphens",
			Severity: SeverityError,
		})
	}

	if metadata.Version != "" && !v.isValidSemVer(metadata.Version) {
		warnings = append(warnings, ValidationWarning{
			Path:       "metadata.version",
			Field:      "version",
			Value:      metadata.Version,
			Rule:       "semver",
			Message:    "metadata.version should follow semantic versioning format",
			Suggestion: "Use format like '1.0.0' or '2.1.3-beta.1'",
		})
	}

	for i, tag := range metadata.Tags {
		if !v.isValidTag(tag) {
			errors = append(errors, ValidationError{
				Path:     fmt.Sprintf("metadata.tags[%d]", i),
				Field:    "tag",
				Value:    tag,
				Rule:     "format",
				Message:  "tag contains invalid characters",
				Severity: SeverityError,
			})
		}
	}

	return errors, warnings
}

func (v *RequirementValidator) validateSpecDetails(spec *RequirementSpecDetails) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	k8sErrors, k8sWarnings := v.validateKubernetesRequirements(&spec.Kubernetes)
	errors = append(errors, k8sErrors...)
	warnings = append(warnings, k8sWarnings...)

	resourceErrors, resourceWarnings := v.validateResourceRequirements(&spec.Resources)
	errors = append(errors, resourceErrors...)
	warnings = append(warnings, resourceWarnings...)

	storageErrors, storageWarnings := v.validateStorageRequirements(&spec.Storage)
	errors = append(errors, storageErrors...)
	warnings = append(warnings, storageWarnings...)

	return errors, warnings
}

func (v *RequirementValidator) validateKubernetesRequirements(k8s *KubernetesRequirements) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	if k8s.MinVersion != "" && !v.isValidKubernetesVersion(k8s.MinVersion) {
		errors = append(errors, ValidationError{
			Path:     "kubernetes.minVersion",
			Field:    "minVersion",
			Value:    k8s.MinVersion,
			Rule:     "version_format",
			Message:  "invalid Kubernetes version format",
			Severity: SeverityError,
		})
	}

	if k8s.MaxVersion != "" && !v.isValidKubernetesVersion(k8s.MaxVersion) {
		errors = append(errors, ValidationError{
			Path:     "kubernetes.maxVersion",
			Field:    "maxVersion",
			Value:    k8s.MaxVersion,
			Rule:     "version_format",
			Message:  "invalid Kubernetes version format",
			Severity: SeverityError,
		})
	}

	if k8s.MinVersion != "" && k8s.MaxVersion != "" && !v.isVersionRangeValid(k8s.MinVersion, k8s.MaxVersion) {
		errors = append(errors, ValidationError{
			Path:     "kubernetes.version",
			Field:    "version_range",
			Value:    fmt.Sprintf("%s-%s", k8s.MinVersion, k8s.MaxVersion),
			Rule:     "version_range",
			Message:  "minVersion must be less than or equal to maxVersion",
			Severity: SeverityError,
		})
	}

	if k8s.NodeCount.Min < 0 {
		errors = append(errors, ValidationError{
			Path:     "kubernetes.nodeCount.min",
			Field:    "min",
			Value:    k8s.NodeCount.Min,
			Rule:     "min_value",
			Message:  "minimum node count cannot be negative",
			Severity: SeverityError,
		})
	}

	return errors, warnings
}

func (v *RequirementValidator) validateResourceRequirements(resources *ResourceRequirements) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	if resources.CPU.MinCores < 0 {
		errors = append(errors, ValidationError{
			Path:     "resources.cpu.minCores",
			Field:    "minCores",
			Value:    resources.CPU.MinCores,
			Rule:     "min_value",
			Message:  "minimum CPU cores cannot be negative",
			Severity: SeverityError,
		})
	}

	if resources.Memory.MinBytes < 0 {
		errors = append(errors, ValidationError{
			Path:     "resources.memory.minBytes",
			Field:    "minBytes",
			Value:    resources.Memory.MinBytes,
			Rule:     "min_value",
			Message:  "minimum memory cannot be negative",
			Severity: SeverityError,
		})
	}

	return errors, warnings
}

func (v *RequirementValidator) validateStorageRequirements(storage *StorageRequirements) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning

	if storage.MinCapacity < 0 {
		errors = append(errors, ValidationError{
			Path:     "storage.minCapacity",
			Field:    "minCapacity",
			Value:    storage.MinCapacity,
			Rule:     "min_value",
			Message:  "minimum storage capacity cannot be negative",
			Severity: SeverityError,
		})
	}

	return errors, warnings
}

func (v *RequirementValidator) validateNetworkRequirements(network *NetworkRequirements) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning
	// Basic validation - more detailed validation would be added here
	return errors, warnings
}

func (v *RequirementValidator) validateSecurityRequirements(security *SecurityRequirements) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning
	// Basic validation - more detailed validation would be added here
	return errors, warnings
}

func (v *RequirementValidator) validateCustomRequirements(customs []CustomRequirement) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning
	
	for i, custom := range customs {
		if custom.Name == "" {
			errors = append(errors, ValidationError{
				Path:     fmt.Sprintf("custom[%d].name", i),
				Field:    "name",
				Rule:     "required",
				Message:  "custom requirement name is required",
				Severity: SeverityError,
			})
		}
	}
	
	return errors, warnings
}

func (v *RequirementValidator) validateVendorRequirements(vendor *VendorRequirements) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning
	// Basic validation - more detailed validation would be added here
	return errors, warnings
}

func (v *RequirementValidator) validateReplicatedRequirements(replicated *ReplicatedRequirements) ([]ValidationError, []ValidationWarning) {
	var errors []ValidationError
	var warnings []ValidationWarning
	// Basic validation - more detailed validation would be added here
	return errors, warnings
}

func getDefaultValidationRules() []ValidationRule {
	return []ValidationRule{
		{
			Name:        "api_version_required",
			Description: "API version is required",
			Path:        "apiVersion",
			Required:    true,
		},
		{
			Name:        "kind_required",
			Description: "Kind is required",
			Path:        "kind",
			Required:    true,
		},
		{
			Name:        "metadata_name_required",
			Description: "Metadata name is required",
			Path:        "metadata.name",
			Required:    true,
		},
	}
}
