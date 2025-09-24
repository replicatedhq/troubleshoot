package autodiscovery

import (
	"fmt"
	"os"
	"strings"

	"k8s.io/klog/v2"
)

// RBACReporter handles reporting of RBAC permission issues to users
type RBACReporter struct {
	warnings           []string
	filteredCollectors []CollectorSpec
	permissionIssues   []PermissionIssue
}

// PermissionIssue represents a specific RBAC permission problem
type PermissionIssue struct {
	Resource  string
	Namespace string
	Verb      string
	Collector string
	Reason    string
}

// NewRBACReporter creates a new RBAC reporter
func NewRBACReporter() *RBACReporter {
	return &RBACReporter{
		warnings:           make([]string, 0),
		filteredCollectors: make([]CollectorSpec, 0),
		permissionIssues:   make([]PermissionIssue, 0),
	}
}

// ReportFilteredCollector reports that a collector was filtered due to RBAC permissions
func (r *RBACReporter) ReportFilteredCollector(collector CollectorSpec, reason string) {
	warning := fmt.Sprintf("⚠️  Skipping %s: %s", collector.Name, reason)
	r.warnings = append(r.warnings, warning)
	r.filteredCollectors = append(r.filteredCollectors, collector)

	// Log the warning (visible to user in debug mode)
	klog.Warningf("RBAC: %s", warning)

	// Also output to stderr so user sees it even without debug mode
	fmt.Fprintf(os.Stderr, "%s\n", warning)

	// Track the specific permission issue
	r.trackPermissionIssue(collector, reason)
}

// ReportMissingPermission reports a specific missing permission
func (r *RBACReporter) ReportMissingPermission(resource, namespace, verb, collectorName string) {
	var location string
	if namespace != "" {
		location = fmt.Sprintf("%s in namespace %s", resource, namespace)
	} else {
		location = fmt.Sprintf("cluster-wide %s", resource)
	}

	warning := fmt.Sprintf("⚠️  Missing %s permission for %s (needed by %s collector)", verb, location, collectorName)
	r.warnings = append(r.warnings, warning)

	// Log the warning
	klog.Warningf("RBAC: %s", warning)
	fmt.Fprintf(os.Stderr, "%s\n", warning)

	// Track this permission issue
	issue := PermissionIssue{
		Resource:  resource,
		Namespace: namespace,
		Verb:      verb,
		Collector: collectorName,
		Reason:    fmt.Sprintf("Missing %s permission", verb),
	}
	r.permissionIssues = append(r.permissionIssues, issue)
}

// trackPermissionIssue extracts and tracks permission issue details
func (r *RBACReporter) trackPermissionIssue(collector CollectorSpec, reason string) {
	issue := PermissionIssue{
		Collector: collector.Name,
		Namespace: collector.Namespace,
		Reason:    reason,
	}

	// Try to extract resource and verb from collector type
	switch collector.Type {
	case CollectorTypeConfigMaps:
		issue.Resource = "configmaps"
		issue.Verb = "get,list"
	case CollectorTypeSecrets:
		issue.Resource = "secrets"
		issue.Verb = "get,list"
	case CollectorTypeLogs:
		issue.Resource = "pods"
		issue.Verb = "get,list"
	case CollectorTypeClusterResources:
		issue.Resource = "nodes,namespaces"
		issue.Verb = "get,list"
	case CollectorTypeClusterInfo:
		issue.Resource = "nodes"
		issue.Verb = "get,list"
	default:
		issue.Resource = string(collector.Type)
		issue.Verb = "get,list"
	}

	r.permissionIssues = append(r.permissionIssues, issue)
}

// HasWarnings returns true if any warnings were generated
func (r *RBACReporter) HasWarnings() bool {
	return len(r.warnings) > 0
}

// GetWarningCount returns the number of warnings generated
func (r *RBACReporter) GetWarningCount() int {
	return len(r.warnings)
}

// GetFilteredCollectorCount returns the number of collectors that were filtered
func (r *RBACReporter) GetFilteredCollectorCount() int {
	return len(r.filteredCollectors)
}

// GeneratePermissionSummary generates a summary of permission issues
func (r *RBACReporter) GeneratePermissionSummary() {
	if !r.HasWarnings() {
		return
	}

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "🔒 RBAC Permission Summary:\n")
	fmt.Fprintf(os.Stderr, "   • %d collectors were skipped due to insufficient permissions\n", len(r.filteredCollectors))
	fmt.Fprintf(os.Stderr, "   • This may result in incomplete troubleshooting data\n")
	fmt.Fprintf(os.Stderr, "\n")
}

// GenerateRemediationReport generates actionable commands to fix permission issues
func (r *RBACReporter) GenerateRemediationReport() {
	if !r.HasWarnings() {
		return
	}

	fmt.Fprintf(os.Stderr, "🔧 To collect missing resources, grant the following permissions:\n\n")

	// Generate specific permission commands based on what was missing
	clusterWideResources := []string{}
	namespacedResources := []string{}
	affectedNamespaces := make(map[string]bool)

	for _, issue := range r.permissionIssues {
		if issue.Namespace != "" {
			namespacedResources = append(namespacedResources, issue.Resource)
			affectedNamespaces[issue.Namespace] = true
		} else {
			clusterWideResources = append(clusterWideResources, issue.Resource)
		}
	}

	// Remove duplicates
	clusterWideResources = removeDuplicates(clusterWideResources)
	namespacedResources = removeDuplicates(namespacedResources)

	// Generate cluster-wide permissions command
	if len(clusterWideResources) > 0 {
		fmt.Fprintf(os.Stderr, "# Grant cluster-wide permissions:\n")
		fmt.Fprintf(os.Stderr, "kubectl create clusterrole troubleshoot-cluster-reader \\\n")
		fmt.Fprintf(os.Stderr, "  --verb=get,list \\\n")
		fmt.Fprintf(os.Stderr, "  --resource=%s\n\n", strings.Join(clusterWideResources, ","))

		fmt.Fprintf(os.Stderr, "kubectl create clusterrolebinding troubleshoot-cluster-reader \\\n")
		fmt.Fprintf(os.Stderr, "  --clusterrole=troubleshoot-cluster-reader \\\n")
		fmt.Fprintf(os.Stderr, "  --user=$(kubectl config view --minify -o jsonpath='{.contexts[0].context.user}')\n\n")
	}

	// Generate namespaced permissions command
	if len(namespacedResources) > 0 {
		fmt.Fprintf(os.Stderr, "# Grant namespaced permissions:\n")
		fmt.Fprintf(os.Stderr, "kubectl create clusterrole troubleshoot-namespace-reader \\\n")
		fmt.Fprintf(os.Stderr, "  --verb=get,list \\\n")
		fmt.Fprintf(os.Stderr, "  --resource=%s\n\n", strings.Join(namespacedResources, ","))

		fmt.Fprintf(os.Stderr, "kubectl create clusterrolebinding troubleshoot-namespace-reader \\\n")
		fmt.Fprintf(os.Stderr, "  --clusterrole=troubleshoot-namespace-reader \\\n")
		fmt.Fprintf(os.Stderr, "  --user=$(kubectl config view --minify -o jsonpath='{.contexts[0].context.user}')\n\n")
	}

	// Alternative: Single comprehensive role
	fmt.Fprintf(os.Stderr, "# Or create a comprehensive troubleshoot role:\n")
	fmt.Fprintf(os.Stderr, "kubectl create clusterrole troubleshoot-comprehensive \\\n")
	fmt.Fprintf(os.Stderr, "  --verb=get,list \\\n")
	fmt.Fprintf(os.Stderr, "  --resource=configmaps,secrets,pods,services,deployments,statefulsets,daemonsets,events,namespaces,nodes\n\n")

	fmt.Fprintf(os.Stderr, "kubectl create clusterrolebinding troubleshoot-comprehensive \\\n")
	fmt.Fprintf(os.Stderr, "  --clusterrole=troubleshoot-comprehensive \\\n")
	fmt.Fprintf(os.Stderr, "  --user=$(kubectl config view --minify -o jsonpath='{.contexts[0].context.user}')\n\n")

	// Provide alternative with service account
	fmt.Fprintf(os.Stderr, "# Alternative: Use current context user\n")
	fmt.Fprintf(os.Stderr, "CURRENT_USER=$(kubectl config current-context)\n")
	fmt.Fprintf(os.Stderr, "kubectl create clusterrolebinding troubleshoot-current-user \\\n")
	fmt.Fprintf(os.Stderr, "  --clusterrole=troubleshoot-comprehensive \\\n")
	fmt.Fprintf(os.Stderr, "  --user=$CURRENT_USER\n\n")

	fmt.Fprintf(os.Stderr, "💡 After granting permissions, re-run the support bundle collection.\n")
	fmt.Fprintf(os.Stderr, "\n")
}

// GenerateDebugInfo generates detailed debug information about RBAC filtering
func (r *RBACReporter) GenerateDebugInfo() {
	if !r.HasWarnings() {
		klog.V(2).Info("RBAC: No permission issues detected")
		return
	}

	klog.V(2).Infof("RBAC: Generated %d warnings for permission issues", len(r.warnings))
	klog.V(2).Infof("RBAC: Filtered %d collectors due to permissions", len(r.filteredCollectors))

	for _, issue := range r.permissionIssues {
		klog.V(3).Infof("RBAC Issue: %s collector needs %s permission for %s in namespace %s",
			issue.Collector, issue.Verb, issue.Resource, issue.Namespace)
	}
}

// Reset clears all warnings and tracked issues (useful for testing)
func (r *RBACReporter) Reset() {
	r.warnings = make([]string, 0)
	r.filteredCollectors = make([]CollectorSpec, 0)
	r.permissionIssues = make([]PermissionIssue, 0)
}

// GetFilteredCollectors returns the list of collectors that were filtered
func (r *RBACReporter) GetFilteredCollectors() []CollectorSpec {
	return r.filteredCollectors
}

// GetPermissionIssues returns the list of permission issues
func (r *RBACReporter) GetPermissionIssues() []PermissionIssue {
	return r.permissionIssues
}

// removeDuplicates removes duplicate strings from a slice
func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	var result []string

	for _, item := range slice {
		if !keys[item] {
			keys[item] = true
			result = append(result, item)
		}
	}

	return result
}

// SummarizeCollectionResults provides a final summary of what was collected vs. what was skipped
func (r *RBACReporter) SummarizeCollectionResults(totalCollectors int) {
	collectedCount := totalCollectors - len(r.filteredCollectors)

	if len(r.filteredCollectors) > 0 {
		fmt.Fprintf(os.Stderr, "\n📊 Collection Summary:\n")
		fmt.Fprintf(os.Stderr, "   ✅ Successfully collected: %d collectors\n", collectedCount)
		fmt.Fprintf(os.Stderr, "   ⚠️  Skipped due to permissions: %d collectors\n", len(r.filteredCollectors))
		fmt.Fprintf(os.Stderr, "   📊 Completion rate: %.1f%%\n", float64(collectedCount)/float64(totalCollectors)*100)

		if len(r.filteredCollectors) > 0 {
			fmt.Fprintf(os.Stderr, "\n   Missing collectors:\n")
			for _, collector := range r.filteredCollectors {
				fmt.Fprintf(os.Stderr, "   • %s (%s)\n", collector.Name, collector.Type)
			}
		}
		fmt.Fprintf(os.Stderr, "\n")
	} else {
		klog.V(2).Infof("RBAC: All %d collectors collected successfully", totalCollectors)
	}
}
