package autodiscovery

import (
	"context"
	"fmt"
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

func TestNewDiscoverer(t *testing.T) {
	tests := []struct {
		name         string
		clientConfig *rest.Config
		client       kubernetes.Interface
		wantErr      bool
	}{
		{
			name:         "valid parameters",
			clientConfig: &rest.Config{},
			client:       fake.NewSimpleClientset(),
			wantErr:      false,
		},
		{
			name:         "nil client config",
			clientConfig: nil,
			client:       fake.NewSimpleClientset(),
			wantErr:      true,
		},
		{
			name:         "nil client",
			clientConfig: &rest.Config{},
			client:       nil,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			discoverer, err := NewDiscoverer(tt.clientConfig, tt.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewDiscoverer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && discoverer == nil {
				t.Error("NewDiscoverer() returned nil discoverer")
			}
		})
	}
}

func TestDiscoverer_DiscoverFoundational(t *testing.T) {
	// Create fake client with test data
	client := fake.NewSimpleClientset()

	// Add test namespaces
	testNamespaces := []corev1.Namespace{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "default",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-app",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kube-system",
			},
		},
	}

	for _, ns := range testNamespaces {
		client.CoreV1().Namespaces().Create(context.Background(), &ns, metav1.CreateOptions{})
	}

	discoverer, err := NewDiscoverer(&rest.Config{}, client)
	if err != nil {
		t.Fatalf("Failed to create discoverer: %v", err)
	}

	tests := []struct {
		name               string
		opts               DiscoveryOptions
		wantCollectorTypes map[CollectorType]int // type -> expected count
		wantMinCollectors  int
		wantErr            bool
	}{
		{
			name: "default options",
			opts: DiscoveryOptions{
				Namespaces:    []string{"default"},
				IncludeImages: false,
				RBACCheck:     false,
				Timeout:       10 * time.Second,
			},
			wantCollectorTypes: map[CollectorType]int{
				CollectorTypeClusterInfo:      1,
				CollectorTypeClusterResources: 1,
				CollectorTypeLogs:             1,
				CollectorTypeConfigMaps:       1,
				CollectorTypeSecrets:          1,
			},
			wantMinCollectors: 5,
			wantErr:           false,
		},
		{
			name: "with images",
			opts: DiscoveryOptions{
				Namespaces:    []string{"test-app"},
				IncludeImages: true,
				RBACCheck:     false,
				Timeout:       10 * time.Second,
			},
			wantCollectorTypes: map[CollectorType]int{
				CollectorTypeClusterInfo:      1,
				CollectorTypeClusterResources: 1,
				CollectorTypeLogs:             1,
				CollectorTypeConfigMaps:       1,
				CollectorTypeSecrets:          1,
				CollectorTypeImageFacts:       1,
			},
			wantMinCollectors: 6,
			wantErr:           false,
		},
		{
			name: "multiple namespaces",
			opts: DiscoveryOptions{
				Namespaces:    []string{"default", "test-app"},
				IncludeImages: false,
				RBACCheck:     false,
				Timeout:       10 * time.Second,
			},
			wantMinCollectors: 8, // 2 cluster + 3*2 namespace collectors
			wantErr:           false,
		},
		{
			name: "no namespaces specified",
			opts: DiscoveryOptions{
				Namespaces:    []string{},
				IncludeImages: false,
				RBACCheck:     false,
				Timeout:       10 * time.Second,
			},
			wantMinCollectors: 2, // At least cluster collectors
			wantErr:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			collectors, err := discoverer.DiscoverFoundational(ctx, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("DiscoverFoundational() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if len(collectors) < tt.wantMinCollectors {
					t.Errorf("DiscoverFoundational() returned %d collectors, want at least %d",
						len(collectors), tt.wantMinCollectors)
				}

				// Check collector types if specified
				if tt.wantCollectorTypes != nil {
					collectorCounts := make(map[CollectorType]int)
					for _, collector := range collectors {
						collectorCounts[collector.Type]++
					}

					for expectedType, expectedCount := range tt.wantCollectorTypes {
						if collectorCounts[expectedType] != expectedCount {
							t.Errorf("DiscoverFoundational() got %d collectors of type %s, want %d",
								collectorCounts[expectedType], expectedType, expectedCount)
						}
					}
				}

				// Verify all collectors have foundational source
				for _, collector := range collectors {
					if collector.Source != SourceFoundational {
						t.Errorf("DiscoverFoundational() collector %s has source %s, want %s",
							collector.Name, collector.Source, SourceFoundational)
					}
				}
			}
		})
	}
}

func TestDiscoverer_AugmentWithFoundational(t *testing.T) {
	client := fake.NewSimpleClientset()

	// Add test namespace
	client.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "test-app"},
	}, metav1.CreateOptions{})

	discoverer, err := NewDiscoverer(&rest.Config{}, client)
	if err != nil {
		t.Fatalf("Failed to create discoverer: %v", err)
	}

	tests := []struct {
		name           string
		yamlCollectors []CollectorSpec
		opts           DiscoveryOptions
		wantMinCount   int
		wantErr        bool
	}{
		{
			name: "augment with yaml collectors",
			yamlCollectors: []CollectorSpec{
				{
					Type:      CollectorTypeLogs,
					Name:      "custom-logs",
					Namespace: "test-app",
					Spec:      &troubleshootv1beta2.Logs{},
					Priority:  100,
					Source:    SourceYAML,
				},
			},
			opts: DiscoveryOptions{
				Namespaces: []string{"test-app"},
				RBACCheck:  false,
			},
			wantMinCount: 5, // Should have foundational + yaml (with deduplication)
			wantErr:      false,
		},
		{
			name:           "no yaml collectors",
			yamlCollectors: []CollectorSpec{},
			opts: DiscoveryOptions{
				Namespaces: []string{"test-app"},
				RBACCheck:  false,
			},
			wantMinCount: 5, // Just foundational collectors
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			collectors, err := discoverer.AugmentWithFoundational(ctx, tt.yamlCollectors, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("AugmentWithFoundational() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if len(collectors) < tt.wantMinCount {
					t.Errorf("AugmentWithFoundational() returned %d collectors, want at least %d",
						len(collectors), tt.wantMinCount)
				}

				// Check that YAML collectors are preserved
				yamlCount := 0
				foundationalCount := 0
				for _, collector := range collectors {
					switch collector.Source {
					case SourceYAML:
						yamlCount++
					case SourceFoundational:
						foundationalCount++
					}
				}

				if yamlCount != len(tt.yamlCollectors) {
					t.Errorf("AugmentWithFoundational() preserved %d YAML collectors, want %d",
						yamlCount, len(tt.yamlCollectors))
				}

				if foundationalCount == 0 {
					t.Error("AugmentWithFoundational() should include foundational collectors")
				}
			}
		})
	}
}

func TestDiscoverer_getTargetNamespaces(t *testing.T) {
	client := fake.NewSimpleClientset()

	// Add test namespaces
	testNamespaces := []corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "test-app"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
	}

	for _, ns := range testNamespaces {
		client.CoreV1().Namespaces().Create(context.Background(), &ns, metav1.CreateOptions{})
	}

	discoverer, err := NewDiscoverer(&rest.Config{}, client)
	if err != nil {
		t.Fatalf("Failed to create discoverer: %v", err)
	}

	tests := []struct {
		name                string
		requestedNamespaces []string
		wantNamespaces      []string
		wantErr             bool
	}{
		{
			name:                "specific namespaces",
			requestedNamespaces: []string{"default", "test-app"},
			wantNamespaces:      []string{"default", "test-app"},
			wantErr:             false,
		},
		{
			name:                "no namespaces specified",
			requestedNamespaces: []string{},
			wantNamespaces:      []string{"default", "test-app", "kube-system"}, // All available
			wantErr:             false,
		},
		{
			name:                "single namespace",
			requestedNamespaces: []string{"default"},
			wantNamespaces:      []string{"default"},
			wantErr:             false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			namespaces, err := discoverer.getTargetNamespaces(ctx, tt.requestedNamespaces)

			if (err != nil) != tt.wantErr {
				t.Errorf("getTargetNamespaces() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil {
				if len(tt.requestedNamespaces) > 0 {
					// For specific namespaces, check exact match
					if len(namespaces) != len(tt.wantNamespaces) {
						t.Errorf("getTargetNamespaces() returned %d namespaces, want %d",
							len(namespaces), len(tt.wantNamespaces))
					}
				} else {
					// For auto-discovery, check we got some namespaces
					if len(namespaces) == 0 {
						t.Error("getTargetNamespaces() should return at least one namespace")
					}
				}
			}
		})
	}
}

func TestDiscoverer_generateFoundationalCollectors(t *testing.T) {
	client := fake.NewSimpleClientset()
	discoverer, err := NewDiscoverer(&rest.Config{}, client)
	if err != nil {
		t.Fatalf("Failed to create discoverer: %v", err)
	}

	tests := []struct {
		name             string
		namespaces       []string
		opts             DiscoveryOptions
		wantMinCount     int
		wantClusterLevel bool
	}{
		{
			name:             "single namespace",
			namespaces:       []string{"default"},
			opts:             DiscoveryOptions{IncludeImages: false},
			wantMinCount:     5, // 2 cluster + 3 namespace collectors
			wantClusterLevel: true,
		},
		{
			name:             "multiple namespaces",
			namespaces:       []string{"default", "test-app"},
			opts:             DiscoveryOptions{IncludeImages: false},
			wantMinCount:     8, // 2 cluster + 3*2 namespace collectors
			wantClusterLevel: true,
		},
		{
			name:             "with images",
			namespaces:       []string{"default"},
			opts:             DiscoveryOptions{IncludeImages: true},
			wantMinCount:     6, // 2 cluster + 4 namespace collectors
			wantClusterLevel: true,
		},
		{
			name:             "no namespaces",
			namespaces:       []string{},
			opts:             DiscoveryOptions{IncludeImages: false},
			wantMinCount:     2, // Just cluster collectors
			wantClusterLevel: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collectors := discoverer.generateFoundationalCollectors(context.Background(), tt.namespaces, tt.opts)

			if len(collectors) < tt.wantMinCount {
				t.Errorf("generateFoundationalCollectors() returned %d collectors, want at least %d",
					len(collectors), tt.wantMinCount)
			}

			// Check for cluster-level collectors
			hasClusterInfo := false
			hasClusterResources := false
			namespaceCollectors := make(map[string]int)

			for _, collector := range collectors {
				if collector.Type == CollectorTypeClusterInfo {
					hasClusterInfo = true
				}
				if collector.Type == CollectorTypeClusterResources {
					hasClusterResources = true
				}
				if collector.Namespace != "" {
					namespaceCollectors[collector.Namespace]++
				}
			}

			if tt.wantClusterLevel {
				if !hasClusterInfo {
					t.Error("generateFoundationalCollectors() missing cluster info collector")
				}
				if !hasClusterResources {
					t.Error("generateFoundationalCollectors() missing cluster resources collector")
				}
			}

			// Check namespace collectors
			for _, namespace := range tt.namespaces {
				if count := namespaceCollectors[namespace]; count == 0 {
					t.Errorf("generateFoundationalCollectors() has no collectors for namespace %s", namespace)
				}
			}
		})
	}
}

func TestDiscoverer_mergeAndDeduplicateCollectors(t *testing.T) {
	client := fake.NewSimpleClientset()
	discoverer, err := NewDiscoverer(&rest.Config{}, client)
	if err != nil {
		t.Fatalf("Failed to create discoverer: %v", err)
	}

	tests := []struct {
		name                   string
		yamlCollectors         []CollectorSpec
		foundationalCollectors []CollectorSpec
		wantCount              int
		wantYAMLPreferred      bool
	}{
		{
			name: "no conflicts",
			yamlCollectors: []CollectorSpec{
				{
					Type:      CollectorTypeLogs,
					Name:      "yaml-logs",
					Namespace: "app1",
					Priority:  100,
					Source:    SourceYAML,
				},
			},
			foundationalCollectors: []CollectorSpec{
				{
					Type:      CollectorTypeConfigMaps,
					Name:      "configmaps-app1",
					Namespace: "app1",
					Priority:  80,
					Source:    SourceFoundational,
				},
			},
			wantCount: 2,
		},
		{
			name: "with conflicts - YAML should win",
			yamlCollectors: []CollectorSpec{
				{
					Type:      CollectorTypeLogs,
					Name:      "logs-app1", // Same name to trigger deduplication
					Namespace: "app1",
					Priority:  100,
					Source:    SourceYAML,
				},
			},
			foundationalCollectors: []CollectorSpec{
				{
					Type:      CollectorTypeLogs,
					Name:      "logs-app1", // Same name to trigger deduplication
					Namespace: "app1",
					Priority:  90,
					Source:    SourceFoundational,
				},
			},
			wantCount:         1,
			wantYAMLPreferred: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merged := discoverer.mergeAndDeduplicateCollectors(tt.yamlCollectors, tt.foundationalCollectors)

			if len(merged) != tt.wantCount {
				t.Errorf("mergeAndDeduplicateCollectors() returned %d collectors, want %d",
					len(merged), tt.wantCount)
			}

			if tt.wantYAMLPreferred {
				// Check that YAML collectors are preferred over foundational ones
				yamlCollectorFound := false
				for _, collector := range merged {
					if collector.Source == SourceYAML {
						yamlCollectorFound = true
						break
					}
				}
				if !yamlCollectorFound {
					t.Error("mergeAndDeduplicateCollectors() should prefer YAML collectors over foundational")
				}
			}
		})
	}
}

// Benchmark tests for performance
func BenchmarkDiscoverer_DiscoverFoundational(b *testing.B) {
	client := fake.NewSimpleClientset()

	// Add multiple test namespaces
	for i := 0; i < 10; i++ {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("test-ns-%d", i),
			},
		}
		client.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	}

	discoverer, err := NewDiscoverer(&rest.Config{}, client)
	if err != nil {
		b.Fatalf("Failed to create discoverer: %v", err)
	}

	opts := DiscoveryOptions{
		Namespaces:    []string{}, // Auto-discover all namespaces
		IncludeImages: true,
		RBACCheck:     false,
		Timeout:       30 * time.Second,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := discoverer.DiscoverFoundational(context.Background(), opts)
		if err != nil {
			b.Fatalf("DiscoverFoundational failed: %v", err)
		}
	}
}
