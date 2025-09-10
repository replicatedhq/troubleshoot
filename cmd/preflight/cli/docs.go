package cli

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

func DocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs [preflight-file]",
		Short: "Extract and display documentation from a preflight spec",
		Long: `Extract all docString fields from enabled requirements in a preflight YAML file.
This command processes templated preflight specs, evaluates conditionals, and outputs
only the documentation for requirements that would be included based on the provided values.

Examples:
  # Extract docs with default values
  preflight docs ml-platform-preflight.yaml

  # Extract docs with values from files
  preflight docs ml-platform-preflight.yaml --values base-values.yaml --values prod-values.yaml

  # Extract docs with inline values
  preflight docs ml-platform-preflight.yaml --set jupyter.enabled=true --set monitoring.enabled=false

  # Extract docs and save to file
  preflight docs ml-platform-preflight.yaml --output requirements.md`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			templateFile := args[0]
			valuesFiles := v.GetStringSlice("values")
			outputFile := v.GetString("output")
			setValues := v.GetStringSlice("set")

			return extractDocs(templateFile, valuesFiles, setValues, outputFile)
		},
	}

	cmd.Flags().StringSlice("values", []string{}, "Path to YAML files containing template values (can be used multiple times)")
	cmd.Flags().StringSlice("set", []string{}, "Set template values on the command line (can be used multiple times)")
	cmd.Flags().StringP("output", "o", "", "Output file (default: stdout)")

	// Bind flags to viper
	viper.BindPFlag("values", cmd.Flags().Lookup("values"))
	viper.BindPFlag("set", cmd.Flags().Lookup("set"))
	viper.BindPFlag("output", cmd.Flags().Lookup("output"))

	return cmd
}

// PreflightDoc represents a preflight document with requirements
type PreflightDoc struct {
	APIVersion   string                 `yaml:"apiVersion"`
	Kind         string                 `yaml:"kind"`
	Metadata     map[string]interface{} `yaml:"metadata"`
	Requirements []Requirement          `yaml:"requirements"`
}

// Requirement represents a requirement with its documentation
type Requirement struct {
	Name      string                 `yaml:"name"`
	DocString string                 `yaml:"docString"`
	Checks    []map[string]interface{} `yaml:"checks,omitempty"`
}

func extractDocs(templateFile string, valuesFiles []string, setValues []string, outputFile string) error {
	// Read the template file
	templateContent, err := os.ReadFile(templateFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read template file %s", templateFile)
	}

	// Prepare the values map
	values := make(map[string]interface{})

	// Load values from files if provided
	for _, valuesFile := range valuesFiles {
		fileValues, err := loadValuesFile(valuesFile)
		if err != nil {
			return errors.Wrapf(err, "failed to load values file %s", valuesFile)
		}
		values = mergeMaps(values, fileValues)
	}

	// Apply --set values
	for _, setValue := range setValues {
		if err := applySetValue(values, setValue); err != nil {
			return errors.Wrapf(err, "failed to apply set value: %s", setValue)
		}
	}

	// Process the template
	rendered, err := renderTemplate(string(templateContent), values)
	if err != nil {
		return errors.Wrap(err, "failed to render template")
	}

	// Parse the rendered YAML to extract docStrings
	docs, err := extractDocStrings(rendered)
	if err != nil {
		return errors.Wrap(err, "failed to extract documentation")
	}

	// Output the result
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(docs), 0644); err != nil {
			return errors.Wrapf(err, "failed to write output file %s", outputFile)
		}
		fmt.Printf("Documentation extracted successfully to %s\n", outputFile)
	} else {
		fmt.Print(docs)
	}

	return nil
}

func extractDocStrings(yamlContent string) (string, error) {
	// Parse the YAML
	var preflight PreflightDoc
	if err := yaml.Unmarshal([]byte(yamlContent), &preflight); err != nil {
		return "", errors.Wrap(err, "failed to parse YAML")
	}

	// Extract and combine all docStrings
	var docs strings.Builder
	
	for i, req := range preflight.Requirements {
		if req.DocString != "" {
			// Add separator between requirements (except for the first one)
			if i > 0 {
				docs.WriteString("\n" + strings.Repeat("=", 80) + "\n\n")
			}
			
			// Clean up the docString (remove leading/trailing whitespace)
			cleanedDoc := strings.TrimSpace(req.DocString)
			docs.WriteString(cleanedDoc)
			docs.WriteString("\n")
		}
	}

	return docs.String(), nil
}

// The following functions are reused from template.go
// In a real implementation, these would be exported from the preflight package

// loadValuesFile loads values from a YAML file
func loadValuesFile(filename string) (map[string]interface{}, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var values map[string]interface{}
	if err := yaml.Unmarshal(data, &values); err != nil {
		return nil, errors.Wrap(err, "failed to parse values file as YAML")
	}

	return values, nil
}

// applySetValue applies a single --set value to the values map
func applySetValue(values map[string]interface{}, setValue string) error {
	parts := strings.SplitN(setValue, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid set value format: %s (expected key=value)", setValue)
	}

	key := parts[0]
	value := parts[1]

	// Parse the value to appropriate type
	var parsedValue interface{}
	
	// Try to parse as boolean
	if value == "true" {
		parsedValue = true
	} else if value == "false" {
		parsedValue = false
	} else if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		// Try to parse as array
		if err := yaml.Unmarshal([]byte(value), &parsedValue); err != nil {
			parsedValue = value // Fall back to string
		}
	} else {
		// Try to parse as number or keep as string
		var numValue float64
		if _, err := fmt.Sscanf(value, "%f", &numValue); err == nil {
			// Check if it's an integer
			if numValue == float64(int(numValue)) {
				parsedValue = int(numValue)
			} else {
				parsedValue = numValue
			}
		} else {
			parsedValue = value
		}
	}

	// Handle nested keys (e.g., postgres.enabled)
	keys := strings.Split(key, ".")
	setNestedValue(values, keys, parsedValue)

	return nil
}

// setNestedValue sets a value in a nested map structure
func setNestedValue(m map[string]interface{}, keys []string, value interface{}) {
	if len(keys) == 0 {
		return
	}

	if len(keys) == 1 {
		m[keys[0]] = value
		return
	}

	// Ensure intermediate maps exist
	if _, ok := m[keys[0]]; !ok {
		m[keys[0]] = make(map[string]interface{})
	}

	if nextMap, ok := m[keys[0]].(map[string]interface{}); ok {
		setNestedValue(nextMap, keys[1:], value)
	} else {
		// If the intermediate value is not a map, replace it
		m[keys[0]] = make(map[string]interface{})
		setNestedValue(m[keys[0]].(map[string]interface{}), keys[1:], value)
	}
}

// mergeMaps recursively merges two maps
func mergeMaps(base, overlay map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy base map
	for k, v := range base {
		result[k] = v
	}

	// Overlay values
	for k, v := range overlay {
		if baseVal, exists := result[k]; exists {
			// If both are maps, merge recursively
			if baseMap, ok := baseVal.(map[string]interface{}); ok {
				if overlayMap, ok := v.(map[string]interface{}); ok {
					result[k] = mergeMaps(baseMap, overlayMap)
					continue
				}
			}
		}
		result[k] = v
	}

	return result
}

// renderTemplate processes the template with the provided values
func renderTemplate(templateContent string, values map[string]interface{}) (string, error) {
	// Create template with Sprig functions
	tmpl := template.New("preflight").Funcs(sprig.FuncMap())

	// Parse the template
	tmpl, err := tmpl.Parse(templateContent)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse template")
	}

	// Execute the template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, values); err != nil {
		return "", errors.Wrap(err, "failed to execute template")
	}

	// Post-process to remove empty lines and clean up whitespace
	result := cleanRenderedYAML(buf.String())

	return result, nil
}

// cleanRenderedYAML removes empty lines and cleans up the rendered YAML
func cleanRenderedYAML(content string) string {
	lines := strings.Split(content, "\n")
	var cleaned []string
	var lastWasEmpty bool

	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		
		// Skip multiple consecutive empty lines
		if trimmed == "" {
			if !lastWasEmpty {
				cleaned = append(cleaned, "")
				lastWasEmpty = true
			}
		} else {
			cleaned = append(cleaned, trimmed)
			lastWasEmpty = false
		}
	}

	// Remove trailing empty lines
	for len(cleaned) > 0 && cleaned[len(cleaned)-1] == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}

	return strings.Join(cleaned, "\n") + "\n"
}