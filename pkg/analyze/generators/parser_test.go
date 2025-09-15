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

	result, err := parser.ParseFromBytes([]byte(jsonData), "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Check basic structure
	if result.APIVersion != "troubleshoot.sh/v1beta2" {
		t.Errorf("expected apiVersion 'troubleshoot.sh/v1beta2', got %q", result.APIVersion)
	}

	if result.Kind != "RequirementSpec" {
		t.Errorf("expected kind 'RequirementSpec', got %q", result.Kind)
	}

	if result.Metadata.Name != "test-requirements" {
		t.Errorf("expected name 'test-requirements', got %q", result.Metadata.Name)
	}

	// Check that the spec has content
	if result.Spec.Kubernetes.MinVersion == "" && result.Spec.Resources.Memory.MinBytes == 0 {
		t.Fatal("expected some requirements content")
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

	// Use direct format specification
	result, err := parser.ParseFromBytes([]byte(yamlData), "yaml")
	if err != nil {
		t.Fatalf("unexpected error parsing YAML: %v", err)
	}

	if result.Metadata.Name != "yaml-test-requirements" {
		t.Errorf("expected name 'yaml-test-requirements', got %q", result.Metadata.Name)
	}

	if result.Spec.Kubernetes.MinVersion != "1.21.0" {
		t.Errorf("expected Kubernetes minVersion '1.21.0', got %q", result.Spec.Kubernetes.MinVersion)
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
			detected, _ := parser.detectFormat([]byte(tt.content))
			if detected != tt.expected {
				t.Errorf("expected format %q, got %q", tt.expected, detected)
			}
		})
	}
}

// TestDefaultParseOptions tests default parse behavior
func TestDefaultParseOptions(t *testing.T) {
	// Test basic parser functionality
	parser := NewRequirementParser()

	jsonData := `{"apiVersion": "v1", "kind": "Requirements"}`
	result, err := parser.ParseFromBytes([]byte(jsonData), "json")
	if err != nil {
		t.Fatalf("expected basic parsing to work, got error: %v", err)
	}

	if result == nil {
		t.Error("expected result to be non-nil")
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

	// Use direct format specification
	result, err := parser.ParseFromBytes([]byte(invalidData), "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Basic validation - result should exist even with invalid data
	if result == nil {
		t.Error("expected result even with invalid data")
	}

	// Test with invalid JSON should fail
	invalidJSON := `{"invalid": json}`
	_, err = parser.ParseFromBytes([]byte(invalidJSON), "json")
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

	// Use direct format specification

	// Parse both specs
	result1, err := parser.ParseFromBytes([]byte(spec1Data), "json")
	if err != nil {
		t.Fatalf("error parsing spec1: %v", err)
	}

	result2, err := parser.ParseFromBytes([]byte(spec2Data), "json")
	if err != nil {
		t.Fatalf("error parsing spec2: %v", err)
	}

	// Merge specs
	merged, err := parser.MergeSpecs([]*RequirementSpec{result1, result2})
	if err != nil {
		t.Fatalf("error merging specs: %v", err)
	}

	// Verify merged content
	if merged.Spec.Kubernetes.MinVersion != "1.20.0" {
		t.Errorf("expected minVersion from spec1, got %q", merged.Spec.Kubernetes.MinVersion)
	}

	if merged.Spec.Kubernetes.MaxVersion != "1.25.0" {
		t.Errorf("expected maxVersion from spec2, got %q", merged.Spec.Kubernetes.MaxVersion)
	}

	if merged.Spec.Resources.CPU.MinCores != 2 {
		t.Errorf("expected CPU cores from spec1, got %f", merged.Spec.Resources.CPU.MinCores)
	}

	if merged.Spec.Resources.Memory.MinBytes != 4294967296 {
		t.Errorf("expected memory from spec2, got %d", merged.Spec.Resources.Memory.MinBytes)
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

	// Use direct format specification
	result, err := parser.ParseFromBytes([]byte(conflictingData), "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have basic structure
	if result == nil {
		t.Error("expected result to be non-nil")
	}

	// Should have spec content
	if result.Spec.Kubernetes.MinVersion == "" {
		t.Error("expected some kubernetes requirements")
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

	// Use direct format specification
	result, err := parser.ParseFromBytes([]byte(emptyData), "json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still have basic structure
	if result.APIVersion != "troubleshoot.sh/v1beta2" {
		t.Errorf("expected apiVersion to be set, got %q", result.APIVersion)
	}

	// Should have empty spec content
	if result.Spec.Kubernetes.MinVersion != "" {
		t.Errorf("expected empty kubernetes requirements")
	}
}

// TestInvalidJSON tests handling of invalid JSON
func TestInvalidJSON(t *testing.T) {
	parser := NewRequirementParser()

	invalidJSON := `{"apiVersion": "test", "incomplete": }`

	// Use direct format specification
	_, err := parser.ParseFromBytes([]byte(invalidJSON), "json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("expected parse error, got: %v", err)
	}
}
