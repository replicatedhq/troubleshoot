package supportbundle

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// ExtractLicenseFromBundle extracts the license ID from a support bundle
// It looks in cluster-resources/configmaps/* for a license field
// Returns both the license ID and the app slug (from the filename where license was found)
func ExtractLicenseFromBundle(bundlePath string) (string, string, error) {
	file, err := os.Open(bundlePath)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to open bundle file")
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return "", "", errors.Wrap(err, "failed to create gzip reader")
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", errors.Wrap(err, "failed to read tar header")
		}

		// Only process files in cluster-resources/configmaps/ (may be nested under bundle directory)
		if !strings.Contains(header.Name, "cluster-resources/configmaps/") {
			continue
		}

		// Skip directories
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Process .yaml, .yml, and .json files
		if !strings.HasSuffix(header.Name, ".yaml") &&
		   !strings.HasSuffix(header.Name, ".yml") &&
		   !strings.HasSuffix(header.Name, ".json") {
			continue
		}

		// Read the file content
		content := make([]byte, header.Size)
		if _, err := io.ReadFull(tarReader, content); err != nil {
			continue // Skip files we can't read
		}

		// Try to extract license from this configmap
		var license string
		if strings.HasSuffix(header.Name, ".json") {
			license = extractLicenseFromJSON(content)
		} else {
			license = extractLicenseFromConfigMap(content)
		}

		if license != "" {
			// Extract app slug from filename
			filename := filepath.Base(header.Name)
			appSlug := strings.TrimSuffix(filename, ".json")
			appSlug = strings.TrimSuffix(appSlug, ".yaml")
			appSlug = strings.TrimSuffix(appSlug, ".yml")
			return license, appSlug, nil
		}
	}

	return "", "", nil // No license found
}

// extractLicenseFromConfigMap attempts to extract a license ID from a ConfigMap YAML
func extractLicenseFromConfigMap(content []byte) string {
	// First try to parse as YAML
	var configMap map[string]interface{}
	if err := yaml.Unmarshal(content, &configMap); err != nil {
		// If YAML parsing fails, try regex as fallback
		return extractLicenseWithRegex(string(content))
	}

	// Look for data field in ConfigMap
	data, ok := configMap["data"].(map[interface{}]interface{})
	if !ok {
		return extractLicenseWithRegex(string(content))
	}

	// Check for license field in data
	for key, value := range data {
		keyStr, ok := key.(string)
		if !ok {
			continue
		}

		// Look for license-related keys
		if strings.ToLower(keyStr) == "license" || strings.Contains(strings.ToLower(keyStr), "license") {
			valueStr, ok := value.(string)
			if ok && isValidLicenseID(valueStr) {
				return valueStr
			}
			// The license might be YAML within YAML
			if licenseID := extractLicenseFromNested(valueStr); licenseID != "" {
				return licenseID
			}
		}
	}

	// Fallback to regex search
	return extractLicenseWithRegex(string(content))
}

// extractLicenseFromNested tries to extract license from nested YAML content
func extractLicenseFromNested(content string) string {
	// Try to parse as YAML
	var nested map[string]interface{}
	if err := yaml.Unmarshal([]byte(content), &nested); err != nil {
		return extractLicenseWithRegex(content)
	}

	// Look for licenseID or license field
	if licenseID, ok := nested["licenseID"].(string); ok && isValidLicenseID(licenseID) {
		return licenseID
	}
	if licenseID, ok := nested["license_id"].(string); ok && isValidLicenseID(licenseID) {
		return licenseID
	}
	if licenseID, ok := nested["license"].(string); ok && isValidLicenseID(licenseID) {
		return licenseID
	}

	return extractLicenseWithRegex(content)
}

// extractLicenseWithRegex uses regex to find license patterns in text
func extractLicenseWithRegex(content string) string {
	// Common patterns for license IDs in various formats
	// Including patterns that might appear in embedded YAML within JSON
	patterns := []string{
		`licenseID:\s*["']?([a-zA-Z0-9]{20,30})["']?`,
		`license_id:\s*["']?([a-zA-Z0-9]{20,30})["']?`,
		`license:\s*["']?([a-zA-Z0-9]{20,30})["']?`,
		`"licenseID":\s*"([a-zA-Z0-9]{20,30})"`,
		`"license_id":\s*"([a-zA-Z0-9]{20,30})"`,
		`"license":\s*"([a-zA-Z0-9]{20,30})"`,
		`licenseID: ([a-zA-Z0-9]{20,30})`, // YAML format without quotes
		`\\nlicenseID: ([a-zA-Z0-9]{20,30})`, // With escaped newline
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			if isValidLicenseID(matches[1]) {
				return matches[1]
			}
		}
	}

	return ""
}

// extractLicenseFromJSON extracts license ID from a JSON file
func extractLicenseFromJSON(content []byte) string {
	// First try to find license ID directly in the raw content
	// This handles cases where the license is in embedded YAML/strings
	if license := extractLicenseWithRegex(string(content)); license != "" {
		return license
	}

	// Then try parsing as JSON for structured search
	var data map[string]interface{}
	if err := json.Unmarshal(content, &data); err != nil {
		return ""
	}

	// Look for license-related fields at any level
	return findLicenseInMap(data)
}

// findLicenseInMap recursively searches for license ID in a map
func findLicenseInMap(data interface{}) string {
	switch v := data.(type) {
	case map[string]interface{}:
		// Check for license fields at this level
		for key, value := range v {
			keyLower := strings.ToLower(key)
			if keyLower == "licenseid" || keyLower == "license_id" || keyLower == "license" {
				if str, ok := value.(string); ok && isValidLicenseID(str) {
					return str
				}
			}
		}
		// Recurse into nested objects
		for _, value := range v {
			if result := findLicenseInMap(value); result != "" {
				return result
			}
		}
	case []interface{}:
		// Recurse into arrays
		for _, item := range v {
			if result := findLicenseInMap(item); result != "" {
				return result
			}
		}
	case string:
		// Check if this string itself is a license
		if isValidLicenseID(v) {
			return v
		}
	}
	return ""
}

// isValidLicenseID checks if a string looks like a valid license ID
func isValidLicenseID(s string) bool {
	// License IDs are typically 20-30 character alphanumeric strings
	if len(s) < 20 || len(s) > 30 {
		return false
	}

	// Must be alphanumeric
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
			return false
		}
	}

	return true
}

// ExtractAppSlugFromBundle attempts to extract the app slug from a support bundle
// by looking in configmaps for appSlug field
func ExtractAppSlugFromBundle(bundlePath string) (string, error) {
	file, err := os.Open(bundlePath)
	if err != nil {
		return "", errors.Wrap(err, "failed to open bundle file")
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return "", errors.Wrap(err, "failed to create gzip reader")
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", errors.Wrap(err, "failed to read tar header")
		}

		// Only process files in cluster-resources/configmaps/ (may be nested under bundle directory)
		if !strings.Contains(header.Name, "cluster-resources/configmaps/") {
			continue
		}

		// Skip directories
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Process .yaml, .yml, and .json files
		if !strings.HasSuffix(header.Name, ".yaml") &&
		   !strings.HasSuffix(header.Name, ".yml") &&
		   !strings.HasSuffix(header.Name, ".json") {
			continue
		}

		// Read the file content
		content := make([]byte, header.Size)
		if _, err := io.ReadFull(tarReader, content); err != nil {
			continue // Skip files we can't read
		}

		// Try to extract app slug from this content
		if appSlug := extractAppSlugFromContent(string(content)); appSlug != "" {
			return appSlug, nil
		}

		// Also try to extract from the filename as fallback
		filename := filepath.Base(header.Name)
		filename = strings.TrimSuffix(filename, ".yaml")
		filename = strings.TrimSuffix(filename, ".yml")
		filename = strings.TrimSuffix(filename, ".json")

		// Skip common Kubernetes configmaps
		if filename == "kube-root-ca.crt" || strings.HasPrefix(filename, "kube-") ||
		   strings.HasPrefix(filename, "kotsadm-") {
			continue
		}

		// Use the filename as a potential app slug
		if filename != "" && !strings.Contains(filename, "..") {
			return filename, nil
		}
	}

	return "", fmt.Errorf("could not determine app slug from bundle")
}

// extractAppSlugFromContent tries to find app slug in file content
func extractAppSlugFromContent(content string) string {
	// Patterns to find app slug in various formats
	patterns := []string{
		`appSlug:\s*["']?([a-zA-Z0-9\-]+)["']?`,
		`app_slug:\s*["']?([a-zA-Z0-9\-]+)["']?`,
		`"appSlug":\s*"([a-zA-Z0-9\-]+)"`,
		`"app_slug":\s*"([a-zA-Z0-9\-]+)"`,
		`appSlug: ([a-zA-Z0-9\-]+)`, // YAML format without quotes
		`\\nappSlug: ([a-zA-Z0-9\-]+)`, // With escaped newline
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		if matches := re.FindStringSubmatch(content); len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}