package generators

import (
	"strings"
	"testing"
)

// TestNewRequirementParser tests creating a new requirement parser
func TestNewRequirementParser(t *testing.T) {
	parser := NewRequirementParser()

	if parser == nil {
		t.Fatal("expected non-nil parser")
	}

	if parser.validator == nil {
		t.Fatal("expected validator to be initialized")
	}

	if parser.categorizer == nil {
		t.Fatal("expected categorizer to be initialized")
	}

	if parser.conflictResolver == nil {
		t.Fatal("expected conflict resolver to be initialized")
	}
}

// TestParseFromBytes tests parsing requirement specifications from bytes
func TestParseFromBytes(t *testing.T) {
	parser := NewRequirementParser()

	jsonData := `{
		"apiVersion": "troubleshoot.sh/v1beta2",
		"kind": "RequirementSpec",
		"metadata": {
			"name": "test-requirements",
			"version": "1.0.0"
		},
		"spec": {
			"kubernetes": {
				"minVersion": "1.20.0",
				"nodeCount": {
					"min": 3
				}
			},
			"resources": {
				"cpu": {
					"minCores": 2
				},
				"memory": {
					"minBytes": 4294967296
				}
			}
		}
	}`

	opts := DefaultParseOptions()
	result, err := parser.ParseFromBytes([]byte(jsonData), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if result.Spec == nil {
		t.Fatal("expected non-nil spec")
	}

	// Check basic structure
	if result.Spec.APIVersion != "troubleshoot.sh/v1beta2" {
		t.Errorf("expected apiVersion 'troubleshoot.sh/v1beta2', got %q", result.Spec.APIVersion)
	}

	if result.Spec.Kind != "RequirementSpec" {
		t.Errorf("expected kind 'RequirementSpec', got %q", result.Spec.Kind)
	}

	if result.Spec.Metadata.Name != "test-requirements" {
		t.Errorf("expected name 'test-requirements', got %q", result.Spec.Metadata.Name)
	}

	// Check that requirements were categorized
	if len(result.CategorizedReqs) == 0 {
		t.Fatal("expected categorized requirements")
	}

	// Check summary
	if result.Summary.TotalRequirements == 0 {
		t.Error("expected non-zero total requirements in summary")
	}
}

// TestParseYAML tests parsing YAML format
func TestParseYAML(t *testing.T) {
	parser := NewRequirementParser()

	yamlData := `
apiVersion: troubleshoot.sh/v1beta2
kind: RequirementSpec
metadata:
  name: yaml-test-requirements
  version: "1.0.0"
spec:
  kubernetes:
    minVersion: "1.21.0"
    nodeCount:
      min: 2
  storage:
    minCapacity: 1073741824
`

	opts := DefaultParseOptions()
	opts.Format = "yaml"

	result, err := parser.ParseFromBytes([]byte(yamlData), opts)
	if err != nil {
		t.Fatalf("unexpected error parsing YAML: %v", err)
	}

	if result.Spec.Metadata.Name != "yaml-test-requirements" {
		t.Errorf("expected name 'yaml-test-requirements', got %q", result.Spec.Metadata.Name)
	}

	if result.Spec.Spec.Kubernetes.MinVersion != "1.21.0" {
		t.Errorf("expected Kubernetes minVersion '1.21.0', got %q", result.Spec.Spec.Kubernetes.MinVersion)
	}
}

// TestFormatDetection tests automatic format detection
func TestFormatDetection(t *testing.T) {
	parser := NewRequirementParser()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "JSON format",
			content:  `{"apiVersion": "test"}`,
			expected: "json",
		},
		{
			name: "YAML format",
			content: `apiVersion: test
kind: RequirementSpec`,
			expected: "yaml",
		},
		{
			name:     "YAML with separators",
			content:  `---\napiVersion: test`,
			expected: "yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detected := parser.detectFormat([]byte(tt.content))
			if detected != tt.expected {
				t.Errorf("expected format %q, got %q", tt.expected, detected)
			}
		})
	}
}

// TestDefaultParseOptions tests default parse options
func TestDefaultParseOptions(t *testing.T) {
	opts := DefaultParseOptions()

	if opts.Format != "auto" {
		t.Errorf("expected default format 'auto', got %q", opts.Format)
	}

	if !opts.AllowUnknown {
		t.Error("expected AllowUnknown to be true by default")
	}

	if !opts.ResolveConflicts {
		t.Error("expected ResolveConflicts to be true by default")
	}

	if !opts.ValidateSchema {
		t.Error("expected ValidateSchema to be true by default")
	}
}

// TestValidationErrors tests handling of validation errors
func TestValidationErrors(t *testing.T) {
	parser := NewRequirementParser()

	invalidData := `{
		"kind": "RequirementSpec",
		"metadata": {
			"name": "invalid-name-with-CAPS"
		},
		"spec": {}
	}`

	opts := DefaultParseOptions()
	opts.StrictMode = false // Don't fail on validation errors

	result, err := parser.ParseFromBytes([]byte(invalidData), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have validation errors
	if len(result.ValidationErrors) == 0 {
		t.Error("expected validation errors for invalid data")
	}

	// Test strict mode
	opts.StrictMode = true
	_, err = parser.ParseFromBytes([]byte(invalidData), opts)
	if err == nil {
		t.Error("expected error in strict mode with validation errors")
	}
}

// TestMergeSpecs tests merging multiple specifications
func TestMergeSpecs(t *testing.T) {
	parser := NewRequirementParser()

	spec1Data := `{
		"apiVersion": "troubleshoot.sh/v1beta2",
		"kind": "RequirementSpec",
		"metadata": {"name": "spec1"},
		"spec": {
			"kubernetes": {"minVersion": "1.20.0"},
			"resources": {"cpu": {"minCores": 2}}
		}
	}`

	spec2Data := `{
		"apiVersion": "troubleshoot.sh/v1beta2",
		"kind": "RequirementSpec",
		"metadata": {"name": "spec2"},
		"spec": {
			"kubernetes": {"maxVersion": "1.25.0"},
			"resources": {"memory": {"minBytes": 4294967296}}
		}
	}`

	opts := DefaultParseOptions()

	// Parse both specs
	result1, err := parser.ParseFromBytes([]byte(spec1Data), opts)
	if err != nil {
		t.Fatalf("error parsing spec1: %v", err)
	}

	result2, err := parser.ParseFromBytes([]byte(spec2Data), opts)
	if err != nil {
		t.Fatalf("error parsing spec2: %v", err)
	}

	// Merge specs
	merged, err := parser.MergeSpecs([]*RequirementSpec{result1.Spec, result2.Spec}, opts)
	if err != nil {
		t.Fatalf("error merging specs: %v", err)
	}

	// Verify merged content
	if merged.Spec.Spec.Kubernetes.MinVersion != "1.20.0" {
		t.Errorf("expected minVersion from spec1, got %q", merged.Spec.Spec.Kubernetes.MinVersion)
	}

	if merged.Spec.Spec.Kubernetes.MaxVersion != "1.25.0" {
		t.Errorf("expected maxVersion from spec2, got %q", merged.Spec.Spec.Kubernetes.MaxVersion)
	}

	if merged.Spec.Spec.Resources.CPU.MinCores != 2 {
		t.Errorf("expected CPU cores from spec1, got %f", merged.Spec.Spec.Resources.CPU.MinCores)
	}

	if merged.Spec.Spec.Resources.Memory.MinBytes != 4294967296 {
		t.Errorf("expected memory from spec2, got %d", merged.Spec.Spec.Resources.Memory.MinBytes)
	}
}

// TestConflictResolution tests conflict resolution during parsing
func TestConflictResolution(t *testing.T) {
	parser := NewRequirementParser()

	conflictingData := `{
		"apiVersion": "troubleshoot.sh/v1beta2",
		"kind": "RequirementSpec",
		"metadata": {"name": "conflicting"},
		"spec": {
			"kubernetes": {
				"minVersion": "1.25.0",
				"maxVersion": "1.20.0"
			}
		}
	}`

	opts := DefaultParseOptions()
	result, err := parser.ParseFromBytes([]byte(conflictingData), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should detect conflicts
	if len(result.Conflicts) == 0 {
		t.Error("expected conflicts to be detected")
	}

	// Should have resolved requirements
	if len(result.ResolvedRequirements) == 0 {
		t.Error("expected resolved requirements")
	}
}

// TestParseEmptySpec tests parsing empty specifications
func TestParseEmptySpec(t *testing.T) {
	parser := NewRequirementParser()

	emptyData := `{
		"apiVersion": "troubleshoot.sh/v1beta2",
		"kind": "RequirementSpec",
		"metadata": {"name": "empty"},
		"spec": {}
	}`

	opts := DefaultParseOptions()
	result, err := parser.ParseFromBytes([]byte(emptyData), opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still have basic structure
	if result.Spec.APIVersion != "troubleshoot.sh/v1beta2" {
		t.Errorf("expected apiVersion to be set, got %q", result.Spec.APIVersion)
	}

	// Summary should reflect empty spec
	if result.Summary.TotalRequirements != 0 {
		t.Errorf("expected 0 requirements for empty spec, got %d", result.Summary.TotalRequirements)
	}
}

// TestInvalidJSON tests handling of invalid JSON
func TestInvalidJSON(t *testing.T) {
	parser := NewRequirementParser()

	invalidJSON := `{"apiVersion": "test", "incomplete": }`

	opts := DefaultParseOptions()
	_, err := parser.ParseFromBytes([]byte(invalidJSON), opts)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}
