package autodiscovery

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// NamespaceScanner handles namespace discovery and filtering
type NamespaceScanner struct {
	client kubernetes.Interface
}

// NewNamespaceScanner creates a new namespace scanner
func NewNamespaceScanner(client kubernetes.Interface) *NamespaceScanner {
	return &NamespaceScanner{
		client: client,
	}
}

// ScanOptions configures namespace scanning behavior
type ScanOptions struct {
	// IncludePatterns are glob patterns for namespaces to include
	IncludePatterns []string
	// ExcludePatterns are glob patterns for namespaces to exclude
	ExcludePatterns []string
	// LabelSelector filters namespaces by labels
	LabelSelector string
	// IncludeSystemNamespaces includes system namespaces like kube-system
	IncludeSystemNamespaces bool
}

// NamespaceInfo contains information about a discovered namespace
type NamespaceInfo struct {
	Name   string
	Labels map[string]string
	// IsSystem indicates if this is a system namespace
	IsSystem bool
	// ResourceCount provides counts of key resources in the namespace
	ResourceCount ResourceCount
}

// ResourceCount tracks resource counts in a namespace
type ResourceCount struct {
	Pods        int
	Deployments int
	Services    int
	ConfigMaps  int
	Secrets     int
}

// ScanNamespaces discovers and returns information about accessible namespaces
func (ns *NamespaceScanner) ScanNamespaces(ctx context.Context, opts ScanOptions) ([]NamespaceInfo, error) {
	klog.V(2).Info("Starting namespace scan")

	// Get all namespaces the user can access
	namespaceList, err := ns.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: opts.LabelSelector,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to list namespaces")
	}

	var namespaceInfos []NamespaceInfo

	for _, namespace := range namespaceList.Items {
		// Check if namespace should be included
		if !ns.shouldIncludeNamespace(namespace.Name, namespace.Labels, opts) {
			klog.V(4).Infof("Excluding namespace %s based on filters", namespace.Name)
			continue
		}

		// Get resource counts for the namespace
		resourceCount, err := ns.getResourceCount(ctx, namespace.Name)
		if err != nil {
			klog.Warningf("Failed to get resource count for namespace %s: %v", namespace.Name, err)
			// Continue with empty resource count
			resourceCount = ResourceCount{}
		}

		namespaceInfo := NamespaceInfo{
			Name:          namespace.Name,
			Labels:        namespace.Labels,
			IsSystem:      ns.isSystemNamespace(namespace.Name),
			ResourceCount: resourceCount,
		}

		namespaceInfos = append(namespaceInfos, namespaceInfo)
	}

	klog.V(2).Infof("Namespace scan completed: found %d namespaces", len(namespaceInfos))
	return namespaceInfos, nil
}

// GetTargetNamespaces returns a list of namespace names to target for collection
func (ns *NamespaceScanner) GetTargetNamespaces(ctx context.Context, requestedNamespaces []string, opts ScanOptions) ([]string, error) {
	// If specific namespaces are requested, validate and return them
	if len(requestedNamespaces) > 0 {
		validNamespaces, err := ns.validateNamespaces(ctx, requestedNamespaces)
		if err != nil {
			return nil, errors.Wrap(err, "failed to validate requested namespaces")
		}
		return validNamespaces, nil
	}

	// Otherwise, scan and filter namespaces
	namespaceInfos, err := ns.ScanNamespaces(ctx, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to scan namespaces")
	}

	var targetNamespaces []string
	for _, nsInfo := range namespaceInfos {
		targetNamespaces = append(targetNamespaces, nsInfo.Name)
	}

	return targetNamespaces, nil
}

// shouldIncludeNamespace determines if a namespace should be included based on filters
func (ns *NamespaceScanner) shouldIncludeNamespace(name string, nsLabels map[string]string, opts ScanOptions) bool {
	// Check system namespace exclusion
	if !opts.IncludeSystemNamespaces && ns.isSystemNamespace(name) {
		return false
	}

	// Check exclude patterns first
	for _, pattern := range opts.ExcludePatterns {
		if ns.matchesPattern(name, pattern) {
			klog.V(4).Infof("Namespace %s excluded by pattern %s", name, pattern)
			return false
		}
	}

	// If include patterns are specified, namespace must match at least one
	if len(opts.IncludePatterns) > 0 {
		matched := false
		for _, pattern := range opts.IncludePatterns {
			if ns.matchesPattern(name, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			klog.V(4).Infof("Namespace %s does not match any include pattern", name)
			return false
		}
	}

	return true
}

// isSystemNamespace determines if a namespace is a system namespace
func (ns *NamespaceScanner) isSystemNamespace(name string) bool {
	systemNamespaces := []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"kubernetes-dashboard",
		"cattle-system",
		"rancher-system",
		"longhorn-system",
		"monitoring",
		"logging",
		"istio-system",
		"linkerd",
	}

	for _, sysNs := range systemNamespaces {
		if name == sysNs {
			return true
		}
	}

	// Also consider namespaces with common system prefixes
	systemPrefixes := []string{
		"kube-",
		"cattle-",
		"rancher-",
		"istio-",
		"linkerd-",
	}

	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	return false
}

// matchesPattern checks if a name matches a glob pattern (simplified implementation)
func (ns *NamespaceScanner) matchesPattern(name, pattern string) bool {
	// Simple pattern matching - support * wildcard
	if pattern == "*" {
		return true
	}

	if !strings.Contains(pattern, "*") {
		return name == pattern
	}

	// Handle patterns with * wildcards
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		// Pattern is "*substring*"
		substring := pattern[1 : len(pattern)-1]
		return strings.Contains(name, substring)
	}

	if strings.HasPrefix(pattern, "*") {
		// Pattern is "*suffix"
		suffix := pattern[1:]
		return strings.HasSuffix(name, suffix)
	}

	if strings.HasSuffix(pattern, "*") {
		// Pattern is "prefix*"
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(name, prefix)
	}

	// For more complex patterns, fall back to exact match
	return name == pattern
}

// getResourceCount counts key resources in a namespace
func (ns *NamespaceScanner) getResourceCount(ctx context.Context, namespace string) (ResourceCount, error) {
	count := ResourceCount{}

	// Count pods
	pods, err := ns.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningf("Failed to count pods in namespace %s: %v", namespace, err)
	} else {
		count.Pods = len(pods.Items)
	}

	// Count deployments
	deployments, err := ns.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningf("Failed to count deployments in namespace %s: %v", namespace, err)
	} else {
		count.Deployments = len(deployments.Items)
	}

	// Count services
	services, err := ns.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningf("Failed to count services in namespace %s: %v", namespace, err)
	} else {
		count.Services = len(services.Items)
	}

	// Count configmaps
	configmaps, err := ns.client.CoreV1().ConfigMaps(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningf("Failed to count configmaps in namespace %s: %v", namespace, err)
	} else {
		count.ConfigMaps = len(configmaps.Items)
	}

	// Count secrets
	secrets, err := ns.client.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Warningf("Failed to count secrets in namespace %s: %v", namespace, err)
	} else {
		count.Secrets = len(secrets.Items)
	}

	klog.V(4).Infof("Resource count for namespace %s: pods=%d, deployments=%d, services=%d, configmaps=%d, secrets=%d",
		namespace, count.Pods, count.Deployments, count.Services, count.ConfigMaps, count.Secrets)

	return count, nil
}

// validateNamespaces checks if the requested namespaces exist and are accessible
func (ns *NamespaceScanner) validateNamespaces(ctx context.Context, requestedNamespaces []string) ([]string, error) {
	var validNamespaces []string

	for _, nsName := range requestedNamespaces {
		_, err := ns.client.CoreV1().Namespaces().Get(ctx, nsName, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("Cannot access namespace %s: %v", nsName, err)
			continue
		}
		validNamespaces = append(validNamespaces, nsName)
	}

	if len(validNamespaces) == 0 {
		return nil, fmt.Errorf("none of the requested namespaces are accessible: %v", requestedNamespaces)
	}

	if len(validNamespaces) < len(requestedNamespaces) {
		klog.Warningf("Some requested namespaces are not accessible. Using: %v", validNamespaces)
	}

	return validNamespaces, nil
}

// FilterNamespacesByLabel filters namespaces using a label selector
func (ns *NamespaceScanner) FilterNamespacesByLabel(ctx context.Context, namespaces []string, labelSelector string) ([]string, error) {
	if labelSelector == "" {
		return namespaces, nil
	}

	// Parse label selector
	selector, err := labels.Parse(labelSelector)
	if err != nil {
		return nil, errors.Wrap(err, "invalid label selector")
	}

	var filteredNamespaces []string

	for _, nsName := range namespaces {
		namespace, err := ns.client.CoreV1().Namespaces().Get(ctx, nsName, metav1.GetOptions{})
		if err != nil {
			klog.Warningf("Cannot get namespace %s: %v", nsName, err)
			continue
		}

		if selector.Matches(labels.Set(namespace.Labels)) {
			filteredNamespaces = append(filteredNamespaces, nsName)
		}
	}

	return filteredNamespaces, nil
}

// GetNamespacesByResourceActivity returns namespaces sorted by resource activity
func (ns *NamespaceScanner) GetNamespacesByResourceActivity(ctx context.Context, opts ScanOptions) ([]NamespaceInfo, error) {
	namespaceInfos, err := ns.ScanNamespaces(ctx, opts)
	if err != nil {
		return nil, err
	}

	// Sort by total resource count (descending)
	for i := 0; i < len(namespaceInfos); i++ {
		for j := i + 1; j < len(namespaceInfos); j++ {
			countI := ns.getTotalResourceCount(namespaceInfos[i].ResourceCount)
			countJ := ns.getTotalResourceCount(namespaceInfos[j].ResourceCount)
			if countI < countJ {
				namespaceInfos[i], namespaceInfos[j] = namespaceInfos[j], namespaceInfos[i]
			}
		}
	}

	return namespaceInfos, nil
}

// getTotalResourceCount calculates the total resource count for a namespace
func (ns *NamespaceScanner) getTotalResourceCount(count ResourceCount) int {
	return count.Pods + count.Deployments + count.Services + count.ConfigMaps + count.Secrets
}
