package autodiscovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// RBACChecker handles RBAC permission validation
type RBACChecker struct {
	client kubernetes.Interface
	cache  *permissionCache
}

// permissionCache caches RBAC check results to avoid repeated API calls
type permissionCache struct {
	mu      sync.RWMutex
	entries map[string]permissionCacheEntry
	ttl     time.Duration
}

type permissionCacheEntry struct {
	allowed   bool
	timestamp time.Time
}

// NewRBACChecker creates a new RBAC checker
func NewRBACChecker(client kubernetes.Interface) (*RBACChecker, error) {
	if client == nil {
		return nil, errors.New("kubernetes client is required")
	}

	cache := &permissionCache{
		entries: make(map[string]permissionCacheEntry),
		ttl:     5 * time.Minute, // Cache permissions for 5 minutes
	}

	return &RBACChecker{
		client: client,
		cache:  cache,
	}, nil
}

// FilterByPermissions filters resources based on RBAC permissions
func (r *RBACChecker) FilterByPermissions(ctx context.Context, resources []Resource) ([]Resource, error) {
	klog.V(3).Infof("Checking RBAC permissions for %d resources", len(resources))

	var allowedResources []Resource
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Check permissions concurrently for better performance
	semaphore := make(chan struct{}, 10) // Limit concurrent checks to 10

	for _, resource := range resources {
		wg.Add(1)
		go func(res Resource) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			allowed, err := r.CheckPermission(ctx, res)
			if err != nil {
				klog.Warningf("Permission check failed for %s/%s in namespace %s: %v", 
					res.APIVersion, res.Kind, res.Namespace, err)
				// On error, be permissive and allow the resource
				allowed = true
			}

			if allowed {
				mu.Lock()
				allowedResources = append(allowedResources, res)
				mu.Unlock()
			} else {
				klog.V(4).Infof("Access denied for resource %s/%s in namespace %s", 
					res.APIVersion, res.Kind, res.Namespace)
			}
		}(resource)
	}

	wg.Wait()

	klog.V(3).Infof("RBAC filtering result: %d/%d resources allowed", len(allowedResources), len(resources))
	return allowedResources, nil
}

// CheckPermission checks if the current user has permission to access a specific resource
func (r *RBACChecker) CheckPermission(ctx context.Context, resource Resource) (bool, error) {
	cacheKey := r.getCacheKey(resource)

	// Check cache first
	if allowed, found := r.cache.get(cacheKey); found {
		klog.V(5).Infof("Permission cache hit for %s", cacheKey)
		return allowed, nil
	}

	// Determine the verb based on resource type
	verb := r.getVerbForResource(resource)

	// Create SelfSubjectAccessReview
	review := &authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Namespace: resource.Namespace,
				Verb:      verb,
				Group:     r.getAPIGroup(resource.APIVersion),
				Version:   r.getAPIVersion(resource.APIVersion),
				Resource:  r.getResourceName(resource.Kind),
				Name:      resource.Name,
			},
		},
	}

	// Perform the access review
	result, err := r.client.AuthorizationV1().SelfSubjectAccessReviews().Create(
		ctx, review, metav1.CreateOptions{},
	)
	if err != nil {
		return false, errors.Wrap(err, "failed to check RBAC permissions")
	}

	allowed := result.Status.Allowed

	// Cache the result
	r.cache.set(cacheKey, allowed)

	klog.V(4).Infof("RBAC check for %s: allowed=%t (reason: %s)", 
		cacheKey, allowed, result.Status.Reason)

	return allowed, nil
}

// CheckBulkPermissions checks multiple permissions efficiently using batch operations
func (r *RBACChecker) CheckBulkPermissions(ctx context.Context, resources []Resource) (map[string]bool, error) {
	results := make(map[string]bool)
	
	for _, resource := range resources {
		allowed, err := r.CheckPermission(ctx, resource)
		if err != nil {
			klog.Warningf("Permission check failed for %s: %v", r.getCacheKey(resource), err)
			// Be permissive on error
			allowed = true
		}
		
		key := r.getCacheKey(resource)
		results[key] = allowed
	}

	return results, nil
}

// getCacheKey generates a cache key for a resource
func (r *RBACChecker) getCacheKey(resource Resource) string {
	return fmt.Sprintf("%s/%s/%s/%s", 
		resource.APIVersion, resource.Kind, resource.Namespace, resource.Name)
}

// getVerbForResource determines the appropriate RBAC verb for a resource type
func (r *RBACChecker) getVerbForResource(resource Resource) string {
	// Most collection operations require 'get' and 'list' permissions
	// We check for 'list' as it's usually more restrictive
	switch resource.Kind {
	case "Pod":
		return "list" // Need to list pods to collect logs
	case "ConfigMap", "Secret":
		return "get" // Individual configmaps/secrets
	case "Event":
		return "list" // Need to list events
	case "Node":
		return "list" // Cluster info requires listing nodes
	default:
		return "get"
	}
}

// getAPIGroup extracts the API group from APIVersion
func (r *RBACChecker) getAPIGroup(apiVersion string) string {
	if apiVersion == "v1" {
		return "" // Core API group is empty string
	}
	
	// Split "group/version" format
	parts := make([]string, 2)
	for i, part := range []string{apiVersion} {
		if i == 0 && len(part) > 0 {
			// Handle "group/version" or just "version"
			if groupVersion := part; len(groupVersion) > 0 {
				if slash := 0; slash < len(groupVersion) {
					for j, c := range groupVersion {
						if c == '/' {
							parts[0] = groupVersion[:j]
							parts[1] = groupVersion[j+1:]
							break
						}
					}
					if parts[0] == "" {
						parts[0] = groupVersion
					}
				}
			}
		}
	}
	
	return parts[0]
}

// getAPIVersion extracts the version from APIVersion
func (r *RBACChecker) getAPIVersion(apiVersion string) string {
	if apiVersion == "v1" {
		return "v1"
	}
	
	// For "group/version" format, return the version part
	parts := make([]string, 2)
	if slash := -1; slash < len(apiVersion) {
		for i, c := range apiVersion {
			if c == '/' {
				slash = i
				break
			}
		}
		if slash >= 0 {
			parts[1] = apiVersion[slash+1:]
			return parts[1]
		}
	}
	
	return apiVersion
}

// getResourceName converts a Kind to the appropriate resource name for RBAC
func (r *RBACChecker) getResourceName(kind string) string {
	// Convert Kind to plural resource name (simplified mapping)
	switch kind {
	case "Pod":
		return "pods"
	case "ConfigMap":
		return "configmaps"
	case "Secret":
		return "secrets"
	case "Event":
		return "events"
	case "Node":
		return "nodes"
	case "Deployment":
		return "deployments"
	case "Service":
		return "services"
	case "ReplicaSet":
		return "replicasets"
	default:
		// Default to lowercased kind + "s" (not always correct, but reasonable fallback)
		return fmt.Sprintf("%ss", kind)
	}
}

// get retrieves a cached permission result
func (pc *permissionCache) get(key string) (bool, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	entry, exists := pc.entries[key]
	if !exists {
		return false, false
	}

	// Check if the entry has expired
	if time.Since(entry.timestamp) > pc.ttl {
		return false, false
	}

	return entry.allowed, true
}

// set stores a permission result in the cache
func (pc *permissionCache) set(key string, allowed bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.entries[key] = permissionCacheEntry{
		allowed:   allowed,
		timestamp: time.Now(),
	}

	// Clean up expired entries periodically (simple cleanup)
	if len(pc.entries) > 1000 {
		pc.cleanup()
	}
}

// cleanup removes expired cache entries
func (pc *permissionCache) cleanup() {
	now := time.Now()
	for key, entry := range pc.entries {
		if now.Sub(entry.timestamp) > pc.ttl {
			delete(pc.entries, key)
		}
	}
}
