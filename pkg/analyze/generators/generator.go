package generators

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	analyzer "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"k8s.io/klog/v2"
)

// AnalyzerGenerator generates analyzer specifications from requirements
type AnalyzerGenerator struct {
	templates  map[string]AnalyzerTemplate
	validators map[string]RequirementValidator
}

// AnalyzerTemplate defines how to generate analyzers for specific requirement types
type AnalyzerTemplate struct {
	Name        string
	Description string
	Category    string
	Priority    int
	Generator   func(ctx context.Context, req interface{}) ([]analyzer.AnalyzerSpec, error)
	Validator   func(req interface{}) error
}

// RequirementValidator validates requirement specifications
type RequirementValidator func(requirement interface{}) error

// GenerationOptions configures analyzer generation
type GenerationOptions struct {
	IncludeOptional bool
	Strict          bool
	DefaultPriority int
	CategoryFilter  []string
	CustomTemplates map[string]AnalyzerTemplate
}

// NewAnalyzerGenerator creates a new analyzer generator with default templates
func NewAnalyzerGenerator() *AnalyzerGenerator {
	g := &AnalyzerGenerator{
		templates:  make(map[string]AnalyzerTemplate),
		validators: make(map[string]RequirementValidator),
	}

	// Register default templates
	g.registerDefaultTemplates()
	g.registerDefaultValidators()

	return g
}

// GenerateAnalyzers creates analyzer specifications from requirements
func (g *AnalyzerGenerator) GenerateAnalyzers(ctx context.Context, requirements *analyzer.RequirementSpec, opts *GenerationOptions) ([]analyzer.AnalyzerSpec, error) {
	ctx, span := otel.Tracer(constants.LIB_TRACER_NAME).Start(ctx, "AnalyzerGenerator.GenerateAnalyzers")
	defer span.End()

	if requirements == nil {
		return nil, errors.New("requirements cannot be nil")
	}

	if opts == nil {
		opts = &GenerationOptions{
			IncludeOptional: true,
			DefaultPriority: 5,
		}
	}

	var allSpecs []analyzer.AnalyzerSpec

	// Generate Kubernetes version analyzers
	if specs, err := g.generateKubernetesAnalyzers(ctx, &requirements.Spec.Kubernetes, opts); err == nil {
		allSpecs = append(allSpecs, specs...)
	} else {
		klog.Warningf("Failed to generate Kubernetes analyzers: %v", err)
	}

	// Generate resource requirement analyzers
	if specs, err := g.generateResourceAnalyzers(ctx, &requirements.Spec.Resources, opts); err == nil {
		allSpecs = append(allSpecs, specs...)
	} else {
		klog.Warningf("Failed to generate resource analyzers: %v", err)
	}

	// Generate storage requirement analyzers
	if specs, err := g.generateStorageAnalyzers(ctx, &requirements.Spec.Storage, opts); err == nil {
		allSpecs = append(allSpecs, specs...)
	} else {
		klog.Warningf("Failed to generate storage analyzers: %v", err)
	}

	// Generate network requirement analyzers
	if specs, err := g.generateNetworkAnalyzers(ctx, &requirements.Spec.Network, opts); err == nil {
		allSpecs = append(allSpecs, specs...)
	} else {
		klog.Warningf("Failed to generate network analyzers: %v", err)
	}

	// Generate custom analyzers
	for _, customReq := range requirements.Spec.Custom {
		if specs, err := g.generateCustomAnalyzers(ctx, &customReq, opts); err == nil {
			allSpecs = append(allSpecs, specs...)
		} else {
			klog.Warningf("Failed to generate custom analyzer %s: %v", customReq.Name, err)
		}
	}

	// Apply category filtering if specified
	if len(opts.CategoryFilter) > 0 {
		allSpecs = g.filterByCategory(allSpecs, opts.CategoryFilter)
	}

	// Sort by priority (higher priority first)
	g.sortByPriority(allSpecs)

	span.SetAttributes(
		attribute.Int("total_generated", len(allSpecs)),
		attribute.String("requirements_name", requirements.Metadata.Name),
		attribute.Bool("include_optional", opts.IncludeOptional),
	)

	return allSpecs, nil
}

// generateKubernetesAnalyzers creates analyzers for Kubernetes requirements
func (g *AnalyzerGenerator) generateKubernetesAnalyzers(ctx context.Context, req *analyzer.KubernetesRequirements, opts *GenerationOptions) ([]analyzer.AnalyzerSpec, error) {
	var specs []analyzer.AnalyzerSpec

	// Kubernetes version check analyzer
	if req.MinVersion != "" || req.MaxVersion != "" {
		spec := analyzer.AnalyzerSpec{
			Name:     "kubernetes-version-requirement",
			Type:     "cluster",
			Category: "kubernetes",
			Priority: 10,
			Config: map[string]interface{}{
				"checkName":  "Kubernetes Version Check",
				"minVersion": req.MinVersion,
				"maxVersion": req.MaxVersion,
				"outcomes":   g.generateVersionOutcomes(req.MinVersion, req.MaxVersion),
			},
		}
		specs = append(specs, spec)
	}

	// Required components analyzer
	if len(req.Required) > 0 {
		spec := analyzer.AnalyzerSpec{
			Name:     "kubernetes-components-required",
			Type:     "cluster",
			Category: "kubernetes",
			Priority: 9,
			Config: map[string]interface{}{
				"checkName": "Required Components Check",
				"required":  req.Required,
				"outcomes":  g.generateComponentOutcomes(req.Required, true),
			},
		}
		specs = append(specs, spec)
	}

	// Forbidden components analyzer
	if len(req.Forbidden) > 0 {
		spec := analyzer.AnalyzerSpec{
			Name:     "kubernetes-components-forbidden",
			Type:     "cluster",
			Category: "kubernetes",
			Priority: 8,
			Config: map[string]interface{}{
				"checkName": "Forbidden Components Check",
				"forbidden": req.Forbidden,
				"outcomes":  g.generateComponentOutcomes(req.Forbidden, false),
			},
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// generateResourceAnalyzers creates analyzers for resource requirements
func (g *AnalyzerGenerator) generateResourceAnalyzers(ctx context.Context, req *analyzer.ResourceRequirements, opts *GenerationOptions) ([]analyzer.AnalyzerSpec, error) {
	var specs []analyzer.AnalyzerSpec

	// Node resources analyzer
	if req.CPU.Min != "" || req.Memory.Min != "" || req.Disk.Min != "" {
		spec := analyzer.AnalyzerSpec{
			Name:     "node-resources-requirement",
			Type:     "resources",
			Category: "capacity",
			Priority: 9,
			Config: map[string]interface{}{
				"checkName": "Node Resources Check",
				"cpu":       req.CPU,
				"memory":    req.Memory,
				"disk":      req.Disk,
				"outcomes":  g.generateResourceOutcomes(req),
			},
		}
		specs = append(specs, spec)
	}

	// Cluster capacity analyzer
	if req.CPU.Min != "" || req.Memory.Min != "" {
		spec := analyzer.AnalyzerSpec{
			Name:     "cluster-capacity-requirement",
			Type:     "resources",
			Category: "capacity",
			Priority: 8,
			Config: map[string]interface{}{
				"checkName":    "Cluster Capacity Check",
				"requirements": req,
				"outcomes":     g.generateClusterCapacityOutcomes(req),
			},
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// generateStorageAnalyzers creates analyzers for storage requirements
func (g *AnalyzerGenerator) generateStorageAnalyzers(ctx context.Context, req *analyzer.StorageRequirements, opts *GenerationOptions) ([]analyzer.AnalyzerSpec, error) {
	var specs []analyzer.AnalyzerSpec

	// Storage class analyzer
	if len(req.Classes) > 0 {
		spec := analyzer.AnalyzerSpec{
			Name:     "storage-class-requirement",
			Type:     "storage",
			Category: "storage",
			Priority: 8,
			Config: map[string]interface{}{
				"checkName":    "Storage Class Check",
				"storageClass": req.Classes[0], // Use first class as primary
				"outcomes":     g.generateStorageClassOutcomes(req.Classes),
			},
		}
		specs = append(specs, spec)
	}

	// Persistent volume analyzer
	if req.MinCapacity != "" {
		spec := analyzer.AnalyzerSpec{
			Name:     "persistent-volume-requirement",
			Type:     "storage",
			Category: "storage",
			Priority: 7,
			Config: map[string]interface{}{
				"checkName":   "Persistent Volume Capacity Check",
				"minCapacity": req.MinCapacity,
				"accessModes": req.AccessModes,
				"outcomes":    g.generatePVOutcomes(req),
			},
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// generateNetworkAnalyzers creates analyzers for network requirements
func (g *AnalyzerGenerator) generateNetworkAnalyzers(ctx context.Context, req *analyzer.NetworkRequirements, opts *GenerationOptions) ([]analyzer.AnalyzerSpec, error) {
	var specs []analyzer.AnalyzerSpec

	// Port connectivity analyzer
	for _, port := range req.Ports {
		spec := analyzer.AnalyzerSpec{
			Name:     fmt.Sprintf("port-connectivity-%d", port.Port),
			Type:     "network",
			Category: "networking",
			Priority: 7,
			Config: map[string]interface{}{
				"checkName": fmt.Sprintf("Port %d Connectivity Check", port.Port),
				"port":      port.Port,
				"protocol":  port.Protocol,
				"required":  port.Required,
				"outcomes":  g.generatePortOutcomes(port),
			},
		}
		specs = append(specs, spec)
	}

	// General connectivity analyzer
	if len(req.Connectivity) > 0 {
		spec := analyzer.AnalyzerSpec{
			Name:     "network-connectivity-requirement",
			Type:     "network",
			Category: "networking",
			Priority: 6,
			Config: map[string]interface{}{
				"checkName":    "Network Connectivity Check",
				"connectivity": req.Connectivity,
				"outcomes":     g.generateConnectivityOutcomes(req.Connectivity),
			},
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// generateCustomAnalyzers creates analyzers for custom requirements
func (g *AnalyzerGenerator) generateCustomAnalyzers(ctx context.Context, req *analyzer.CustomRequirement, opts *GenerationOptions) ([]analyzer.AnalyzerSpec, error) {
	var specs []analyzer.AnalyzerSpec

	// Check if we have a template for this custom type
	template, exists := g.templates[req.Type]
	if exists {
		customSpecs, err := template.Generator(ctx, req)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate custom analyzer %s", req.Name)
		}
		specs = append(specs, customSpecs...)
	} else {
		// Generic custom analyzer
		spec := analyzer.AnalyzerSpec{
			Name:     req.Name,
			Type:     req.Type,
			Category: "custom",
			Priority: opts.DefaultPriority,
			Config: map[string]interface{}{
				"checkName": req.Name,
				"condition": req.Condition,
				"context":   req.Context,
				"outcomes":  g.generateCustomOutcomes(req),
			},
		}
		specs = append(specs, spec)
	}

	return specs, nil
}

// Outcome generation methods

func (g *AnalyzerGenerator) generateVersionOutcomes(minVersion, maxVersion string) []map[string]interface{} {
	var outcomes []map[string]interface{}

	// Pass condition
	passCondition := "true"
	if minVersion != "" && maxVersion != "" {
		passCondition = fmt.Sprintf(">= %s && < %s", minVersion, maxVersion)
	} else if minVersion != "" {
		passCondition = fmt.Sprintf(">= %s", minVersion)
	} else if maxVersion != "" {
		passCondition = fmt.Sprintf("< %s", maxVersion)
	}

	outcomes = append(outcomes, map[string]interface{}{
		"pass": map[string]interface{}{
			"when":    passCondition,
			"message": "Kubernetes version meets requirements",
		},
	})

	// Fail condition
	if minVersion != "" {
		outcomes = append(outcomes, map[string]interface{}{
			"fail": map[string]interface{}{
				"when":    fmt.Sprintf("< %s", minVersion),
				"message": fmt.Sprintf("Kubernetes version is below minimum required version %s", minVersion),
			},
		})
	}

	if maxVersion != "" {
		outcomes = append(outcomes, map[string]interface{}{
			"fail": map[string]interface{}{
				"when":    fmt.Sprintf(">= %s", maxVersion),
				"message": fmt.Sprintf("Kubernetes version is at or above maximum supported version %s", maxVersion),
			},
		})
	}

	return outcomes
}

func (g *AnalyzerGenerator) generateComponentOutcomes(components []string, required bool) []map[string]interface{} {
	var outcomes []map[string]interface{}

	for _, component := range components {
		if required {
			outcomes = append(outcomes, map[string]interface{}{
				"fail": map[string]interface{}{
					"when":    fmt.Sprintf("missing %s", component),
					"message": fmt.Sprintf("Required component %s is missing", component),
				},
			})
		} else {
			outcomes = append(outcomes, map[string]interface{}{
				"fail": map[string]interface{}{
					"when":    fmt.Sprintf("present %s", component),
					"message": fmt.Sprintf("Forbidden component %s is present", component),
				},
			})
		}
	}

	// Default pass outcome
	if required {
		outcomes = append(outcomes, map[string]interface{}{
			"pass": map[string]interface{}{
				"message": "All required components are present",
			},
		})
	} else {
		outcomes = append(outcomes, map[string]interface{}{
			"pass": map[string]interface{}{
				"message": "No forbidden components are present",
			},
		})
	}

	return outcomes
}

func (g *AnalyzerGenerator) generateResourceOutcomes(req *analyzer.ResourceRequirements) []map[string]interface{} {
	var outcomes []map[string]interface{}

	// CPU requirements
	if req.CPU.Min != "" {
		outcomes = append(outcomes, map[string]interface{}{
			"fail": map[string]interface{}{
				"when":    fmt.Sprintf("cpu < %s", req.CPU.Min),
				"message": fmt.Sprintf("Insufficient CPU resources. Minimum required: %s", req.CPU.Min),
			},
		})
	}

	// Memory requirements
	if req.Memory.Min != "" {
		outcomes = append(outcomes, map[string]interface{}{
			"fail": map[string]interface{}{
				"when":    fmt.Sprintf("memory < %s", req.Memory.Min),
				"message": fmt.Sprintf("Insufficient memory resources. Minimum required: %s", req.Memory.Min),
			},
		})
	}

	// Disk requirements
	if req.Disk.Min != "" {
		outcomes = append(outcomes, map[string]interface{}{
			"warn": map[string]interface{}{
				"when":    fmt.Sprintf("disk < %s", req.Disk.Min),
				"message": fmt.Sprintf("Low disk space. Minimum recommended: %s", req.Disk.Min),
			},
		})
	}

	// Pass condition
	outcomes = append(outcomes, map[string]interface{}{
		"pass": map[string]interface{}{
			"message": "Resource requirements are satisfied",
		},
	})

	return outcomes
}

func (g *AnalyzerGenerator) generateClusterCapacityOutcomes(req *analyzer.ResourceRequirements) []map[string]interface{} {
	var outcomes []map[string]interface{}

	outcomes = append(outcomes, map[string]interface{}{
		"fail": map[string]interface{}{
			"when":    "clusterCapacity < requirements",
			"message": "Cluster does not have sufficient capacity to meet requirements",
		},
	})

	outcomes = append(outcomes, map[string]interface{}{
		"warn": map[string]interface{}{
			"when":    "clusterCapacity < requirements * 1.2",
			"message": "Cluster capacity is close to requirements. Consider adding buffer capacity",
		},
	})

	outcomes = append(outcomes, map[string]interface{}{
		"pass": map[string]interface{}{
			"message": "Cluster has sufficient capacity for requirements",
		},
	})

	return outcomes
}

func (g *AnalyzerGenerator) generateStorageClassOutcomes(classes []string) []map[string]interface{} {
	var outcomes []map[string]interface{}

	for _, class := range classes {
		outcomes = append(outcomes, map[string]interface{}{
			"fail": map[string]interface{}{
				"when":    fmt.Sprintf("storageClass == %s && !exists", class),
				"message": fmt.Sprintf("Required storage class %s does not exist", class),
			},
		})
	}

	outcomes = append(outcomes, map[string]interface{}{
		"pass": map[string]interface{}{
			"message": "Required storage classes are available",
		},
	})

	return outcomes
}

func (g *AnalyzerGenerator) generatePVOutcomes(req *analyzer.StorageRequirements) []map[string]interface{} {
	var outcomes []map[string]interface{}

	outcomes = append(outcomes, map[string]interface{}{
		"fail": map[string]interface{}{
			"when":    fmt.Sprintf("availableCapacity < %s", req.MinCapacity),
			"message": fmt.Sprintf("Insufficient storage capacity. Minimum required: %s", req.MinCapacity),
		},
	})

	if len(req.AccessModes) > 0 {
		for _, mode := range req.AccessModes {
			outcomes = append(outcomes, map[string]interface{}{
				"warn": map[string]interface{}{
					"when":    fmt.Sprintf("!accessMode.%s", mode),
					"message": fmt.Sprintf("Access mode %s may not be supported", mode),
				},
			})
		}
	}

	outcomes = append(outcomes, map[string]interface{}{
		"pass": map[string]interface{}{
			"message": "Storage requirements are satisfied",
		},
	})

	return outcomes
}

func (g *AnalyzerGenerator) generatePortOutcomes(port analyzer.PortRequirement) []map[string]interface{} {
	var outcomes []map[string]interface{}

	if port.Required {
		outcomes = append(outcomes, map[string]interface{}{
			"fail": map[string]interface{}{
				"when":    fmt.Sprintf("port.%d.%s == false", port.Port, strings.ToLower(port.Protocol)),
				"message": fmt.Sprintf("Required port %d/%s is not accessible", port.Port, port.Protocol),
			},
		})
	}

	outcomes = append(outcomes, map[string]interface{}{
		"pass": map[string]interface{}{
			"when":    fmt.Sprintf("port.%d.%s == true", port.Port, strings.ToLower(port.Protocol)),
			"message": fmt.Sprintf("Port %d/%s is accessible", port.Port, port.Protocol),
		},
	})

	return outcomes
}

func (g *AnalyzerGenerator) generateConnectivityOutcomes(connectivity []string) []map[string]interface{} {
	var outcomes []map[string]interface{}

	for _, target := range connectivity {
		outcomes = append(outcomes, map[string]interface{}{
			"fail": map[string]interface{}{
				"when":    fmt.Sprintf("connectivity.%s == false", target),
				"message": fmt.Sprintf("Cannot reach %s", target),
			},
		})
	}

	outcomes = append(outcomes, map[string]interface{}{
		"pass": map[string]interface{}{
			"message": "All connectivity requirements are satisfied",
		},
	})

	return outcomes
}

func (g *AnalyzerGenerator) generateCustomOutcomes(req *analyzer.CustomRequirement) []map[string]interface{} {
	var outcomes []map[string]interface{}

	// Parse the condition and generate appropriate outcomes
	condition := req.Condition
	if condition == "" {
		condition = "true"
	}

	// Basic pattern matching for common conditions
	if strings.Contains(condition, ">=") || strings.Contains(condition, ">") {
		outcomes = append(outcomes, map[string]interface{}{
			"fail": map[string]interface{}{
				"when":    fmt.Sprintf("!(%s)", condition),
				"message": fmt.Sprintf("Custom requirement '%s' not met", req.Name),
			},
		})
	}

	outcomes = append(outcomes, map[string]interface{}{
		"pass": map[string]interface{}{
			"when":    condition,
			"message": fmt.Sprintf("Custom requirement '%s' is satisfied", req.Name),
		},
	})

	return outcomes
}

// Helper methods

func (g *AnalyzerGenerator) filterByCategory(specs []analyzer.AnalyzerSpec, categories []string) []analyzer.AnalyzerSpec {
	if len(categories) == 0 {
		return specs
	}

	var filtered []analyzer.AnalyzerSpec
	categorySet := make(map[string]bool)
	for _, cat := range categories {
		categorySet[cat] = true
	}

	for _, spec := range specs {
		if categorySet[spec.Category] {
			filtered = append(filtered, spec)
		}
	}

	return filtered
}

func (g *AnalyzerGenerator) sortByPriority(specs []analyzer.AnalyzerSpec) {
	// Simple bubble sort by priority (higher first)
	n := len(specs)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-1-i; j++ {
			if specs[j].Priority < specs[j+1].Priority {
				specs[j], specs[j+1] = specs[j+1], specs[j]
			}
		}
	}
}

// Template and validator registration

func (g *AnalyzerGenerator) registerDefaultTemplates() {
	// Register built-in analyzer templates
	g.templates["database"] = AnalyzerTemplate{
		Name:        "Database Analyzer",
		Description: "Analyzes database connectivity and requirements",
		Category:    "database",
		Priority:    7,
		Generator:   g.generateDatabaseAnalyzer,
		Validator:   g.validateDatabaseRequirement,
	}

	g.templates["api"] = AnalyzerTemplate{
		Name:        "API Analyzer",
		Description: "Analyzes API endpoint connectivity and requirements",
		Category:    "api",
		Priority:    6,
		Generator:   g.generateAPIAnalyzer,
		Validator:   g.validateAPIRequirement,
	}
}

func (g *AnalyzerGenerator) registerDefaultValidators() {
	g.validators["version"] = func(req interface{}) error {
		// Validate version format
		versionStr, ok := req.(string)
		if !ok {
			return errors.New("version must be a string")
		}

		// Basic semantic version validation
		versionRegex := regexp.MustCompile(`^v?\d+\.\d+(\.\d+)?(-.*)?$`)
		if !versionRegex.MatchString(versionStr) {
			return errors.Errorf("invalid version format: %s", versionStr)
		}

		return nil
	}

	g.validators["resource"] = func(req interface{}) error {
		// Validate resource specifications
		resourceStr, ok := req.(string)
		if !ok {
			return errors.New("resource must be a string")
		}

		// Validate resource format (e.g., "100m", "1Gi", "500Mi")
		resourceRegex := regexp.MustCompile(`^(\d+(\.\d+)?)(m|Mi|Gi|Ti|Ki|k|M|G|T)?$`)
		if !resourceRegex.MatchString(resourceStr) {
			return errors.Errorf("invalid resource format: %s", resourceStr)
		}

		return nil
	}
}

// Custom analyzer generators

func (g *AnalyzerGenerator) generateDatabaseAnalyzer(ctx context.Context, req interface{}) ([]analyzer.AnalyzerSpec, error) {
	customReq, ok := req.(*analyzer.CustomRequirement)
	if !ok {
		return nil, errors.New("invalid database requirement type")
	}

	spec := analyzer.AnalyzerSpec{
		Name:     fmt.Sprintf("database-%s", customReq.Name),
		Type:     "database",
		Category: "database",
		Priority: 7,
		Config: map[string]interface{}{
			"checkName": fmt.Sprintf("Database %s Check", customReq.Name),
			"uri":       customReq.Context["uri"],
			"timeout":   "10s",
			"outcomes": []map[string]interface{}{
				{
					"fail": map[string]interface{}{
						"when":    "error",
						"message": "Database connection failed",
					},
				},
				{
					"pass": map[string]interface{}{
						"message": "Database connection successful",
					},
				},
			},
		},
	}

	return []analyzer.AnalyzerSpec{spec}, nil
}

func (g *AnalyzerGenerator) generateAPIAnalyzer(ctx context.Context, req interface{}) ([]analyzer.AnalyzerSpec, error) {
	customReq, ok := req.(*analyzer.CustomRequirement)
	if !ok {
		return nil, errors.New("invalid API requirement type")
	}

	spec := analyzer.AnalyzerSpec{
		Name:     fmt.Sprintf("api-%s", customReq.Name),
		Type:     "http",
		Category: "api",
		Priority: 6,
		Config: map[string]interface{}{
			"checkName": fmt.Sprintf("API %s Check", customReq.Name),
			"get": map[string]interface{}{
				"url": customReq.Context["url"],
			},
			"outcomes": []map[string]interface{}{
				{
					"fail": map[string]interface{}{
						"when":    "status != 200",
						"message": "API endpoint is not accessible",
					},
				},
				{
					"pass": map[string]interface{}{
						"when":    "status == 200",
						"message": "API endpoint is accessible",
					},
				},
			},
		},
	}

	return []analyzer.AnalyzerSpec{spec}, nil
}

// Custom requirement validators

func (g *AnalyzerGenerator) validateDatabaseRequirement(req interface{}) error {
	customReq, ok := req.(*analyzer.CustomRequirement)
	if !ok {
		return errors.New("invalid requirement type")
	}

	if customReq.Context == nil {
		return errors.New("database requirement must have context")
	}

	if _, exists := customReq.Context["uri"]; !exists {
		return errors.New("database requirement must specify 'uri' in context")
	}

	return nil
}

func (g *AnalyzerGenerator) validateAPIRequirement(req interface{}) error {
	customReq, ok := req.(*analyzer.CustomRequirement)
	if !ok {
		return errors.New("invalid requirement type")
	}

	if customReq.Context == nil {
		return errors.New("API requirement must have context")
	}

	if _, exists := customReq.Context["url"]; !exists {
		return errors.New("API requirement must specify 'url' in context")
	}

	return nil
}

// RegisterTemplate registers a custom analyzer template
func (g *AnalyzerGenerator) RegisterTemplate(name string, template AnalyzerTemplate) error {
	if name == "" {
		return errors.New("template name cannot be empty")
	}

	if template.Generator == nil {
		return errors.New("template generator cannot be nil")
	}

	g.templates[name] = template
	return nil
}

// RegisterValidator registers a custom requirement validator
func (g *AnalyzerGenerator) RegisterValidator(name string, validator RequirementValidator) error {
	if name == "" {
		return errors.New("validator name cannot be empty")
	}

	if validator == nil {
		return errors.New("validator cannot be nil")
	}

	g.validators[name] = validator
	return nil
}

// ValidateRequirements validates a requirement specification
func (g *AnalyzerGenerator) ValidateRequirements(ctx context.Context, requirements *analyzer.RequirementSpec) error {
	if requirements == nil {
		return errors.New("requirements cannot be nil")
	}

	// Validate Kubernetes requirements
	if err := g.validateKubernetesRequirements(&requirements.Spec.Kubernetes); err != nil {
		return errors.Wrap(err, "invalid Kubernetes requirements")
	}

	// Validate resource requirements
	if err := g.validateResourceRequirements(&requirements.Spec.Resources); err != nil {
		return errors.Wrap(err, "invalid resource requirements")
	}

	// Validate storage requirements
	if err := g.validateStorageRequirements(&requirements.Spec.Storage); err != nil {
		return errors.Wrap(err, "invalid storage requirements")
	}

	// Validate network requirements
	if err := g.validateNetworkRequirements(&requirements.Spec.Network); err != nil {
		return errors.Wrap(err, "invalid network requirements")
	}

	// Validate custom requirements
	for i, customReq := range requirements.Spec.Custom {
		if err := g.validateCustomRequirement(&customReq); err != nil {
			return errors.Wrapf(err, "invalid custom requirement at index %d", i)
		}
	}

	return nil
}

func (g *AnalyzerGenerator) validateKubernetesRequirements(req *analyzer.KubernetesRequirements) error {
	if req.MinVersion != "" {
		if err := g.validators["version"](req.MinVersion); err != nil {
			return errors.Wrap(err, "invalid minVersion")
		}
	}

	if req.MaxVersion != "" {
		if err := g.validators["version"](req.MaxVersion); err != nil {
			return errors.Wrap(err, "invalid maxVersion")
		}
	}

	return nil
}

func (g *AnalyzerGenerator) validateResourceRequirements(req *analyzer.ResourceRequirements) error {
	if req.CPU.Min != "" {
		if err := g.validators["resource"](req.CPU.Min); err != nil {
			return errors.Wrap(err, "invalid CPU minimum")
		}
	}

	if req.Memory.Min != "" {
		if err := g.validators["resource"](req.Memory.Min); err != nil {
			return errors.Wrap(err, "invalid memory minimum")
		}
	}

	if req.Disk.Min != "" {
		if err := g.validators["resource"](req.Disk.Min); err != nil {
			return errors.Wrap(err, "invalid disk minimum")
		}
	}

	return nil
}

func (g *AnalyzerGenerator) validateStorageRequirements(req *analyzer.StorageRequirements) error {
	if req.MinCapacity != "" {
		if err := g.validators["resource"](req.MinCapacity); err != nil {
			return errors.Wrap(err, "invalid minCapacity")
		}
	}

	// Validate access modes
	validAccessModes := map[string]bool{
		"ReadWriteOnce": true,
		"ReadOnlyMany":  true,
		"ReadWriteMany": true,
	}

	for _, mode := range req.AccessModes {
		if !validAccessModes[mode] {
			return errors.Errorf("invalid access mode: %s", mode)
		}
	}

	return nil
}

func (g *AnalyzerGenerator) validateNetworkRequirements(req *analyzer.NetworkRequirements) error {
	for _, port := range req.Ports {
		if port.Port <= 0 || port.Port > 65535 {
			return errors.Errorf("invalid port number: %d", port.Port)
		}

		validProtocols := map[string]bool{
			"TCP": true,
			"UDP": true,
		}

		if port.Protocol != "" && !validProtocols[strings.ToUpper(port.Protocol)] {
			return errors.Errorf("invalid protocol: %s", port.Protocol)
		}
	}

	return nil
}

func (g *AnalyzerGenerator) validateCustomRequirement(req *analyzer.CustomRequirement) error {
	if req.Name == "" {
		return errors.New("custom requirement name cannot be empty")
	}

	if req.Type == "" {
		return errors.New("custom requirement type cannot be empty")
	}

	// Check if we have a specific validator for this type
	if validator, exists := g.validators[req.Type]; exists {
		return validator(req)
	}

	return nil
}
