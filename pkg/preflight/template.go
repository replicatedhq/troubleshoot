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
	"helm.sh/helm/v3/pkg/strvals"
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

	// Apply --set values (Helm semantics)
	for _, setValue := range setValues {
		if err := applySetValue(values, setValue); err != nil {
			return errors.Wrapf(err, "failed to apply set value: %s", setValue)
		}
	}

	// Choose engine based on apiVersion
	apiVersion := detectAPIVersion(string(templateContent))
	var rendered string
	if strings.HasSuffix(apiVersion, "/v1beta3") || apiVersion == "v1beta3" {
		// Helm for v1beta3
		rendered, err = RenderWithHelmTemplate(string(templateContent), values)
		if err != nil {
			return errors.Wrap(err, "failed to render template using Helm")
		}
	} else {
		// Legacy renderer for older API versions
		rendered, err = renderLegacyTemplate(string(templateContent), values)
		if err != nil {
			return errors.Wrap(err, "failed to render template using legacy renderer")
		}
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

// applySetValue applies a single --set value to the values map using Helm semantics
func applySetValue(values map[string]interface{}, setValue string) error {
	// Normalize optional "Values." prefix so both --set test.enabled and --set Values.test.enabled work
	if idx := strings.Index(setValue, "="); idx > 0 {
		key := setValue[:idx]
		val := setValue[idx+1:]
		if strings.HasPrefix(key, "Values.") {
			key = strings.TrimPrefix(key, "Values.")
			setValue = key + "=" + val
		}
	}
	if err := strvals.ParseInto(setValue, values); err != nil {
		return fmt.Errorf("parsing --set: %w", err)
	}
	return nil
}

// detectAPIVersion attempts to read apiVersion from the raw YAML header
func detectAPIVersion(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		l := strings.TrimSpace(line)
		if strings.HasPrefix(l, "apiVersion:") {
			parts := strings.SplitN(l, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
		if strings.HasPrefix(l, "kind:") || strings.HasPrefix(l, "metadata:") {
			break
		}
	}
	return ""
}

// renderLegacyTemplate uses Go text/template with Sprig and passes values at root
func renderLegacyTemplate(templateContent string, values map[string]interface{}) (string, error) {
	tmpl := template.New("preflight").Funcs(sprig.FuncMap())
	tmpl, err := tmpl.Parse(templateContent)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse template")
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, values); err != nil {
		return "", errors.Wrap(err, "failed to execute template")
	}
	return cleanRenderedYAML(buf.String()), nil
}

func cleanRenderedYAML(content string) string {
	lines := strings.Split(content, "\n")
	var cleaned []string
	var lastWasEmpty bool
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
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
	for len(cleaned) > 0 && cleaned[len(cleaned)-1] == "" {
		cleaned = cleaned[:len(cleaned)-1]
	}
	return strings.Join(cleaned, "\n") + "\n"
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
