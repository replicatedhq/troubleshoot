package preflight

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// RunTemplate processes a templated preflight spec file with provided values
func RunTemplate(templateFile string, valuesFiles []string, setValues []string, outputFile string) error {
	// Read the template file
	templateContent, err := os.ReadFile(templateFile)
	if err != nil {
		return errors.Wrapf(err, "failed to read template file %s", templateFile)
	}

	// Prepare the values map
	values := make(map[string]interface{})

	// Load values from files if provided
	for _, valuesFile := range valuesFiles {
		if valuesFile == "" {
			continue
		}
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

	// Output the result
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(rendered), 0644); err != nil {
			return errors.Wrapf(err, "failed to write output file %s", outputFile)
		}
		fmt.Printf("Template rendered successfully to %s\n", outputFile)
	} else {
		fmt.Print(rendered)
	}

	return nil
}

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

	// Ensure intermediate maps exist or preserve existing ones
	if existing, ok := m[keys[0]]; !ok {
		m[keys[0]] = make(map[string]interface{})
	} else {
		// Check if it's any kind of map (could be map[interface{}]interface{} from YAML)
		switch v := existing.(type) {
		case map[string]interface{}:
			// Already the right type, nothing to do
		case map[interface{}]interface{}:
			// Convert map[interface{}]interface{} to map[string]interface{}
			converted := make(map[string]interface{})
			for k, val := range v {
				if strKey, ok := k.(string); ok {
					converted[strKey] = val
				}
			}
			m[keys[0]] = converted
		default:
			// If it exists but is not a map, replace it with a map
			m[keys[0]] = make(map[string]interface{})
		}
	}

	// Now we know m[keys[0]] is a map
	setNestedValue(m[keys[0]].(map[string]interface{}), keys[1:], value)
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
