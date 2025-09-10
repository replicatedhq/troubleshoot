package cli

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/preflight"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
	"helm.sh/helm/v3/pkg/strvals"
)

func DocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs [preflight-file...]",
		Short: "Extract and display documentation from a preflight spec",
		Long: `Extract all docString fields from enabled requirements in one or more preflight YAML files.
This command processes templated preflight specs, evaluates conditionals, and outputs
only the documentation for requirements that would be included based on the provided values.

Examples:
  # Extract docs with default values
  preflight docs ml-platform-preflight.yaml

  # Extract docs from multiple specs with values from files
  preflight docs spec1.yaml spec2.yaml --values base-values.yaml --values prod-values.yaml

  # Extract docs with inline values
  preflight docs ml-platform-preflight.yaml --set jupyter.enabled=true --set monitoring.enabled=false

  # Extract docs and save to file
  preflight docs ml-platform-preflight.yaml --output requirements.md`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			v := viper.GetViper()

			templateFiles := args
			valuesFiles := v.GetStringSlice("values")
			outputFile := v.GetString("output")
			setValues := v.GetStringSlice("set")

			return extractDocs(templateFiles, valuesFiles, setValues, outputFile)
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
	Name      string                   `yaml:"name"`
	DocString string                   `yaml:"docString"`
	Checks    []map[string]interface{} `yaml:"checks,omitempty"`
}

func extractDocs(templateFiles []string, valuesFiles []string, setValues []string, outputFile string) error {
	// Prepare the values map (merge all files, then apply sets)
	values := make(map[string]interface{})

	// Load values from files if provided
	for _, valuesFile := range valuesFiles {
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

	// Accumulate docs across all provided templates
	var combinedDocs strings.Builder

	for _, templateFile := range templateFiles {
		// Read the template file
		templateContent, err := os.ReadFile(templateFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read template file %s", templateFile)
		}

		// Decide rendering engine. Try Helm when .Values is referenced; if it fails, fall back to legacy.
		useHelm := shouldUseHelmEngine(string(templateContent))

		var rendered string
		if useHelm {
			rendered, err = preflight.RenderWithHelmTemplate(string(templateContent), values)
			if err != nil {
				// Fall back to legacy with dual-context (root + Values) for mixed templates
				execValues := legacyContext(values)
				rendered, err = renderTemplate(string(templateContent), execValues)
				if err != nil {
					return errors.Wrap(err, "failed to render template (helm fallback also failed)")
				}
			}
		} else {
			// Legacy with dual-context (root + Values)
			execValues := legacyContext(values)
			rendered, err = renderTemplate(string(templateContent), execValues)
			if err != nil {
				return errors.Wrap(err, "failed to render template")
			}
		}

		// Parse the rendered YAML to extract docStrings
		docs, err := extractDocStrings(rendered)
		if err != nil {
			return errors.Wrap(err, "failed to extract documentation")
		}

		if strings.TrimSpace(docs) != "" {
			if combinedDocs.Len() > 0 {
				combinedDocs.WriteString("\n\n")
			}
			combinedDocs.WriteString(docs)
		}
	}

	// Output the result
	if outputFile != "" {
		if err := os.WriteFile(outputFile, []byte(combinedDocs.String()), 0644); err != nil {
			return errors.Wrapf(err, "failed to write output file %s", outputFile)
		}
		fmt.Printf("Documentation extracted successfully to %s\n", outputFile)
	} else {
		fmt.Print(combinedDocs.String())
	}

	return nil
}

// shouldUseHelmEngine returns true if the template appears to use Helm's .Values
func shouldUseHelmEngine(content string) bool {
	return strings.Contains(content, ".Values")
}

// legacyContext returns a map that supports both .foo and .Values.foo lookups
func legacyContext(values map[string]interface{}) map[string]interface{} {
	ctx := make(map[string]interface{}, len(values)+1)
	for k, v := range values {
		ctx[k] = v
	}
	ctx["Values"] = values
	return ctx
}

func extractDocStrings(yamlContent string) (string, error) {
	// Parse the YAML
	var preflightDoc PreflightDoc
	if err := yaml.Unmarshal([]byte(yamlContent), &preflightDoc); err != nil {
		return "", errors.Wrap(err, "failed to parse YAML")
	}

	// Extract and combine all docStrings
	var docs strings.Builder

	for i, req := range preflightDoc.Requirements {
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

// applySetValue applies a single --set value to the values map (Helm semantics)
func applySetValue(values map[string]interface{}, setValue string) error {
	// Normalize optional "Values." prefix
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
