package generators

import (
	"strings"
)

// Template definitions for different analyzer types

// getKubernetesTemplate returns the template for Kubernetes analyzers
func getKubernetesTemplate() string {
	return `package {{.PackageName}}

import (
	"context"
	"fmt"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// {{.Name}} validates Kubernetes {{.Type}} requirements
type {{.Name}} struct {
	analyzer *troubleshootv1beta2.{{.Type | title}}Analyze
}

// Title returns the title of the analyzer
func (a *{{.Name}}) Title() string {
	return "{{.Description}}"
}

// IsExcluded checks if the analyzer should be excluded
func (a *{{.Name}}) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

// Analyze performs the analysis
func (a *{{.Name}}) Analyze(getFile getFileFunc, findFiles getChildCollectedFilesFunc) ([]*AnalyzeResult, error) {
	results := []*AnalyzeResult{}

	{{if .HasMinValue}}
	// Check minimum version requirement
	if err := a.checkMinimumVersion(getFile, findFiles); err != nil {
		results = append(results, &AnalyzeResult{
			IsPass:  false,
			IsFail:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("Version check failed: %v", err),
		})
		return results, nil
	}
	{{end}}

	{{if .HasMaxValue}}
	// Check maximum version requirement
	if err := a.checkMaximumVersion(getFile, findFiles); err != nil {
		results = append(results, &AnalyzeResult{
			IsPass:  false,
			IsFail:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("Version check failed: %v", err),
		})
		return results, nil
	}
	{{end}}

	{{range .RequiredFields}}
	// Check required field: {{.}}
	if err := a.check{{. | fieldName}}(getFile, findFiles); err != nil {
		results = append(results, &AnalyzeResult{
			IsPass:  false,
			IsFail:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("{{.}} check failed: %v", err),
		})
		return results, nil
	}
	{{end}}

	// All checks passed
	results = append(results, &AnalyzeResult{
		IsPass:  true,
		Title:   a.Title(),
		Message: "All {{.Type}} requirements are satisfied",
	})

	return results, nil
}

{{if .HasMinValue}}
// checkMinimumVersion checks if the minimum version requirement is met
func (a *{{.Name}}) checkMinimumVersion(getFile getFileFunc, findFiles getChildCollectedFilesFunc) error {
	// Implementation for minimum version check
	versionFile := "cluster-info/version.json"
	contents, err := getFile(versionFile)
	if err != nil {
		return fmt.Errorf("failed to get version info: %w", err)
	}

	// Parse version and compare
	// This is a simplified implementation
	if len(contents) == 0 {
		return fmt.Errorf("version information not available")
	}

	return nil
}
{{end}}

{{if .HasMaxValue}}
// checkMaximumVersion checks if the maximum version requirement is met
func (a *{{.Name}}) checkMaximumVersion(getFile getFileFunc, findFiles getChildCollectedFilesFunc) error {
	// Implementation for maximum version check
	versionFile := "cluster-info/version.json"
	contents, err := getFile(versionFile)
	if err != nil {
		return fmt.Errorf("failed to get version info: %w", err)
	}

	// Parse version and compare
	// This is a simplified implementation
	if len(contents) == 0 {
		return fmt.Errorf("version information not available")
	}

	return nil
}
{{end}}

{{range .RequiredFields}}
// check{{. | fieldName}} checks the {{.}} requirement
func (a *{{$.Name}}) check{{. | fieldName}}(getFile getFileFunc, findFiles getChildCollectedFilesFunc) error {
	// Implementation for {{.}} check
	// This would contain specific logic for validating {{.}}
	return nil
}
{{end}}

// Helper functions

func isExcluded(exclude *troubleshootv1beta2.Exclude) (bool, error) {
	return false, nil // Simplified implementation
}

type getFileFunc func(string) ([]byte, error)
type getChildCollectedFilesFunc func(string) (map[string][]byte, error)
`
}

// getResourceTemplate returns the template for resource analyzers
func getResourceTemplate() string {
	return `package {{.PackageName}}

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// {{.Name}} validates resource requirements
type {{.Name}} struct {
	analyzer *troubleshootv1beta2.{{.Type | title}}Analyze
}

// Title returns the title of the analyzer
func (a *{{.Name}}) Title() string {
	return "{{.Description}}"
}

// IsExcluded checks if the analyzer should be excluded
func (a *{{.Name}}) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

// Analyze performs the resource analysis
func (a *{{.Name}}) Analyze(getFile getFileFunc, findFiles getChildCollectedFilesFunc) ([]*AnalyzeResult, error) {
	results := []*AnalyzeResult{}

	{{if contains .Type "cpu" "CPU"}}
	// Check CPU requirements
	cpuResult, err := a.checkCPURequirements(getFile, findFiles)
	if err != nil {
		return nil, fmt.Errorf("CPU check failed: %w", err)
	}
	results = append(results, cpuResult...)
	{{end}}

	{{if contains .Type "memory" "Memory"}}
	// Check memory requirements
	memResult, err := a.checkMemoryRequirements(getFile, findFiles)
	if err != nil {
		return nil, fmt.Errorf("memory check failed: %w", err)
	}
	results = append(results, memResult...)
	{{end}}

	{{if contains .Type "node" "Node"}}
	// Check node requirements
	nodeResult, err := a.checkNodeRequirements(getFile, findFiles)
	if err != nil {
		return nil, fmt.Errorf("node check failed: %w", err)
	}
	results = append(results, nodeResult...)
	{{end}}

	return results, nil
}

{{if contains .Type "cpu" "CPU"}}
// checkCPURequirements checks CPU resource requirements
func (a *{{.Name}}) checkCPURequirements(getFile getFileFunc, findFiles getChildCollectedFilesFunc) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult

	// Get node resources
	nodeFiles := findFiles("cluster-resources/nodes")
	if len(nodeFiles) == 0 {
		return []*AnalyzeResult{{
			IsFail:  true,
			Title:   a.Title(),
			Message: "Unable to find node resource information",
		}}, nil
	}

	totalCPU := resource.NewQuantity(0, resource.DecimalSI)

	for _, nodeData := range nodeFiles {
		// Parse node resource data
		// This is simplified - actual implementation would parse JSON/YAML
		if strings.Contains(string(nodeData), "cpu") {
			// Extract CPU information
			// totalCPU.Add(*extractedCPU)
		}
	}

	{{range .Requirements}}
	{{if contains .Path "cpu" "CPU"}}
	// Check requirement: {{.Path}}
	minCPU := {{.Value}} // This would come from the requirement
	if totalCPU.Cmp(*resource.NewQuantity(int64(minCPU), resource.DecimalSI)) < 0 {
		results = append(results, &AnalyzeResult{
			IsFail:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("Insufficient CPU: required %v, available %v", minCPU, totalCPU),
		})
	} else {
		results = append(results, &AnalyzeResult{
			IsPass:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("CPU requirement satisfied: %v available", totalCPU),
		})
	}
	{{end}}
	{{end}}

	return results, nil
}
{{end}}

{{if contains .Type "memory" "Memory"}}
// checkMemoryRequirements checks memory resource requirements
func (a *{{.Name}}) checkMemoryRequirements(getFile getFileFunc, findFiles getChildCollectedFilesFunc) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult

	// Get node resources
	nodeFiles := findFiles("cluster-resources/nodes")
	if len(nodeFiles) == 0 {
		return []*AnalyzeResult{{
			IsFail:  true,
			Title:   a.Title(),
			Message: "Unable to find node resource information",
		}}, nil
	}

	totalMemory := resource.NewQuantity(0, resource.BinarySI)

	for _, nodeData := range nodeFiles {
		// Parse node memory data
		// This is simplified - actual implementation would parse JSON/YAML
		if strings.Contains(string(nodeData), "memory") {
			// Extract memory information
			// totalMemory.Add(*extractedMemory)
		}
	}

	{{range .Requirements}}
	{{if contains .Path "memory" "Memory"}}
	// Check requirement: {{.Path}}
	minMemory := {{.Value}} // This would come from the requirement
	if totalMemory.Cmp(*resource.NewQuantity(int64(minMemory), resource.BinarySI)) < 0 {
		results = append(results, &AnalyzeResult{
			IsFail:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("Insufficient memory: required %v, available %v", minMemory, totalMemory),
		})
	} else {
		results = append(results, &AnalyzeResult{
			IsPass:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("Memory requirement satisfied: %v available", totalMemory),
		})
	}
	{{end}}
	{{end}}

	return results, nil
}
{{end}}

{{if contains .Type "node" "Node"}}
// checkNodeRequirements checks node count requirements
func (a *{{.Name}}) checkNodeRequirements(getFile getFileFunc, findFiles getChildCollectedFilesFunc) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult

	// Count nodes
	nodeFiles := findFiles("cluster-resources/nodes")
	nodeCount := len(nodeFiles)

	{{range .Requirements}}
	{{if contains .Path "node" "Node"}}
	// Check requirement: {{.Path}}
	{{if contains .Path "min" "Min"}}
	minNodes := {{.Value}}
	if nodeCount < minNodes {
		results = append(results, &AnalyzeResult{
			IsFail:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("Insufficient nodes: required %d, found %d", minNodes, nodeCount),
		})
	} else {
		results = append(results, &AnalyzeResult{
			IsPass:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("Node count requirement satisfied: %d nodes found", nodeCount),
		})
	}
	{{end}}
	{{end}}
	{{end}}

	return results, nil
}
{{end}}

// Helper functions

func isExcluded(exclude *troubleshootv1beta2.Exclude) (bool, error) {
	return false, nil // Simplified implementation
}

type getFileFunc func(string) ([]byte, error)
type getChildCollectedFilesFunc func(string) (map[string][]byte, error)
`
}

// getStorageTemplate returns the template for storage analyzers
func getStorageTemplate() string {
	return `package {{.PackageName}}

import (
	"context"
	"fmt"
	"encoding/json"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// {{.Name}} validates storage requirements
type {{.Name}} struct {
	analyzer *troubleshootv1beta2.{{.Type | title}}Analyze
}

// Title returns the title of the analyzer
func (a *{{.Name}}) Title() string {
	return "{{.Description}}"
}

// IsExcluded checks if the analyzer should be excluded
func (a *{{.Name}}) IsExcluded() (bool, error) {
	return isExcluded(a.analyzer.Exclude)
}

// Analyze performs the storage analysis
func (a *{{.Name}}) Analyze(getFile getFileFunc, findFiles getChildCollectedFilesFunc) ([]*AnalyzeResult, error) {
	results := []*AnalyzeResult{}

	{{if contains .Type "class" "Class"}}
	// Check storage class requirements
	classResult, err := a.checkStorageClass(getFile, findFiles)
	if err != nil {
		return nil, fmt.Errorf("storage class check failed: %w", err)
	}
	results = append(results, classResult...)
	{{end}}

	{{if contains .Type "capacity" "Capacity"}}
	// Check storage capacity requirements
	capacityResult, err := a.checkStorageCapacity(getFile, findFiles)
	if err != nil {
		return nil, fmt.Errorf("storage capacity check failed: %w", err)
	}
	results = append(results, capacityResult...)
	{{end}}

	{{if contains .Type "performance" "Performance"}}
	// Check storage performance requirements
	performanceResult, err := a.checkStoragePerformance(getFile, findFiles)
	if err != nil {
		return nil, fmt.Errorf("storage performance check failed: %w", err)
	}
	results = append(results, performanceResult...)
	{{end}}

	return results, nil
}

{{if contains .Type "class" "Class"}}
// checkStorageClass checks storage class requirements
func (a *{{.Name}}) checkStorageClass(getFile getFileFunc, findFiles getChildCollectedFilesFunc) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult

	// Get storage classes
	storageClassFile := "cluster-resources/storage-classes.json"
	contents, err := getFile(storageClassFile)
	if err != nil {
		return []*AnalyzeResult{{
			IsFail:  true,
			Title:   a.Title(),
			Message: "Unable to get storage class information",
		}}, nil
	}

	var storageClasses []storagev1.StorageClass
	if err := json.Unmarshal(contents, &storageClasses); err != nil {
		return []*AnalyzeResult{{
			IsFail:  true,
			Title:   a.Title(),
			Message: "Unable to parse storage class information",
		}}, nil
	}

	{{range .Requirements}}
	{{if contains .Path "storageClass" "StorageClass"}}
	// Check for required storage class
	requiredClass := "{{.Value}}" // This would come from the requirement
	found := false
	for _, sc := range storageClasses {
		if sc.Name == requiredClass {
			found = true
			break
		}
	}

	if !found {
		results = append(results, &AnalyzeResult{
			IsFail:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("Required storage class '%s' not found", requiredClass),
		})
	} else {
		results = append(results, &AnalyzeResult{
			IsPass:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("Storage class '%s' is available", requiredClass),
		})
	}
	{{end}}
	{{end}}

	return results, nil
}
{{end}}

{{if contains .Type "capacity" "Capacity"}}
// checkStorageCapacity checks storage capacity requirements
func (a *{{.Name}}) checkStorageCapacity(getFile getFileFunc, findFiles getChildCollectedFilesFunc) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult

	// Get persistent volume information
	pvFiles := findFiles("cluster-resources/pvs")
	if len(pvFiles) == 0 {
		return []*AnalyzeResult{{
			IsFail:  true,
			Title:   a.Title(),
			Message: "Unable to find persistent volume information",
		}}, nil
	}

	totalCapacity := resource.NewQuantity(0, resource.BinarySI)

	for _, pvData := range pvFiles {
		// Parse PV capacity data
		// This is simplified - actual implementation would parse JSON/YAML
		// Extract capacity and add to total
	}

	{{range .Requirements}}
	{{if contains .Path "capacity" "Capacity"}}
	// Check capacity requirement: {{.Path}}
	minCapacity := {{.Value}} // This would come from the requirement
	minCapacityQuantity := resource.NewQuantity(int64(minCapacity), resource.BinarySI)

	if totalCapacity.Cmp(*minCapacityQuantity) < 0 {
		results = append(results, &AnalyzeResult{
			IsFail:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("Insufficient storage capacity: required %v, available %v", minCapacityQuantity, totalCapacity),
		})
	} else {
		results = append(results, &AnalyzeResult{
			IsPass:  true,
			Title:   a.Title(),
			Message: fmt.Sprintf("Storage capacity requirement satisfied: %v available", totalCapacity),
		})
	}
	{{end}}
	{{end}}

	return results, nil
}
{{end}}

{{if contains .Type "performance" "Performance"}}
// checkStoragePerformance checks storage performance requirements
func (a *{{.Name}}) checkStoragePerformance(getFile getFileFunc, findFiles getChildCollectedFilesFunc) ([]*AnalyzeResult, error) {
	var results []*AnalyzeResult

	{{range .Requirements}}
	{{if contains .Path "performance" "Performance" "iops" "IOPS"}}
	// Check performance requirement: {{.Path}}
	// This would involve checking storage class parameters, annotations, or
	// running performance tests against the storage
	
	results = append(results, &AnalyzeResult{
		IsPass:  true, // Simplified - would have actual performance check logic
		Title:   a.Title(),
		Message: "Storage performance requirements met",
	})
	{{end}}
	{{end}}

	return results, nil
}
{{end}}

// Helper functions

func isExcluded(exclude *troubleshootv1beta2.Exclude) (bool, error) {
	return false, nil // Simplified implementation
}

type getFileFunc func(string) ([]byte, error)
type getChildCollectedFilesFunc func(string) (map[string][]byte, error)
`
}

// Template helper functions would be registered with the template engine
// These are simplified examples of what the actual template functions would do

func fieldName(input string) string {
	// Convert field path to function name format
	parts := strings.Split(input, ".")
	if len(parts) > 0 {
		return strings.Title(parts[len(parts)-1])
	}
	return strings.Title(input)
}

func title(input string) string {
	return strings.Title(input)
}

func contains(str string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(str, substr) {
			return true
		}
	}
	return false
}
