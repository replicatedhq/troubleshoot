package discovery

import (
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
)

func TestHasResource(t *testing.T) {
	testKind := "Foo"
	testKindGroupVersion := "v1"

	testcases := []struct {
		name            string
		apiResourceList []*metav1.APIResourceList
		wantResult      bool
	}{
		{
			name:            "empty api resource list",
			apiResourceList: []*metav1.APIResourceList{},
			wantResult:      false,
		},
		{
			name: "have resource",
			apiResourceList: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Kind: "Foo",
						},
						{
							Kind: "Bar",
						},
					},
				},
			},
			wantResult: true,
		},
		{
			name: "have resource but different version",
			apiResourceList: []*metav1.APIResourceList{
				{
					GroupVersion: "v2",
					APIResources: []metav1.APIResource{
						{
							Kind: "Foo",
						},
						{
							Kind: "Bar",
						},
					},
				},
			},
			wantResult: false,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := fakeclientset.NewSimpleClientset()
			fakeDiscovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
			if !ok {
				t.Fatalf("could not convert Discovery() to *FakeDiscovery")
			}
			fakeDiscovery.Resources = tc.apiResourceList

			exists, err := HasResource(fakeDiscovery, testKindGroupVersion, testKind)
			if err != nil {
				t.Fatalf("failed to check if resource exists: %v", err)
			}
			if exists != tc.wantResult {
				t.Errorf("unexpected result for HasResource:\n\t(WNT) %t\n\t(GOT) %t", tc.wantResult, exists)
			}
		})
	}
}

// TestHasResourceWithPartialDiscoveryFailure verifies that HasResource correctly handles
// partial discovery failures where ServerGroupsAndResources() returns both an error AND
// partial results (non-nil apiLists). This simulates real Kubernetes behavior when some
// API groups fail to load but others succeed.
func TestHasResourceWithPartialDiscoveryFailure(t *testing.T) {
	testKind := "Foo"
	testKindGroupVersion := "v1"

	testcases := []struct {
		name            string
		apiResourceList []*metav1.APIResourceList
		discoveryError  error
		wantResult      bool
		wantError       bool
		description     string
	}{
		{
			name: "resource found in partial results with discovery error",
			apiResourceList: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Kind: "Foo",
						},
						{
							Kind: "Bar",
						},
					},
				},
			},
			discoveryError: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "apps", Version: "v1"}: errors.New("failed to retrieve apps/v1"),
				},
			},
			wantResult:  true,
			wantError:   false,
			description: "Should return (true, nil) when resource exists in partial results despite discovery error",
		},
		{
			name: "resource not found in partial results with discovery error",
			apiResourceList: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Kind: "Bar",
						},
						{
							Kind: "Baz",
						},
					},
				},
			},
			discoveryError: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "apps", Version: "v1"}: errors.New("failed to retrieve apps/v1"),
				},
			},
			wantResult:  false,
			wantError:   true,
			description: "Should return (false, error) when resource not in partial results and discovery error exists",
		},
		{
			name:            "nil api resource list with discovery error",
			apiResourceList: nil,
			discoveryError: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "apps", Version: "v1"}: errors.New("failed to retrieve apps/v1"),
				},
			},
			wantResult:  false,
			wantError:   true,
			description: "Should return (false, error) when apiLists is nil and discovery error exists",
		},
		{
			name:            "empty api resource list with discovery error",
			apiResourceList: []*metav1.APIResourceList{},
			discoveryError: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "apps", Version: "v1"}: errors.New("failed to retrieve apps/v1"),
				},
			},
			wantResult:  false,
			wantError:   true,
			description: "Should return (false, error) when apiLists is empty and discovery error exists",
		},
		{
			name: "multiple groups with partial results and discovery error",
			apiResourceList: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Kind: "Pod",
						},
						{
							Kind: "Service",
						},
					},
				},
				{
					GroupVersion: "v2",
					APIResources: []metav1.APIResource{
						{
							Kind: "Foo",
						},
					},
				},
			},
			discoveryError: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "apps", Version: "v1"}:       errors.New("failed to retrieve apps/v1"),
					{Group: "batch", Version: "v1beta1"}: errors.New("failed to retrieve batch/v1beta1"),
				},
			},
			wantResult:  false,
			wantError:   true,
			description: "Should return (false, error) when resource not found across multiple partial groups with discovery error",
		},
		{
			name: "resource found with different version in partial results with discovery error",
			apiResourceList: []*metav1.APIResourceList{
				{
					GroupVersion: "v2",
					APIResources: []metav1.APIResource{
						{
							Kind: "Foo",
						},
					},
				},
			},
			discoveryError: &discovery.ErrGroupDiscoveryFailed{
				Groups: map[schema.GroupVersion]error{
					{Group: "apps", Version: "v1"}: errors.New("failed to retrieve apps/v1"),
				},
			},
			wantResult:  false,
			wantError:   true,
			description: "Should return (false, error) when resource exists with different version in partial results",
		},
		{
			name: "generic error with partial results containing resource",
			apiResourceList: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Kind: "Foo",
						},
					},
				},
			},
			discoveryError: errors.New("connection timeout"),
			wantResult:     true,
			wantError:      false,
			description:    "Should return (true, nil) when resource exists in partial results even with generic error",
		},
		{
			name: "generic error without resource in partial results",
			apiResourceList: []*metav1.APIResourceList{
				{
					GroupVersion: "v1",
					APIResources: []metav1.APIResource{
						{
							Kind: "Bar",
						},
					},
				},
			},
			discoveryError: errors.New("connection timeout"),
			wantResult:     false,
			wantError:      true,
			description:    "Should return (false, error) when resource not in partial results with generic error",
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := fakeclientset.NewSimpleClientset()
			fakeDiscovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
			if !ok {
				t.Fatalf("could not convert Discovery() to *FakeDiscovery")
			}

			// Configure the fake discovery to return both resources and error
			fakeDiscovery.Resources = tc.apiResourceList

			// Create a mock discovery interface that returns both error and partial results
			mockDiscovery := &mockDiscoveryWithPartialFailure{
				FakeDiscovery: fakeDiscovery,
				errorToReturn: tc.discoveryError,
			}

			exists, err := HasResource(mockDiscovery, testKindGroupVersion, testKind)

			// Verify error expectation
			if tc.wantError && err == nil {
				t.Errorf("%s: expected error but got nil", tc.description)
			}
			if !tc.wantError && err != nil {
				t.Errorf("%s: expected no error but got: %v", tc.description, err)
			}

			// Verify result expectation
			if exists != tc.wantResult {
				t.Errorf("%s: unexpected result for HasResource:\n\t(WANT) %t\n\t(GOT) %t", tc.description, tc.wantResult, exists)
			}
		})
	}
}

// mockDiscoveryWithPartialFailure wraps FakeDiscovery to simulate partial discovery failures
// where ServerGroupsAndResources() returns both an error AND partial results.
type mockDiscoveryWithPartialFailure struct {
	*fakediscovery.FakeDiscovery
	errorToReturn error
}

// ServerGroupsAndResources simulates the Kubernetes API behavior where partial results
// can be returned even when an error occurs. This happens when some API groups fail to
// load but others succeed.
func (m *mockDiscoveryWithPartialFailure) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	groups, resources, _ := m.FakeDiscovery.ServerGroupsAndResources()
	return groups, resources, m.errorToReturn
}
