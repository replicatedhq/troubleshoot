package preflight

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

// TestCollectWithContext_ClusterResourcesFirst verifies that clusterResources
// collector runs first, even when it's not first in the spec.
func TestCollectWithContext_ClusterResourcesFirst(t *testing.T) {
	// Create a preflight spec with collectors in a specific order
	// where clusterResources is NOT first
	preflight := &troubleshootv1beta2.Preflight{
		Spec: troubleshootv1beta2.PreflightSpec{
			Collectors: []*troubleshootv1beta2.Collect{
				{
					Data: &troubleshootv1beta2.Data{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "test-data",
						},
						Name: "test.json",
						Data: `{"test": "data"}`,
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{},
				},
			},
		},
	}

	// Use a fake Kubernetes client to avoid network calls
	fakeClient := fake.NewSimpleClientset()
	restConfig := &rest.Config{
		Host: "https://fake-host",
	}

	opts := CollectOpts{
		Namespace:              "default",
		KubernetesRestConfig:   restConfig,
		ProgressChan:           make(chan interface{}, 100),
		BundlePath:             t.TempDir(),
		IgnorePermissionErrors: true, // Ignore RBAC errors in tests
	}

	// Manually test the ordering logic by simulating what CollectWithContext does
	collectSpecs := make([]*troubleshootv1beta2.Collect, 0)
	if preflight.Spec.Collectors != nil {
		collectSpecs = append(collectSpecs, preflight.Spec.Collectors...)
	}
	collectSpecs = collect.EnsureCollectorInList(
		collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}},
	)
	collectSpecs = collect.EnsureCollectorInList(
		collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}},
	)
	collectSpecs = collect.DedupCollectors(collectSpecs)
	collectSpecs = collect.EnsureClusterResourcesFirst(collectSpecs)

	// Verify clusterResources is first in the specs
	require.NotEmpty(t, collectSpecs, "should have collectors")
	require.NotNil(t, collectSpecs[0].ClusterResources, "first collector should be clusterResources")

	// Now simulate the map grouping and order preservation
	allCollectorsMap := make(map[reflect.Type][]collect.Collector)
	collectorTypeOrder := make([]reflect.Type, 0)

	for _, desiredCollector := range collectSpecs {
		if collectorInterface, ok := collect.GetCollector(desiredCollector, opts.BundlePath, opts.Namespace, opts.KubernetesRestConfig, fakeClient, nil); ok {
			if collector, ok := collectorInterface.(collect.Collector); ok {
				// Skip RBAC check for this unit test
				collectorType := reflect.TypeOf(collector)
				if _, exists := allCollectorsMap[collectorType]; !exists {
					collectorTypeOrder = append(collectorTypeOrder, collectorType)
				}
				allCollectorsMap[collectorType] = append(allCollectorsMap[collectorType], collector)
			}
		}
	}

	// Verify that clusterResources type is first in the order
	require.NotEmpty(t, collectorTypeOrder, "should have collector types")

	// Find the clusterResources type by checking the actual collectors
	var clusterResourcesType reflect.Type
	for collectorType, collectors := range allCollectorsMap {
		if len(collectors) > 0 {
			if _, ok := collectors[0].(*collect.CollectClusterResources); ok {
				clusterResourcesType = collectorType
				break
			}
		}
	}
	require.NotNil(t, clusterResourcesType, "should find clusterResources type")
	assert.Equal(t, clusterResourcesType, collectorTypeOrder[0], "clusterResources type should be first in collectorTypeOrder")
}

// TestCollectWithContext_PreservesOrderAfterClusterResources verifies that
// after clusterResources, other collectors maintain their relative order.
func TestCollectWithContext_PreservesOrderAfterClusterResources(t *testing.T) {
	// Create a preflight spec with multiple collectors in a specific order
	preflight := &troubleshootv1beta2.Preflight{
		Spec: troubleshootv1beta2.PreflightSpec{
			Collectors: []*troubleshootv1beta2.Collect{
				{
					Data: &troubleshootv1beta2.Data{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "data-first",
						},
						Name: "first.json",
						Data: `{"first": "data"}`,
					},
				},
				{
					Secret: &troubleshootv1beta2.Secret{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "secret-second",
						},
					},
				},
			},
		},
	}

	// Use a fake Kubernetes client
	fakeClient := fake.NewSimpleClientset()
	restConfig := &rest.Config{
		Host: "https://fake-host",
	}

	opts := CollectOpts{
		Namespace:              "default",
		KubernetesRestConfig:   restConfig,
		ProgressChan:           make(chan interface{}, 100),
		BundlePath:             t.TempDir(),
		IgnorePermissionErrors: true,
	}

	// Simulate the ordering logic
	collectSpecs := make([]*troubleshootv1beta2.Collect, 0)
	if preflight.Spec.Collectors != nil {
		collectSpecs = append(collectSpecs, preflight.Spec.Collectors...)
	}
	collectSpecs = collect.EnsureCollectorInList(
		collectSpecs, troubleshootv1beta2.Collect{ClusterInfo: &troubleshootv1beta2.ClusterInfo{}},
	)
	collectSpecs = collect.EnsureCollectorInList(
		collectSpecs, troubleshootv1beta2.Collect{ClusterResources: &troubleshootv1beta2.ClusterResources{}},
	)
	collectSpecs = collect.DedupCollectors(collectSpecs)
	collectSpecs = collect.EnsureClusterResourcesFirst(collectSpecs)

	// Group collectors by type and track order
	allCollectorsMap := make(map[reflect.Type][]collect.Collector)
	collectorTypeOrder := make([]reflect.Type, 0)

	for _, desiredCollector := range collectSpecs {
		if collectorInterface, ok := collect.GetCollector(desiredCollector, opts.BundlePath, opts.Namespace, opts.KubernetesRestConfig, fakeClient, nil); ok {
			if collector, ok := collectorInterface.(collect.Collector); ok {
				collectorType := reflect.TypeOf(collector)
				if _, exists := allCollectorsMap[collectorType]; !exists {
					collectorTypeOrder = append(collectorTypeOrder, collectorType)
				}
				allCollectorsMap[collectorType] = append(allCollectorsMap[collectorType], collector)
			}
		}
	}

	// Verify clusterResources is first
	require.NotEmpty(t, collectorTypeOrder, "should have collector types")

	// Find the actual types from the collectors
	var clusterResourcesType, dataType, secretType reflect.Type
	for collectorType, collectors := range allCollectorsMap {
		if len(collectors) > 0 {
			switch collectors[0].(type) {
			case *collect.CollectClusterResources:
				clusterResourcesType = collectorType
			case *collect.CollectData:
				dataType = collectorType
			case *collect.CollectSecret:
				secretType = collectorType
			}
		}
	}

	require.NotNil(t, clusterResourcesType, "should find clusterResources type")
	assert.Equal(t, clusterResourcesType, collectorTypeOrder[0], "clusterResources should be first")

	dataIndex := -1
	secretIndex := -1
	for i, ct := range collectorTypeOrder {
		if ct == dataType {
			dataIndex = i
		}
		if ct == secretType {
			secretIndex = i
		}
	}

	if dataIndex >= 0 && secretIndex >= 0 {
		assert.Less(t, dataIndex, secretIndex, "data collectors should come before secret collectors, preserving relative order")
	}
}

// TestCollectHostWithContext_RedactsSensitiveEnvValues verifies that a `run`
// host collector's captured environment is redacted before being written to
// the bundle. CollectHostWithContext is the only collection path (cluster,
// host, remote) that skipped redaction entirely: every env var passed to the
// command -- including credentials embedded in a proxy URL -- landed
// verbatim in <collectorName>-info.json.
func TestCollectHostWithContext_RedactsSensitiveEnvValues(t *testing.T) {
	bundlePath := t.TempDir()

	hostPreflight := &troubleshootv1beta2.HostPreflight{
		Spec: troubleshootv1beta2.HostPreflightSpec{
			Collectors: []*troubleshootv1beta2.HostCollect{
				{
					HostRun: &troubleshootv1beta2.HostRun{
						HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
							CollectorName: "proxy-credential-check",
						},
						Command:          "sh",
						Args:             []string{"-c", "echo ok"},
						IgnoreParentEnvs: true,
						Env: []string{
							"HTTPS_PROXY=http://alice:sw0rdfish@proxy.example.com:3128",
						},
					},
				},
			},
		},
	}

	_, err := CollectHostWithContext(context.Background(), CollectOpts{
		ProgressChan: make(chan interface{}, 100),
		BundlePath:   bundlePath,
	}, hostPreflight)
	require.NoError(t, err)

	infoPath := filepath.Join(bundlePath, "host-collectors/run-host/proxy-credential-check-info.json")
	b, err := os.ReadFile(infoPath)
	require.NoError(t, err)

	assert.NotContains(t, string(b), "sw0rdfish", "proxy credential leaked unredacted into the host preflight bundle")
	assert.Contains(t, string(b), "***HIDDEN***", "expected the built-in URL-userinfo redactor to mask the credential")
}
