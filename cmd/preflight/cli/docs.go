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

// PreflightDoc supports both legacy (requirements) and beta3 (spec.analyzers)
type PreflightDoc struct {
	APIVersion string                 `yaml:"apiVersion"`
	Kind       string                 `yaml:"kind"`
	Metadata   map[string]interface{} `yaml:"metadata"`
	Spec       struct {
		Analyzers []map[string]interface{} `yaml:"analyzers"`
	} `yaml:"spec"`
	// Legacy (pre-beta3 drafts)
	Requirements []Requirement `yaml:"requirements"`
}

type Requirement struct {
	Name      string                   `yaml:"name"`
	DocString string                   `yaml:"docString"`
	Checks    []map[string]interface{} `yaml:"checks,omitempty"`
}

func extractDocs(templateFiles []string, valuesFiles []string, setValues []string, outputFile string) error {
	// Prepare the values map (merge all files, then apply sets)
	values := make(map[string]interface{})

	for _, valuesFile := range valuesFiles {
		fileValues, err := loadValuesFile(valuesFile)
		if err != nil {
			return errors.Wrapf(err, "failed to load values file %s", valuesFile)
		}
		values = mergeMaps(values, fileValues)
	}

	// Normalize maps for Helm set merging
	values = normalizeStringMaps(values)

	for _, setValue := range setValues {
		if err := applySetValue(values, setValue); err != nil {
			return errors.Wrapf(err, "failed to apply set value: %s", setValue)
		}
	}

	var combinedDocs strings.Builder

	for _, templateFile := range templateFiles {
		templateContent, err := os.ReadFile(templateFile)
		if err != nil {
			return errors.Wrapf(err, "failed to read template file %s", templateFile)
		}

		useHelm := shouldUseHelmEngine(string(templateContent))
		var rendered string
		if useHelm {
			rendered, err = preflight.RenderWithHelmTemplate(string(templateContent), values)
			if err != nil {
				execValues := legacyContext(values)
				rendered, err = renderTemplate(string(templateContent), execValues)
				if err != nil {
					return errors.Wrap(err, "failed to render template (helm fallback also failed)")
				}
			}
		} else {
			execValues := legacyContext(values)
			rendered, err = renderTemplate(string(templateContent), execValues)
			if err != nil {
				return errors.Wrap(err, "failed to render template")
			}
		}

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

func shouldUseHelmEngine(content string) bool {
	return strings.Contains(content, ".Values")
}

func legacyContext(values map[string]interface{}) map[string]interface{} {
	ctx := make(map[string]interface{}, len(values)+1)
	for k, v := range values {
		ctx[k] = v
	}
	ctx["Values"] = values
	return ctx
}

func normalizeStringMaps(v interface{}) map[string]interface{} {
	// Avoid unsafe type assertion; normalizeMap may return non-map types.
	if v == nil {
		return map[string]interface{}{}
	}
	normalized := normalizeMap(v)
	if m, ok := normalized.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func normalizeMap(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{}, len(t))
		for k, val := range t {
			m[k] = normalizeMap(val)
		}
		return m
	case map[interface{}]interface{}:
		m := make(map[string]interface{}, len(t))
		for k, val := range t {
			key := fmt.Sprintf("%v", k)
			m[key] = normalizeMap(val)
		}
		return m
	case []interface{}:
		a := make([]interface{}, len(t))
		for i, val := range t {
			a[i] = normalizeMap(val)
		}
		return a
	default:
		return v
	}
}

func extractDocStrings(yamlContent string) (string, error) {
	var preflightDoc PreflightDoc
	if err := yaml.Unmarshal([]byte(yamlContent), &preflightDoc); err != nil {
		return "", errors.Wrap(err, "failed to parse YAML")
	}

	var docs strings.Builder
	first := true

	// Prefer beta3 analyzers docStrings
	if len(preflightDoc.Spec.Analyzers) > 0 {
		for _, analyzer := range preflightDoc.Spec.Analyzers {
			if raw, ok := analyzer["docString"]; ok {
				text, _ := raw.(string)
				text = strings.TrimSpace(text)
				if text == "" {
					continue
				}
				if !first {
					docs.WriteString("\n\n")
				}
				first = false
				writeMarkdownSection(&docs, text, "")
			}
		}
		return docs.String(), nil
	}

	// Fallback: legacy requirements with docString
	for _, req := range preflightDoc.Requirements {
		if strings.TrimSpace(req.DocString) == "" {
			continue
		}
		if !first {
			docs.WriteString("\n\n")
		}
		first = false
		writeMarkdownSection(&docs, req.DocString, req.Name)
	}

	return docs.String(), nil
}

// writeMarkdownSection prints a heading from Title: or name, then the rest
func writeMarkdownSection(b *strings.Builder, docString string, fallbackName string) {
	lines := strings.Split(docString, "\n")
	title := strings.TrimSpace(fallbackName)
	contentStart := 0
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "Title:") {
			parts := strings.SplitN(trim, ":", 2)
			if len(parts) == 2 {
				t := strings.TrimSpace(parts[1])
				if t != "" {
					title = t
				}
			}
			contentStart = i + 1
			break
		}
	}
	if title != "" {
		b.WriteString("### ")
		b.WriteString(title)
		b.WriteString("\n\n")
	}
	remaining := strings.Join(lines[contentStart:], "\n")
	remaining = strings.TrimSpace(remaining)
	if remaining != "" {
		b.WriteString(remaining)
		b.WriteString("\n")
	}
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

// applySetValue applies a single --set value to the values map (Helm semantics)
func applySetValue(values map[string]interface{}, setValue string) error {
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
	if _, ok := m[keys[0]]; !ok {
		m[keys[0]] = make(map[string]interface{})
	}
	if nextMap, ok := m[keys[0]].(map[string]interface{}); ok {
		setNestedValue(nextMap, keys[1:], value)
	} else {
		m[keys[0]] = make(map[string]interface{})
		setNestedValue(m[keys[0]].(map[string]interface{}), keys[1:], value)
	}
}

func mergeMaps(base, overlay map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		if baseVal, exists := result[k]; exists {
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

func renderTemplate(templateContent string, values map[string]interface{}) (string, error) {
	tmpl := template.New("preflight").Funcs(sprig.FuncMap())
	tmpl, err := tmpl.Parse(templateContent)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse template")
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, values); err != nil {
		return "", errors.Wrap(err, "failed to execute template")
	}
	result := cleanRenderedYAML(buf.String())
	return result, nil
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
