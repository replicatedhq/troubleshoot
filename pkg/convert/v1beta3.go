package convert

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// V1Beta2ToV1Beta3Result holds the conversion results
type V1Beta2ToV1Beta3Result struct {
	TemplatedSpec string            `yaml:"-"`
	ValuesFile    string            `yaml:"-"`
	Values        map[string]interface{} `yaml:"-"`
}

// ConvertToV1Beta3 converts a v1beta2 preflight spec to v1beta3 format with templating
func ConvertToV1Beta3(doc []byte) (*V1Beta2ToV1Beta3Result, error) {
	var parsed map[string]interface{}
	err := yaml.Unmarshal(doc, &parsed)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal yaml")
	}

	// Check if it's already v1beta3
	if apiVersion, ok := parsed["apiVersion"]; ok && apiVersion == "troubleshoot.sh/v1beta3" {
		return nil, errors.New("document is already v1beta3")
	}

	// Check if it's v1beta2
	if apiVersion, ok := parsed["apiVersion"]; !ok || apiVersion != "troubleshoot.sh/v1beta2" {
		return nil, errors.Errorf("unsupported apiVersion: %v", apiVersion)
	}

	// Check if it's a preflight spec
	if kind, ok := parsed["kind"]; !ok || kind != "Preflight" {
		return nil, errors.Errorf("unsupported kind: %v", kind)
	}

	// Extract values and create templated spec
	values := make(map[string]interface{})
	converter := &v1beta3Converter{
		values: values,
		spec:   parsed,
	}

	templatedSpec, err := converter.convert()
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert spec")
	}

	// Marshal values
	valuesBytes, err := yaml.Marshal(values)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal values")
	}

	return &V1Beta2ToV1Beta3Result{
		TemplatedSpec: templatedSpec,
		ValuesFile:    string(valuesBytes),
		Values:        values,
	}, nil
}

type v1beta3Converter struct {
	values map[string]interface{}
	spec   map[string]interface{}
}

func (c *v1beta3Converter) convert() (string, error) {
	// Initialize values structure
	c.initializeValues()

	// Get metadata name
	metadataName := "converted-from-v1beta2"
	if metadata, ok := c.spec["metadata"].(map[interface{}]interface{}); ok {
		if name, ok := metadata["name"].(string); ok {
			metadataName = name
		}
	}

	// Process spec
	var analyzers []interface{}
	if spec, ok := c.spec["spec"].(map[interface{}]interface{}); ok {
		if analyzersList, ok := spec["analyzers"].([]interface{}); ok {
			convertedAnalyzers, err := c.convertAnalyzers(analyzersList)
			if err != nil {
				return "", errors.Wrap(err, "failed to convert analyzers")
			}
			analyzers = convertedAnalyzers
		}
	}

	// Build the templated spec string
	var buf bytes.Buffer

	// Header
	buf.WriteString("apiVersion: troubleshoot.sh/v1beta3\n")
	buf.WriteString("kind: Preflight\n")
	buf.WriteString("metadata:\n")
	buf.WriteString(fmt.Sprintf("  name: %s\n", metadataName))
	buf.WriteString("spec:\n")
	buf.WriteString("  analyzers:\n")

	// Add each analyzer
	for _, analyzer := range analyzers {
		if analyzerStr, ok := analyzer.(string); ok {
			// This is already a templated string
			buf.WriteString("    ")
			buf.WriteString(strings.ReplaceAll(analyzerStr, "\n", "\n    "))
			buf.WriteString("\n")
		} else {
			// Convert to YAML and add as-is
			analyzerBytes, err := yaml.Marshal(analyzer)
			if err != nil {
				return "", errors.Wrap(err, "failed to marshal analyzer")
			}
			lines := strings.Split(string(analyzerBytes), "\n")
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					buf.WriteString("    - ")
					buf.WriteString(line)
					buf.WriteString("\n")
				}
			}
		}
	}

	return buf.String(), nil
}

func (c *v1beta3Converter) initializeValues() {
	c.values["kubernetes"] = map[string]interface{}{
		"enabled":             false,
		"minVersion":          "1.20.0",
		"recommendedVersion":  "1.22.0",
	}

	c.values["storage"] = map[string]interface{}{
		"enabled":   false,
		"className": "default",
	}

	c.values["cluster"] = map[string]interface{}{
		"minNodes":         3,
		"recommendedNodes": 5,
		"minCPU":           4,
	}

	c.values["node"] = map[string]interface{}{
		"minMemoryGi":         8,
		"recommendedMemoryGi": 32,
		"minEphemeralGi":      40,
		"recommendedEphemeralGi": 100,
	}

	c.values["ingress"] = map[string]interface{}{
		"enabled": false,
		"type":    "Contour",
	}

	c.values["runtime"] = map[string]interface{}{
		"enabled": false,
	}

	c.values["distribution"] = map[string]interface{}{
		"enabled": false,
	}

	c.values["nodeChecks"] = map[string]interface{}{
		"enabled": false,
		"count": map[string]interface{}{
			"enabled": false,
		},
		"cpu": map[string]interface{}{
			"enabled": false,
		},
		"memory": map[string]interface{}{
			"enabled": false,
		},
		"ephemeral": map[string]interface{}{
			"enabled": false,
		},
	}
}

func (c *v1beta3Converter) convertAnalyzers(analyzers []interface{}) ([]interface{}, error) {
	var result []interface{}

	for _, analyzer := range analyzers {
		if analyzerMap, ok := analyzer.(map[interface{}]interface{}); ok {
			converted, err := c.convertAnalyzer(analyzerMap)
			if err != nil {
				return nil, err
			}
			if converted != nil {
				result = append(result, converted)
			}
		}
	}

	return result, nil
}

func (c *v1beta3Converter) convertAnalyzer(analyzer map[interface{}]interface{}) (interface{}, error) {
	// Convert analyzer based on type
	if _, exists := analyzer["clusterVersion"]; exists {
		return c.convertClusterVersion(analyzer)
	}

	if _, exists := analyzer["customResourceDefinition"]; exists {
		return c.convertCustomResourceDefinition(analyzer)
	}

	if _, exists := analyzer["containerRuntime"]; exists {
		return c.convertContainerRuntime(analyzer)
	}

	if _, exists := analyzer["storageClass"]; exists {
		return c.convertStorageClass(analyzer)
	}

	if _, exists := analyzer["distribution"]; exists {
		return c.convertDistribution(analyzer)
	}

	if _, exists := analyzer["nodeResources"]; exists {
		return c.convertNodeResources(analyzer)
	}

	// For unrecognized analyzers, return as-is with warning comment
	return c.wrapWithWarning(analyzer, "Unknown analyzer type - manual review required")
}

func (c *v1beta3Converter) convertClusterVersion(analyzer map[interface{}]interface{}) (interface{}, error) {
	// Enable kubernetes checks
	c.setNestedValue("kubernetes.enabled", true)

	// Extract version requirements from outcomes
	if cv, ok := analyzer["clusterVersion"].(map[interface{}]interface{}); ok {
		if outcomes, ok := cv["outcomes"].([]interface{}); ok {
			c.extractVersionRequirements(outcomes)
		}
	}

	return c.createTemplatedAnalyzer("kubernetes", analyzer, "")
}

func (c *v1beta3Converter) convertCustomResourceDefinition(analyzer map[interface{}]interface{}) (interface{}, error) {
	c.setNestedValue("ingress.enabled", true)

	if crd, ok := analyzer["customResourceDefinition"].(map[interface{}]interface{}); ok {
		if crdName, ok := crd["customResourceDefinitionName"].(string); ok {
			if strings.Contains(crdName, "contour") {
				c.setNestedValue("ingress.type", "Contour")
			}
		}
	}

	return c.createTemplatedAnalyzer("ingress", analyzer, "")
}

func (c *v1beta3Converter) convertContainerRuntime(analyzer map[interface{}]interface{}) (interface{}, error) {
	c.setNestedValue("runtime.enabled", true)

	return c.createTemplatedAnalyzer("runtime", analyzer, "")
}

func (c *v1beta3Converter) convertStorageClass(analyzer map[interface{}]interface{}) (interface{}, error) {
	c.setNestedValue("storage.enabled", true)

	// Extract storage class name
	if sc, ok := analyzer["storageClass"].(map[interface{}]interface{}); ok {
		if className, ok := sc["storageClassName"].(string); ok {
			c.setNestedValue("storage.className", className)
		}
	}

	// Update the analyzer to use template
	if sc, ok := analyzer["storageClass"].(map[interface{}]interface{}); ok {
		sc["storageClassName"] = "{{ .Values.storage.className }}"
	}

	return c.createTemplatedAnalyzer("storage", analyzer, "")
}

func (c *v1beta3Converter) convertDistribution(analyzer map[interface{}]interface{}) (interface{}, error) {
	c.setNestedValue("distribution.enabled", true)

	return c.createTemplatedAnalyzer("distribution", analyzer, "")
}

func (c *v1beta3Converter) convertNodeResources(analyzer map[interface{}]interface{}) (interface{}, error) {
	if nr, ok := analyzer["nodeResources"].(map[interface{}]interface{}); ok {
		checkName := ""
		if name, ok := nr["checkName"].(string); ok {
			checkName = strings.ToLower(name)
		}

		// Determine node resource type and enable appropriate check
		if strings.Contains(checkName, "node") && strings.Contains(checkName, "count") {
			c.setNestedValue("nodeChecks.enabled", true)
			c.setNestedValue("nodeChecks.count.enabled", true)
			c.extractNodeCountRequirements(nr)
			return c.createTemplatedAnalyzer("nodeChecks.count", analyzer, "")
		}

		if strings.Contains(checkName, "cpu") || strings.Contains(checkName, "core") {
			c.setNestedValue("nodeChecks.enabled", true)
			c.setNestedValue("nodeChecks.cpu.enabled", true)
			c.extractCPURequirements(nr)
			return c.createTemplatedAnalyzer("nodeChecks.cpu", analyzer, "")
		}

		if strings.Contains(checkName, "memory") {
			c.setNestedValue("nodeChecks.enabled", true)
			c.setNestedValue("nodeChecks.memory.enabled", true)
			c.extractMemoryRequirements(nr)
			c.templatizeMemoryOutcomes(analyzer)
			return c.createTemplatedAnalyzer("nodeChecks.memory", analyzer, "")
		}

		if strings.Contains(checkName, "ephemeral") || strings.Contains(checkName, "storage") {
			c.setNestedValue("nodeChecks.enabled", true)
			c.setNestedValue("nodeChecks.ephemeral.enabled", true)
			c.extractEphemeralRequirements(nr)
			c.templatizeEphemeralOutcomes(analyzer)
			return c.createTemplatedAnalyzer("nodeChecks.ephemeral", analyzer, "")
		}
	}

	// Default case - enable general node checks
	c.setNestedValue("nodeChecks.enabled", true)
	return c.createTemplatedAnalyzer("nodeChecks", analyzer, "")
}

func (c *v1beta3Converter) createTemplatedAnalyzer(checkType string, originalAnalyzer map[interface{}]interface{}, docString string) (interface{}, error) {
	// Convert map[interface{}]interface{} to map[string]interface{} for proper YAML output
	convertedAnalyzer := c.convertMapKeys(originalAnalyzer)

	// Add placeholder docString - user should replace with their actual requirements
	convertedAnalyzer["docString"] = "# TODO: Add docString with Title, Requirement, and rationale for this check"

	// Marshal the analyzer to YAML
	analyzerBytes, err := yaml.Marshal(convertedAnalyzer)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal analyzer")
	}

	// Create template string with proper indentation
	analyzerYAML := strings.TrimSuffix(string(analyzerBytes), "\n")

	// Add conditional wrapper
	condition := fmt.Sprintf("{{- if .Values.%s.enabled }}", checkType)
	endCondition := "{{- end }}"

	templateStr := fmt.Sprintf("%s\n- %s\n%s", condition,
		strings.ReplaceAll(analyzerYAML, "\n", "\n  "),
		endCondition)

	return templateStr, nil
}

func (c *v1beta3Converter) wrapWithWarning(analyzer map[interface{}]interface{}, warning string) (interface{}, error) {
	convertedAnalyzer := c.convertMapKeys(analyzer)
	convertedAnalyzer["docString"] = fmt.Sprintf("# TODO: Manual Review Required - %s", warning)
	return convertedAnalyzer, nil
}

func (c *v1beta3Converter) convertMapKeys(m map[interface{}]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		strKey := fmt.Sprintf("%v", k)
		switch val := v.(type) {
		case map[interface{}]interface{}:
			result[strKey] = c.convertMapKeys(val)
		case []interface{}:
			result[strKey] = c.convertSlice(val)
		default:
			result[strKey] = val
		}
	}
	return result
}

func (c *v1beta3Converter) convertSlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case map[interface{}]interface{}:
			result[i] = c.convertMapKeys(val)
		case []interface{}:
			result[i] = c.convertSlice(val)
		default:
			result[i] = val
		}
	}
	return result
}

// Helper methods for extracting requirements from outcomes
func (c *v1beta3Converter) extractVersionRequirements(outcomes []interface{}) {
	for _, outcome := range outcomes {
		if outcomeMap, ok := outcome.(map[interface{}]interface{}); ok {
			if fail, ok := outcomeMap["fail"].(map[interface{}]interface{}); ok {
				if when, ok := fail["when"].(string); ok {
					if version := c.extractVersionFromWhen(when); version != "" {
						c.setNestedValue("kubernetes.minVersion", version)
					}
				}
			}
			if warn, ok := outcomeMap["warn"].(map[interface{}]interface{}); ok {
				if when, ok := warn["when"].(string); ok {
					if version := c.extractVersionFromWhen(when); version != "" {
						c.setNestedValue("kubernetes.recommendedVersion", version)
					}
				}
			}
		}
	}
}

func (c *v1beta3Converter) extractVersionFromWhen(when string) string {
	// Simple version extraction from conditions like "< 1.22.0"
	when = strings.TrimSpace(when)
	if strings.HasPrefix(when, "<") {
		version := strings.TrimSpace(strings.TrimPrefix(when, "<"))
		version = strings.Trim(version, `"`)
		return version
	}
	return ""
}

func (c *v1beta3Converter) extractNodeCountRequirements(nr map[interface{}]interface{}) {
	if outcomes, ok := nr["outcomes"].([]interface{}); ok {
		for _, outcome := range outcomes {
			if outcomeMap, ok := outcome.(map[interface{}]interface{}); ok {
				if fail, ok := outcomeMap["fail"].(map[interface{}]interface{}); ok {
					if when, ok := fail["when"].(string); ok {
						if count := c.extractNumberFromWhen(when, "count()"); count > 0 {
							c.setNestedValue("cluster.minNodes", count)
						}
					}
				}
				if warn, ok := outcomeMap["warn"].(map[interface{}]interface{}); ok {
					if when, ok := warn["when"].(string); ok {
						if count := c.extractNumberFromWhen(when, "count()"); count > 0 {
							c.setNestedValue("cluster.recommendedNodes", count)
						}
					}
				}
			}
		}
	}
}

func (c *v1beta3Converter) extractCPURequirements(nr map[interface{}]interface{}) {
	if outcomes, ok := nr["outcomes"].([]interface{}); ok {
		for _, outcome := range outcomes {
			if outcomeMap, ok := outcome.(map[interface{}]interface{}); ok {
				if fail, ok := outcomeMap["fail"].(map[interface{}]interface{}); ok {
					if when, ok := fail["when"].(string); ok {
						if cpu := c.extractNumberFromWhen(when, "sum(cpuCapacity)"); cpu > 0 {
							c.setNestedValue("cluster.minCPU", cpu)
						}
					}
				}
			}
		}
	}
}

func (c *v1beta3Converter) extractMemoryRequirements(nr map[interface{}]interface{}) {
	if outcomes, ok := nr["outcomes"].([]interface{}); ok {
		for _, outcome := range outcomes {
			if outcomeMap, ok := outcome.(map[interface{}]interface{}); ok {
				if fail, ok := outcomeMap["fail"].(map[interface{}]interface{}); ok {
					if when, ok := fail["when"].(string); ok {
						if memory := c.extractMemoryFromWhen(when); memory > 0 {
							c.setNestedValue("node.minMemoryGi", memory)
						}
					}
				}
				if warn, ok := outcomeMap["warn"].(map[interface{}]interface{}); ok {
					if when, ok := warn["when"].(string); ok {
						if memory := c.extractMemoryFromWhen(when); memory > 0 {
							c.setNestedValue("node.recommendedMemoryGi", memory)
						}
					}
				}
			}
		}
	}
}

func (c *v1beta3Converter) extractEphemeralRequirements(nr map[interface{}]interface{}) {
	if outcomes, ok := nr["outcomes"].([]interface{}); ok {
		for _, outcome := range outcomes {
			if outcomeMap, ok := outcome.(map[interface{}]interface{}); ok {
				if fail, ok := outcomeMap["fail"].(map[interface{}]interface{}); ok {
					if when, ok := fail["when"].(string); ok {
						if storage := c.extractStorageFromWhen(when); storage > 0 {
							c.setNestedValue("node.minEphemeralGi", storage)
						}
					}
				}
				if warn, ok := outcomeMap["warn"].(map[interface{}]interface{}); ok {
					if when, ok := warn["when"].(string); ok {
						if storage := c.extractStorageFromWhen(when); storage > 0 {
							c.setNestedValue("node.recommendedEphemeralGi", storage)
						}
					}
				}
			}
		}
	}
}

func (c *v1beta3Converter) extractNumberFromWhen(when, prefix string) int {
	when = strings.TrimSpace(when)
	if strings.Contains(when, prefix) {
		// Extract number from conditions like "count() < 3"
		parts := strings.Split(when, "<")
		if len(parts) == 2 {
			numStr := strings.TrimSpace(parts[1])
			if num, err := strconv.Atoi(numStr); err == nil {
				return num
			}
		}
	}
	return 0
}

func (c *v1beta3Converter) extractMemoryFromWhen(when string) int {
	when = strings.TrimSpace(when)
	// Handle conditions like "min(memoryCapacity) < 8Gi"
	if strings.Contains(when, "memoryCapacity") {
		parts := strings.Split(when, "<")
		if len(parts) == 2 {
			sizeStr := strings.TrimSpace(parts[1])
			sizeStr = strings.TrimSuffix(sizeStr, "Gi")
			if num, err := strconv.Atoi(sizeStr); err == nil {
				return num
			}
		}
	}
	return 0
}

func (c *v1beta3Converter) extractStorageFromWhen(when string) int {
	when = strings.TrimSpace(when)
	// Handle conditions like "min(ephemeralStorageCapacity) < 40Gi"
	if strings.Contains(when, "ephemeralStorageCapacity") {
		parts := strings.Split(when, "<")
		if len(parts) == 2 {
			sizeStr := strings.TrimSpace(parts[1])
			sizeStr = strings.TrimSuffix(sizeStr, "Gi")
			if num, err := strconv.Atoi(sizeStr); err == nil {
				return num
			}
		}
	}
	return 0
}

func (c *v1beta3Converter) templatizeMemoryOutcomes(analyzer map[interface{}]interface{}) {
	c.templatizeNodeResourceOutcomes(analyzer, "memoryCapacity", "node.minMemoryGi", "node.recommendedMemoryGi")
}

func (c *v1beta3Converter) templatizeEphemeralOutcomes(analyzer map[interface{}]interface{}) {
	c.templatizeNodeResourceOutcomes(analyzer, "ephemeralStorageCapacity", "node.minEphemeralGi", "node.recommendedEphemeralGi")
}

func (c *v1beta3Converter) templatizeNodeResourceOutcomes(analyzer map[interface{}]interface{}, capacity, minKey, recKey string) {
	if nr, ok := analyzer["nodeResources"].(map[interface{}]interface{}); ok {
		if outcomes, ok := nr["outcomes"].([]interface{}); ok {
			for _, outcome := range outcomes {
				if outcomeMap, ok := outcome.(map[interface{}]interface{}); ok {
					// Update fail condition
					if fail, ok := outcomeMap["fail"].(map[interface{}]interface{}); ok {
						if when, ok := fail["when"].(string); ok && strings.Contains(when, capacity) {
							fail["when"] = fmt.Sprintf("min(%s) < {{ .Values.%s }}Gi", capacity, minKey)
						}
						if _, ok := fail["message"].(string); ok {
							parts := strings.Split(minKey, ".")
							fail["message"] = fmt.Sprintf("All nodes must have at least {{ .Values.%s }} GiB of %s.", minKey, parts[len(parts)-1])
						}
					}
					// Update warn condition
					if warn, ok := outcomeMap["warn"].(map[interface{}]interface{}); ok {
						if when, ok := warn["when"].(string); ok && strings.Contains(when, capacity) {
							warn["when"] = fmt.Sprintf("min(%s) < {{ .Values.%s }}Gi", capacity, recKey)
						}
						if _, ok := warn["message"].(string); ok {
							parts := strings.Split(recKey, ".")
							warn["message"] = fmt.Sprintf("All nodes are recommended to have at least {{ .Values.%s }} GiB of %s.", recKey, parts[len(parts)-1])
						}
					}
					// Update pass message
					if pass, ok := outcomeMap["pass"].(map[interface{}]interface{}); ok {
						if _, ok := pass["message"].(string); ok {
							parts := strings.Split(recKey, ".")
							pass["message"] = fmt.Sprintf("All nodes have at least {{ .Values.%s }} GiB of %s.", recKey, parts[len(parts)-1])
						}
					}
				}
			}
		}
	}
}

func (c *v1beta3Converter) setNestedValue(path string, value interface{}) {
	parts := strings.Split(path, ".")
	current := c.values

	for _, part := range parts[:len(parts)-1] {
		if _, ok := current[part]; !ok {
			current[part] = make(map[string]interface{})
		}
		if nextMap, ok := current[part].(map[string]interface{}); ok {
			current = nextMap
		} else {
			// Path exists but isn't a map, need to handle this case
			return
		}
	}

	current[parts[len(parts)-1]] = value
}

