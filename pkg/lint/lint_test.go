package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintMultipleFiles(t *testing.T) {
	// Get the project root by going up from pkg/lint
	projectRoot := filepath.Join("..", "..")
	testDir := filepath.Join(projectRoot, "examples", "test-error-messages")

	tests := []struct {
		name          string
		files         []string
		expectErrors  map[string][]string // filename -> expected error substrings
		expectWarnings map[string][]string // filename -> expected warning substrings
		expectPass    map[string]bool     // filename -> should pass without errors
	}{
		{
			name: "valid v1beta3 with templates",
			files: []string{
				"helm-builtins-v1beta3.yaml",
			},
			expectErrors: map[string][]string{},
			expectWarnings: map[string][]string{
				"helm-builtins-v1beta3.yaml": {
					"Template values that must be provided at runtime: minVersion",
				},
			},
			expectPass: map[string]bool{
				"helm-builtins-v1beta3.yaml": false, // has warnings
			},
		},
		{
			name: "invalid collectors and analyzers",
			files: []string{
				"invalid-collectors-analyzers.yaml",
			},
			expectErrors: map[string][]string{
				"invalid-collectors-analyzers.yaml": {
					// The linter may stop early due to structural issues
					// At minimum, it should catch the hostCollectors type error
					"Expected 'hostCollectors' to be a list",
				},
			},
			expectPass: map[string]bool{
				"invalid-collectors-analyzers.yaml": false,
			},
		},
		{
			name: "missing required fields",
			files: []string{
				"missing-apiversion-v1beta3.yaml",
				"missing-metadata-v1beta3.yaml",
				"no-analyzers-v1beta3.yaml",
			},
			expectErrors: map[string][]string{
				"missing-apiversion-v1beta3.yaml": {
					"Missing or empty 'apiVersion' field",
				},
				"missing-metadata-v1beta3.yaml": {
					"Missing 'metadata' section",
				},
				"no-analyzers-v1beta3.yaml": {
					"Preflight spec must contain 'analyzers'",
				},
			},
			expectPass: map[string]bool{
				"missing-apiversion-v1beta3.yaml": false,
				"missing-metadata-v1beta3.yaml":   false,
				"no-analyzers-v1beta3.yaml":       false,
			},
		},
		{
			name: "v1beta2 file (valid but with docString warning)",
			files: []string{
				"wrong-apiversion-v1beta3.yaml", // Actually has v1beta2 which is valid
			},
			expectErrors: map[string][]string{},
			expectWarnings: map[string][]string{
				"wrong-apiversion-v1beta3.yaml": {
					"Some analyzers are missing docString",
				},
			},
			expectPass: map[string]bool{
				"wrong-apiversion-v1beta3.yaml": true, // No errors, just warnings
			},
		},
		{
			name: "support bundle specs",
			files: []string{
				"support-bundle-no-collectors-v1beta3.yaml",
				"support-bundle-valid-v1beta3.yaml",
			},
			expectErrors: map[string][]string{
				"support-bundle-no-collectors-v1beta3.yaml": {
					"SupportBundle spec must contain 'collectors' or 'hostCollectors'",
				},
			},
			expectPass: map[string]bool{
				"support-bundle-no-collectors-v1beta3.yaml": false,
				"support-bundle-valid-v1beta3.yaml":         true,
			},
		},
		{
			name: "multiple files with mixed validity",
			files: []string{
				"support-bundle-valid-v1beta3.yaml",
				"missing-metadata-v1beta3.yaml",
				"wrong-apiversion-v1beta3.yaml",
			},
			expectErrors: map[string][]string{
				"missing-metadata-v1beta3.yaml": {
					"Missing 'metadata' section",
				},
			},
			expectWarnings: map[string][]string{
				"wrong-apiversion-v1beta3.yaml": {
					"Some analyzers are missing docString",
				},
			},
			expectPass: map[string]bool{
				"support-bundle-valid-v1beta3.yaml": true,
				"missing-metadata-v1beta3.yaml":     false,
				"wrong-apiversion-v1beta3.yaml":     true, // No errors, just warnings
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build full file paths
			filePaths := make([]string, len(tt.files))
			for i, f := range tt.files {
				filePaths[i] = filepath.Join(testDir, f)
				// Check file exists
				if _, err := os.Stat(filePaths[i]); os.IsNotExist(err) {
					t.Skipf("Test file %s does not exist, skipping", filePaths[i])
				}
			}

			// Run linter
			opts := LintOptions{
				FilePaths: filePaths,
				Fix:       false,
				Format:    "text",
			}

			results, err := LintFiles(opts)
			if err != nil {
				t.Fatalf("LintFiles failed: %v", err)
			}

			// Verify we got results for all files
			if len(results) != len(filePaths) {
				t.Errorf("Expected %d results, got %d", len(filePaths), len(results))
			}

			// Check each result
			for _, result := range results {
				filename := filepath.Base(result.FilePath)

				// Check expected errors
				if expectedErrors, ok := tt.expectErrors[filename]; ok {
					if len(expectedErrors) > 0 && len(result.Errors) == 0 {
						t.Errorf("File %s: expected errors but got none", filename)
					}
					for _, expectedErr := range expectedErrors {
						found := false
						for _, err := range result.Errors {
							if strings.Contains(err.Message, expectedErr) {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("File %s: expected error containing '%s' but not found in errors: %v",
								filename, expectedErr, getErrorMessages(result.Errors))
						}
					}
				}

				// Check expected warnings
				if expectedWarnings, ok := tt.expectWarnings[filename]; ok {
					for _, expectedWarn := range expectedWarnings {
						found := false
						for _, warn := range result.Warnings {
							if strings.Contains(warn.Message, expectedWarn) {
								found = true
								break
							}
						}
						if !found {
							t.Errorf("File %s: expected warning containing '%s' but not found in warnings: %v",
								filename, expectedWarn, getWarningMessages(result.Warnings))
						}
					}
				}

				// Check if should pass
				if shouldPass, ok := tt.expectPass[filename]; ok {
					hasNoErrors := len(result.Errors) == 0
					if shouldPass && !hasNoErrors {
						t.Errorf("File %s: expected to pass but has errors: %v",
							filename, getErrorMessages(result.Errors))
					} else if !shouldPass && hasNoErrors && len(tt.expectErrors[filename]) > 0 {
						t.Errorf("File %s: expected to fail but passed", filename)
					}
				}
			}
		})
	}
}

func TestLintWithFix(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "lint-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name        string
		content     string
		expectFix   bool
		fixedContent string // substring that should appear after fix
	}{
		{
			name: "fix v1beta2 with templates to v1beta3",
			content: `apiVersion: troubleshoot.sh/v1beta2
kind: Preflight
metadata:
  name: test-{{ .Values.name }}
spec:
  analyzers:
    - clusterVersion:
        outcomes:
          - pass:
              when: '>= 1.19.0'
              message: OK`,
			expectFix:   true,
			fixedContent: "apiVersion: troubleshoot.sh/v1beta3",
		},
		{
			name: "fix missing leading dot in template",
			content: `apiVersion: troubleshoot.sh/v1beta3
kind: Preflight
metadata:
  name: test-{{ Values.name }}
spec:
  analyzers:
    - clusterVersion:
        outcomes:
          - pass:
              when: '>= 1.19.0'
              message: OK`,
			expectFix:   true,
			fixedContent: "{{ .Values.name }}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write test content to temp file
			testFile := filepath.Join(tmpDir, tt.name+".yaml")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			// Run linter with fix enabled
			opts := LintOptions{
				FilePaths: []string{testFile},
				Fix:       true,
				Format:    "text",
			}

			results, err := LintFiles(opts)
			if err != nil {
				t.Fatalf("LintFiles failed: %v", err)
			}

			if len(results) != 1 {
				t.Fatalf("Expected 1 result, got %d", len(results))
			}

			// Read the potentially fixed content
			fixedBytes, err := os.ReadFile(testFile)
			if err != nil {
				t.Fatalf("Failed to read fixed file: %v", err)
			}
			fixedContent := string(fixedBytes)

			// Check if fix was applied
			if tt.expectFix {
				if !strings.Contains(fixedContent, tt.fixedContent) {
					t.Errorf("Expected fixed content to contain '%s', but got:\n%s",
						tt.fixedContent, fixedContent)
				}
			}
		})
	}
}

func TestHasErrors(t *testing.T) {
	tests := []struct {
		name     string
		results  []LintResult
		expected bool
	}{
		{
			name: "no errors",
			results: []LintResult{
				{
					FilePath: "test1.yaml",
					Errors:   []LintError{},
					Warnings: []LintWarning{{Message: "warning"}},
				},
				{
					FilePath: "test2.yaml",
					Errors:   []LintError{},
				},
			},
			expected: false,
		},
		{
			name: "has errors",
			results: []LintResult{
				{
					FilePath: "test1.yaml",
					Errors:   []LintError{{Message: "error"}},
				},
				{
					FilePath: "test2.yaml",
					Errors:   []LintError{},
				},
			},
			expected: true,
		},
		{
			name:     "empty results",
			results:  []LintResult{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasErrors(tt.results)
			if result != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestFormatResults(t *testing.T) {
	results := []LintResult{
		{
			FilePath: "test.yaml",
			Errors: []LintError{
				{Line: 5, Message: "Missing field", Field: "spec.analyzers"},
			},
			Warnings: []LintWarning{
				{Line: 10, Message: "Consider adding docString", Field: "spec.analyzers[0]"},
			},
		},
	}

	t.Run("text format", func(t *testing.T) {
		output := FormatResults(results, "text")

		// Check for key components in text output
		if !strings.Contains(output, "test.yaml") {
			t.Error("Text output missing file path")
		}
		if !strings.Contains(output, "Error (line 5)") {
			t.Error("Text output missing error with line number")
		}
		if !strings.Contains(output, "Warning (line 10)") {
			t.Error("Text output missing warning with line number")
		}
		if !strings.Contains(output, "Summary:") {
			t.Error("Text output missing summary")
		}
	})

	t.Run("json format", func(t *testing.T) {
		output := FormatResults(results, "json")

		// Check for key JSON components
		if !strings.Contains(output, `"filePath"`) {
			t.Error("JSON output missing filePath field")
		}
		if !strings.Contains(output, `"errors"`) {
			t.Error("JSON output missing errors field")
		}
		if !strings.Contains(output, `"warnings"`) {
			t.Error("JSON output missing warnings field")
		}
		if !strings.Contains(output, `"line": 5`) {
			t.Error("JSON output missing line number")
		}
	})
}

// Helper functions
func getErrorMessages(errors []LintError) []string {
	messages := make([]string, len(errors))
	for i, err := range errors {
		messages[i] = err.Message
	}
	return messages
}

func getWarningMessages(warnings []LintWarning) []string {
	messages := make([]string, len(warnings))
	for i, warn := range warnings {
		messages[i] = warn.Message
	}
	return messages
}