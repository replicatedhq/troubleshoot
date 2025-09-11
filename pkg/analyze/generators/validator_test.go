package generators

import (
	"strings"
	"testing"
)

// TestNewRequirementValidator tests creating a new requirement validator
func TestNewRequirementValidator(t *testing.T) {
	validator := NewRequirementValidator()

	if validator == nil {
		t.Fatal("expected non-nil validator")
	}

	if validator.rules == nil {
		t.Fatal("expected rules to be initialized")
	}

	if len(validator.rules) == 0 {
		t.Error("expected default rules to be loaded")
	}
}

// TestValidateBasicStructure tests basic structure validation
func TestValidateBasicStructure(t *testing.T) {
	validator := NewRequirementValidator()

	tests := []struct {
		name      string
		spec      *RequirementSpec
		expectErr bool
	}{
		{
			name: "valid spec",
			spec: &RequirementSpec{
				APIVersion: "troubleshoot.sh/v1beta2",
				Kind:       "RequirementSpec",
				Metadata:   RequirementMetadata{Name: "test"},
			},
			expectErr: false,
		},
		{
			name:      "nil spec",
			spec:      nil,
			expectErr: true,
		},
		{
			name: "missing apiVersion",
			spec: &RequirementSpec{
				Kind:     "RequirementSpec",
				Metadata: RequirementMetadata{Name: "test"},
			},
			expectErr: true,
		},
		{
			name: "missing kind",
			spec: &RequirementSpec{
				APIVersion: "troubleshoot.sh/v1beta2",
				Metadata:   RequirementMetadata{Name: "test"},
			},
			expectErr: true,
		},
		{
			name: "invalid apiVersion format",
			spec: &RequirementSpec{
				APIVersion: "invalid-version",
				Kind:       "RequirementSpec",
				Metadata:   RequirementMetadata{Name: "test"},
			},
			expectErr: true,
		},
		{
			name: "invalid kind",
			spec: &RequirementSpec{
				APIVersion: "troubleshoot.sh/v1beta2",
				Kind:       "InvalidKind",
				Metadata:   RequirementMetadata{Name: "test"},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateBasicStructure(tt.spec)
			if (err != nil) != tt.expectErr {
				t.Errorf("validateBasicStructure() error = %v, expectErr %v", err, tt.expectErr)
			}
		})
	}
}

// TestValidateMetadata tests metadata validation
func TestValidateMetadata(t *testing.T) {
	validator := NewRequirementValidator()

	tests := []struct {
		name           string
		metadata       RequirementMetadata
		expectErrors   int
		expectWarnings int
	}{
		{
			name: "valid metadata",
			metadata: RequirementMetadata{
				Name:    "valid-name",
				Version: "1.0.0",
				Tags:    []string{"test", "valid"},
			},
			expectErrors:   0,
			expectWarnings: 0,
		},
		{
			name: "missing name",
			metadata: RequirementMetadata{
				Version: "1.0.0",
			},
			expectErrors:   1,
			expectWarnings: 0,
		},
		{
			name: "invalid name format",
			metadata: RequirementMetadata{
				Name: "Invalid_Name_With_Underscores",
			},
			expectErrors:   1,
			expectWarnings: 0,
		},
		{
			name: "invalid version format",
			metadata: RequirementMetadata{
				Name:    "valid-name",
				Version: "not-semver",
			},
			expectErrors:   0,
			expectWarnings: 1,
		},
		{
			name: "invalid tags",
			metadata: RequirementMetadata{
				Name: "valid-name",
				Tags: []string{"valid-tag", "invalid tag with spaces"},
			},
			expectErrors:   1,
			expectWarnings: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors, warnings := validator.validateMetadata(&tt.metadata)

			if len(errors) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d", tt.expectErrors, len(errors))
			}

			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d", tt.expectWarnings, len(warnings))
			}
		})
	}
}

// TestValidateKubernetesRequirements tests Kubernetes requirements validation
func TestValidateKubernetesRequirements(t *testing.T) {
	validator := NewRequirementValidator()

	tests := []struct {
		name         string
		requirements KubernetesRequirements
		expectErrors int
	}{
		{
			name: "valid requirements",
			requirements: KubernetesRequirements{
				MinVersion: "1.20.0",
				MaxVersion: "1.25.0",
				NodeCount:  NodeCountRequirement{Min: 3, Max: 10},
			},
			expectErrors: 0,
		},
		{
			name: "invalid version format",
			requirements: KubernetesRequirements{
				MinVersion: "invalid-version",
				MaxVersion: "1.25.0",
			},
			expectErrors: 1,
		},
		{
			name: "invalid version range",
			requirements: KubernetesRequirements{
				MinVersion: "1.25.0",
				MaxVersion: "1.20.0", // max < min
			},
			expectErrors: 1,
		},
		{
			name: "negative node count",
			requirements: KubernetesRequirements{
				NodeCount: NodeCountRequirement{Min: -1},
			},
			expectErrors: 1,
		},
		{
			name: "invalid node count range",
			requirements: KubernetesRequirements{
				NodeCount: NodeCountRequirement{Min: 10, Max: 5}, // max < min
			},
			expectErrors: 1,
		},
		{
			name: "empty API requirement",
			requirements: KubernetesRequirements{
				APIs: []APIRequirement{{ /* empty */ }},
			},
			expectErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors, _ := validator.validateKubernetesRequirements(&tt.requirements)

			if len(errors) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d", tt.expectErrors, len(errors))
			}
		})
	}
}

// TestValidateResourceRequirements tests resource requirements validation
func TestValidateResourceRequirements(t *testing.T) {
	validator := NewRequirementValidator()

	tests := []struct {
		name         string
		requirements ResourceRequirements
		expectErrors int
	}{
		{
			name: "valid requirements",
			requirements: ResourceRequirements{
				CPU:    CPURequirement{MinCores: 2.0, MaxUtilization: 0.8},
				Memory: MemoryRequirement{MinBytes: 4294967296, MaxUtilization: 0.9},
				Nodes:  NodeRequirement{MinNodes: 3, MaxNodes: 10},
			},
			expectErrors: 0,
		},
		{
			name: "negative CPU cores",
			requirements: ResourceRequirements{
				CPU: CPURequirement{MinCores: -1.0},
			},
			expectErrors: 1,
		},
		{
			name: "invalid CPU utilization",
			requirements: ResourceRequirements{
				CPU: CPURequirement{MaxUtilization: 1.5}, // > 1.0
			},
			expectErrors: 1,
		},
		{
			name: "negative memory",
			requirements: ResourceRequirements{
				Memory: MemoryRequirement{MinBytes: -1},
			},
			expectErrors: 1,
		},
		{
			name: "invalid memory utilization",
			requirements: ResourceRequirements{
				Memory: MemoryRequirement{MaxUtilization: -0.1}, // < 0
			},
			expectErrors: 1,
		},
		{
			name: "negative nodes",
			requirements: ResourceRequirements{
				Nodes: NodeRequirement{MinNodes: -1},
			},
			expectErrors: 1,
		},
		{
			name: "invalid node range",
			requirements: ResourceRequirements{
				Nodes: NodeRequirement{MinNodes: 10, MaxNodes: 5},
			},
			expectErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors, _ := validator.validateResourceRequirements(&tt.requirements)

			if len(errors) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d", tt.expectErrors, len(errors))
			}
		})
	}
}

// TestValidateStorageRequirements tests storage requirements validation
func TestValidateStorageRequirements(t *testing.T) {
	validator := NewRequirementValidator()

	tests := []struct {
		name         string
		requirements StorageRequirements
		expectErrors int
	}{
		{
			name: "valid requirements",
			requirements: StorageRequirements{
				MinCapacity: 1073741824, // 1GB
				StorageClasses: []StorageClassRequirement{
					{Name: "fast-ssd", Provisioner: "kubernetes.io/gce-pd"},
				},
				Performance: PerformanceRequirement{MinIOPS: 1000, MinThroughput: 100},
			},
			expectErrors: 0,
		},
		{
			name: "negative capacity",
			requirements: StorageRequirements{
				MinCapacity: -1,
			},
			expectErrors: 1,
		},
		{
			name: "storage class missing name",
			requirements: StorageRequirements{
				StorageClasses: []StorageClassRequirement{
					{Provisioner: "kubernetes.io/gce-pd"}, // missing name
				},
			},
			expectErrors: 1,
		},
		{
			name: "storage class missing provisioner",
			requirements: StorageRequirements{
				StorageClasses: []StorageClassRequirement{
					{Name: "fast-ssd"}, // missing provisioner
				},
			},
			expectErrors: 1,
		},
		{
			name: "negative performance requirements",
			requirements: StorageRequirements{
				Performance: PerformanceRequirement{MinIOPS: -1, MinThroughput: -100},
			},
			expectErrors: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors, _ := validator.validateStorageRequirements(&tt.requirements)

			if len(errors) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d", tt.expectErrors, len(errors))
			}
		})
	}
}

// TestValidateNetworkRequirements tests network requirements validation
func TestValidateNetworkRequirements(t *testing.T) {
	validator := NewRequirementValidator()

	tests := []struct {
		name         string
		requirements NetworkRequirements
		expectErrors int
	}{
		{
			name: "valid requirements",
			requirements: NetworkRequirements{
				Bandwidth: BandwidthRequirement{MinUpload: 1000000, MinDownload: 10000000},
				Latency:   LatencyRequirement{MaxRTT: 100},
				DNS:       DNSRequirement{Servers: []string{"8.8.8.8", "dns.example.com"}},
				Connectivity: []ConnectivityRequirement{
					{Endpoint: "api.example.com", Port: 443, Type: "https"},
				},
			},
			expectErrors: 0,
		},
		{
			name: "negative bandwidth",
			requirements: NetworkRequirements{
				Bandwidth: BandwidthRequirement{MinUpload: -1, MinDownload: -1},
			},
			expectErrors: 2,
		},
		{
			name: "negative latency",
			requirements: NetworkRequirements{
				Latency: LatencyRequirement{MaxRTT: -1},
			},
			expectErrors: 1,
		},
		{
			name: "invalid DNS server",
			requirements: NetworkRequirements{
				DNS: DNSRequirement{Servers: []string{"invalid-dns-server-format"}},
			},
			expectErrors: 1,
		},
		{
			name: "connectivity missing endpoint",
			requirements: NetworkRequirements{
				Connectivity: []ConnectivityRequirement{
					{Port: 443, Type: "https"}, // missing endpoint
				},
			},
			expectErrors: 1,
		},
		{
			name: "connectivity invalid port",
			requirements: NetworkRequirements{
				Connectivity: []ConnectivityRequirement{
					{Endpoint: "api.example.com", Port: 70000}, // port > 65535
				},
			},
			expectErrors: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors, _ := validator.validateNetworkRequirements(&tt.requirements)

			if len(errors) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d", tt.expectErrors, len(errors))
			}
		})
	}
}

// TestValidateCustomRequirements tests custom requirements validation
func TestValidateCustomRequirements(t *testing.T) {
	validator := NewRequirementValidator()

	tests := []struct {
		name           string
		requirements   []CustomRequirement
		expectErrors   int
		expectWarnings int
	}{
		{
			name: "valid requirements",
			requirements: []CustomRequirement{
				{Name: "custom-check", Type: "application", Required: true},
			},
			expectErrors:   0,
			expectWarnings: 0,
		},
		{
			name: "missing name",
			requirements: []CustomRequirement{
				{Type: "application", Required: true}, // missing name
			},
			expectErrors:   1,
			expectWarnings: 0,
		},
		{
			name: "missing type",
			requirements: []CustomRequirement{
				{Name: "custom-check", Required: true}, // missing type
			},
			expectErrors:   0,
			expectWarnings: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors, warnings := validator.validateCustomRequirements(tt.requirements)

			if len(errors) != tt.expectErrors {
				t.Errorf("expected %d errors, got %d", tt.expectErrors, len(errors))
			}

			if len(warnings) != tt.expectWarnings {
				t.Errorf("expected %d warnings, got %d", tt.expectWarnings, len(warnings))
			}
		})
	}
}

// TestHelperValidationFunctions tests helper validation functions
func TestHelperValidationFunctions(t *testing.T) {
	validator := NewRequirementValidator()

	// Test API version validation
	validAPIVersions := []string{
		"troubleshoot.sh/v1beta2",
		"example.com/v1",
		"api.test/v2alpha1",
	}

	for _, version := range validAPIVersions {
		if !validator.isValidAPIVersion(version) {
			t.Errorf("expected %q to be valid API version", version)
		}
	}

	invalidAPIVersions := []string{
		"invalid",
		"no-slash",
		"missing/version",
		"123invalid/v1",
	}

	for _, version := range invalidAPIVersions {
		if validator.isValidAPIVersion(version) {
			t.Errorf("expected %q to be invalid API version", version)
		}
	}

	// Test name validation
	validNames := []string{
		"valid-name",
		"test123",
		"a-b-c-123",
	}

	for _, name := range validNames {
		if !validator.isValidName(name) {
			t.Errorf("expected %q to be valid name", name)
		}
	}

	invalidNames := []string{
		"Invalid-Name",
		"name_with_underscores",
		"-starts-with-dash",
		"ends-with-dash-",
		"has spaces",
	}

	for _, name := range invalidNames {
		if validator.isValidName(name) {
			t.Errorf("expected %q to be invalid name", name)
		}
	}

	// Test Kubernetes version validation
	validK8sVersions := []string{
		"1.20.0",
		"v1.21.5",
		"1.22.0-beta.1",
	}

	for _, version := range validK8sVersions {
		if !validator.isValidKubernetesVersion(version) {
			t.Errorf("expected %q to be valid Kubernetes version", version)
		}
	}

	invalidK8sVersions := []string{
		"invalid",
		"1.20",    // missing patch version
		"v1.21.x", // non-numeric patch
	}

	for _, version := range invalidK8sVersions {
		if validator.isValidKubernetesVersion(version) {
			t.Errorf("expected %q to be invalid Kubernetes version", version)
		}
	}
}

// TestAddValidationRule tests adding custom validation rules
func TestAddValidationRule(t *testing.T) {
	validator := NewRequirementValidator()
	initialCount := len(validator.rules)

	customRule := ValidationRule{
		Name:        "custom_test_rule",
		Description: "Custom test rule",
		Path:        "test.custom",
		Required:    true,
	}

	validator.AddValidationRule(customRule)

	if len(validator.rules) != initialCount+1 {
		t.Errorf("expected %d rules, got %d", initialCount+1, len(validator.rules))
	}

	// Check if the rule was added
	found := false
	for _, rule := range validator.rules {
		if rule.Name == "custom_test_rule" {
			found = true
			break
		}
	}

	if !found {
		t.Error("custom rule was not found in rules list")
	}
}

// TestValidateSpec tests the high-level ValidateSpec method
func TestValidateSpec(t *testing.T) {
	validator := NewRequirementValidator()

	validSpec := &RequirementSpec{
		APIVersion: "troubleshoot.sh/v1beta2",
		Kind:       "RequirementSpec",
		Metadata: RequirementMetadata{
			Name:    "test-spec",
			Version: "1.0.0",
		},
		Spec: RequirementSpecDetails{
			Kubernetes: KubernetesRequirements{
				MinVersion: "1.20.0",
				NodeCount:  NodeCountRequirement{Min: 3},
			},
			Resources: ResourceRequirements{
				CPU:    CPURequirement{MinCores: 2.0},
				Memory: MemoryRequirement{MinBytes: 4294967296},
			},
		},
	}

	errors, warnings, err := validator.ValidateSpec(validSpec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(errors) > 0 {
		t.Errorf("expected no errors for valid spec, got %d", len(errors))
	}

	// Test with invalid spec
	invalidSpec := &RequirementSpec{
		Kind: "RequirementSpec", // missing apiVersion
		Metadata: RequirementMetadata{
			Name: "INVALID-NAME", // invalid name format
		},
	}

	errors, warnings, err = validator.ValidateSpec(invalidSpec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(errors) == 0 {
		t.Error("expected errors for invalid spec")
	}

	// Test legacy Validate method
	err = validator.Validate(validSpec)
	if err != nil {
		t.Errorf("unexpected error from legacy Validate: %v", err)
	}

	err = validator.Validate(invalidSpec)
	if err == nil {
		t.Error("expected error from legacy Validate for invalid spec")
	} else if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("expected validation failure message, got: %v", err)
	}
}
