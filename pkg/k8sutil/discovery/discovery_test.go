package discovery

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
