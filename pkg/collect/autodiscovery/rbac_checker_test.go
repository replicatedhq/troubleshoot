package autodiscovery

import (
	"context"
	"fmt"
	"testing"
	"time"

	authv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestNewRBACChecker(t *testing.T) {
	tests := []struct {
		name    string
		client  kubernetes.Interface
		wantErr bool
	}{
		{
			name:    "valid client",
			client:  fake.NewSimpleClientset(),
			wantErr: false,
		},
		{
			name:    "nil client",
			client:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker, err := NewRBACChecker(tt.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewRBACChecker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && checker == nil {
				t.Error("NewRBACChecker() returned nil checker")
			}
		})
	}
}

func TestRBACChecker_CheckPermission(t *testing.T) {
	tests := []struct {
		name          string
		resource      Resource
		allowed       bool
		setupReaction func(*fake.Clientset)
		wantErr       bool
	}{
		{
			name: "permission allowed",
			resource: Resource{
				APIVersion: "v1",
				Kind:       "Pod",
				Namespace:  "default",
				Name:       "*",
			},
			allowed: true,
			setupReaction: func(client *fake.Clientset) {
				client.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, &authv1.SelfSubjectAccessReview{
						Status: authv1.SubjectAccessReviewStatus{
							Allowed: true,
						},
					}, nil
				})
			},
			wantErr: false,
		},
		{
			name: "permission denied",
			resource: Resource{
				APIVersion: "v1",
				Kind:       "Secret",
				Namespace:  "kube-system",
				Name:       "*",
			},
			allowed: false,
			setupReaction: func(client *fake.Clientset) {
				client.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
					return true, &authv1.SelfSubjectAccessReview{
						Status: authv1.SubjectAccessReviewStatus{
							Allowed: false,
							Reason:  "access denied",
						},
					}, nil
				})
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			if tt.setupReaction != nil {
				tt.setupReaction(client)
			}

			checker, err := NewRBACChecker(client)
			if err != nil {
				t.Fatalf("Failed to create RBAC checker: %v", err)
			}

			ctx := context.Background()
			allowed, err := checker.CheckPermission(ctx, tt.resource)

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckPermission() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if allowed != tt.allowed {
				t.Errorf("CheckPermission() allowed = %v, want %v", allowed, tt.allowed)
			}
		})
	}
}

func TestRBACChecker_FilterByPermissions(t *testing.T) {
	client := fake.NewSimpleClientset()

	// Setup reactions to simulate RBAC responses
	client.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(k8stesting.CreateAction)
		review := createAction.GetObject().(*authv1.SelfSubjectAccessReview)

		// Simulate different permission responses based on resource type
		allowed := true
		if review.Spec.ResourceAttributes.Resource == "secrets" && review.Spec.ResourceAttributes.Namespace == "kube-system" {
			allowed = false // Deny access to kube-system secrets
		}

		return true, &authv1.SelfSubjectAccessReview{
			Status: authv1.SubjectAccessReviewStatus{
				Allowed: allowed,
			},
		}, nil
	})

	checker, err := NewRBACChecker(client)
	if err != nil {
		t.Fatalf("Failed to create RBAC checker: %v", err)
	}

	resources := []Resource{
		{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  "default",
			Name:       "*",
		},
		{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Namespace:  "default",
			Name:       "*",
		},
		{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  "kube-system",
			Name:       "*",
		},
		{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  "default",
			Name:       "*",
		},
	}

	ctx := context.Background()
	allowedResources, err := checker.FilterByPermissions(ctx, resources)
	if err != nil {
		t.Fatalf("FilterByPermissions() error = %v", err)
	}

	// Should have filtered out the kube-system secret
	expectedCount := 3
	if len(allowedResources) != expectedCount {
		t.Errorf("FilterByPermissions() returned %d resources, want %d", len(allowedResources), expectedCount)
	}

	// Verify that kube-system secret was filtered out
	for _, resource := range allowedResources {
		if resource.Kind == "Secret" && resource.Namespace == "kube-system" {
			t.Error("FilterByPermissions() should have filtered out kube-system secret")
		}
	}
}

func TestRBACChecker_getVerbForResource(t *testing.T) {
	checker := &RBACChecker{}

	tests := []struct {
		resource Resource
		wantVerb string
	}{
		{
			resource: Resource{Kind: "Pod"},
			wantVerb: "list",
		},
		{
			resource: Resource{Kind: "ConfigMap"},
			wantVerb: "get",
		},
		{
			resource: Resource{Kind: "Secret"},
			wantVerb: "get",
		},
		{
			resource: Resource{Kind: "Event"},
			wantVerb: "list",
		},
		{
			resource: Resource{Kind: "Node"},
			wantVerb: "list",
		},
		{
			resource: Resource{Kind: "UnknownResource"},
			wantVerb: "get",
		},
	}

	for _, tt := range tests {
		t.Run(tt.resource.Kind, func(t *testing.T) {
			verb := checker.getVerbForResource(tt.resource)
			if verb != tt.wantVerb {
				t.Errorf("getVerbForResource() = %v, want %v", verb, tt.wantVerb)
			}
		})
	}
}

func TestRBACChecker_getAPIGroup(t *testing.T) {
	checker := &RBACChecker{}

	tests := []struct {
		apiVersion string
		wantGroup  string
	}{
		{
			apiVersion: "v1",
			wantGroup:  "",
		},
		{
			apiVersion: "apps/v1",
			wantGroup:  "apps",
		},
		{
			apiVersion: "extensions/v1beta1",
			wantGroup:  "extensions",
		},
		{
			apiVersion: "apiextensions.k8s.io/v1",
			wantGroup:  "apiextensions.k8s.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.apiVersion, func(t *testing.T) {
			group := checker.getAPIGroup(tt.apiVersion)
			if group != tt.wantGroup {
				t.Errorf("getAPIGroup() = %v, want %v", group, tt.wantGroup)
			}
		})
	}
}

func TestRBACChecker_getAPIVersion(t *testing.T) {
	checker := &RBACChecker{}

	tests := []struct {
		apiVersion  string
		wantVersion string
	}{
		{
			apiVersion:  "v1",
			wantVersion: "v1",
		},
		{
			apiVersion:  "apps/v1",
			wantVersion: "v1",
		},
		{
			apiVersion:  "extensions/v1beta1",
			wantVersion: "v1beta1",
		},
		{
			apiVersion:  "apiextensions.k8s.io/v1",
			wantVersion: "v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.apiVersion, func(t *testing.T) {
			version := checker.getAPIVersion(tt.apiVersion)
			if version != tt.wantVersion {
				t.Errorf("getAPIVersion() = %v, want %v", version, tt.wantVersion)
			}
		})
	}
}

func TestRBACChecker_getResourceName(t *testing.T) {
	checker := &RBACChecker{}

	tests := []struct {
		kind         string
		wantResource string
	}{
		{
			kind:         "Pod",
			wantResource: "pods",
		},
		{
			kind:         "ConfigMap",
			wantResource: "configmaps",
		},
		{
			kind:         "Secret",
			wantResource: "secrets",
		},
		{
			kind:         "Event",
			wantResource: "events",
		},
		{
			kind:         "Node",
			wantResource: "nodes",
		},
		{
			kind:         "Deployment",
			wantResource: "deployments",
		},
		{
			kind:         "Service",
			wantResource: "services",
		},
		{
			kind:         "UnknownKind",
			wantResource: "UnknownKinds", // Default fallback
		},
	}

	for _, tt := range tests {
		t.Run(tt.kind, func(t *testing.T) {
			resource := checker.getResourceName(tt.kind)
			if resource != tt.wantResource {
				t.Errorf("getResourceName() = %v, want %v", resource, tt.wantResource)
			}
		})
	}
}

func TestPermissionCache(t *testing.T) {
	cache := &permissionCache{
		entries: make(map[string]permissionCacheEntry),
		ttl:     100 * time.Millisecond, // Short TTL for testing
	}

	key := "test-resource"

	// Test cache miss
	_, found := cache.get(key)
	if found {
		t.Error("Cache should initially be empty")
	}

	// Test cache set and hit
	cache.set(key, true)
	allowed, found := cache.get(key)
	if !found {
		t.Error("Cache should contain the set key")
	}
	if !allowed {
		t.Error("Cache should return the correct value")
	}

	// Test cache expiration
	time.Sleep(150 * time.Millisecond) // Wait for expiration
	_, found = cache.get(key)
	if found {
		t.Error("Cache entry should have expired")
	}
}

func TestRBACChecker_CheckBulkPermissions(t *testing.T) {
	client := fake.NewSimpleClientset()

	// Setup reaction to simulate RBAC responses
	client.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		createAction := action.(k8stesting.CreateAction)
		review := createAction.GetObject().(*authv1.SelfSubjectAccessReview)

		// Allow pods, deny secrets in kube-system
		allowed := true
		if review.Spec.ResourceAttributes.Resource == "secrets" && review.Spec.ResourceAttributes.Namespace == "kube-system" {
			allowed = false
		}

		return true, &authv1.SelfSubjectAccessReview{
			Status: authv1.SubjectAccessReviewStatus{
				Allowed: allowed,
			},
		}, nil
	})

	checker, err := NewRBACChecker(client)
	if err != nil {
		t.Fatalf("Failed to create RBAC checker: %v", err)
	}

	resources := []Resource{
		{APIVersion: "v1", Kind: "Pod", Namespace: "default", Name: "*"},
		{APIVersion: "v1", Kind: "Secret", Namespace: "kube-system", Name: "*"},
		{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "*"},
	}

	ctx := context.Background()
	results, err := checker.CheckBulkPermissions(ctx, resources)
	if err != nil {
		t.Fatalf("CheckBulkPermissions() error = %v", err)
	}

	if len(results) != len(resources) {
		t.Errorf("CheckBulkPermissions() returned %d results, want %d", len(results), len(resources))
	}

	// Check specific results
	podKey := "v1/Pod/default/*"
	secretKey := "v1/Secret/kube-system/*"
	configMapKey := "v1/ConfigMap/default/*"

	if !results[podKey] {
		t.Error("Pod permission should be allowed")
	}
	if results[secretKey] {
		t.Error("kube-system secret permission should be denied")
	}
	if !results[configMapKey] {
		t.Error("ConfigMap permission should be allowed")
	}
}

func BenchmarkRBACChecker_CheckPermission(b *testing.B) {
	client := fake.NewSimpleClientset()

	client.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authv1.SelfSubjectAccessReview{
			Status: authv1.SubjectAccessReviewStatus{
				Allowed: true,
			},
		}, nil
	})

	checker, err := NewRBACChecker(client)
	if err != nil {
		b.Fatalf("Failed to create RBAC checker: %v", err)
	}

	resource := Resource{
		APIVersion: "v1",
		Kind:       "Pod",
		Namespace:  "default",
		Name:       "*",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := checker.CheckPermission(context.Background(), resource)
		if err != nil {
			b.Fatalf("CheckPermission failed: %v", err)
		}
	}
}

func BenchmarkRBACChecker_FilterByPermissions(b *testing.B) {
	client := fake.NewSimpleClientset()

	client.PrependReactor("create", "selfsubjectaccessreviews", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, &authv1.SelfSubjectAccessReview{
			Status: authv1.SubjectAccessReviewStatus{
				Allowed: true,
			},
		}, nil
	})

	checker, err := NewRBACChecker(client)
	if err != nil {
		b.Fatalf("Failed to create RBAC checker: %v", err)
	}

	// Create a large set of resources for benchmarking
	var resources []Resource
	for i := 0; i < 50; i++ {
		resources = append(resources, Resource{
			APIVersion: "v1",
			Kind:       "Pod",
			Namespace:  fmt.Sprintf("namespace-%d", i),
			Name:       "*",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := checker.FilterByPermissions(context.Background(), resources)
		if err != nil {
			b.Fatalf("FilterByPermissions failed: %v", err)
		}
	}
}
