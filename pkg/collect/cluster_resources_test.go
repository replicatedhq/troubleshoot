package collect

import (
	"context"
	"reflect"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func Test_ConfigMaps(t *testing.T) {
	tests := []struct {
		name          string
		configMapName string
		namespaces    []string
	}{
		{
			name:          "single namespace",
			configMapName: "default",
			namespaces:    []string{"default"},
		},
		{
			name:          "multiple namespaces",
			configMapName: "default",
			namespaces:    []string{"default", "test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			err := createConfigMaps(client, tt.configMapName, tt.namespaces)
			require.NoError(t, err)

			configMaps, _ := configMaps(ctx, client, tt.namespaces)
			assert.Equal(t, len(tt.namespaces), len(configMaps))

			for _, ns := range tt.namespaces {
				assert.NotEmpty(t, configMaps[ns+".json"])
			}
		})
	}
}

func createConfigMaps(client kubernetes.Interface, configMapName string, namespaces []string) error {
	for _, ns := range namespaces {
		_, err := client.CoreV1().ConfigMaps(ns).Create(context.Background(), &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: configMapName,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func Test_VolumeAttachments(t *testing.T) {
	tests := []struct {
		name      string
		leaseName string
	}{
		{
			name:      "single namespace",
			leaseName: "default",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			err := createTestVolumeAttachments(client, tt.leaseName)
			require.NoError(t, err)

			volumeAttachments, _ := volumeAttachments(ctx, client)
			assert.NotEmpty(t, volumeAttachments)
		})
	}
}

func createTestVolumeAttachments(client kubernetes.Interface, leaseName string) error {
	_, err := client.StorageV1().VolumeAttachments().Create(context.Background(), &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: leaseName,
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func Test_Leases(t *testing.T) {
	tests := []struct {
		name       string
		leaseName  string
		namespaces []string
	}{
		{
			name:       "single namespace",
			leaseName:  "default",
			namespaces: []string{"default"},
		},
		{
			name:       "multiple namespaces",
			leaseName:  "default",
			namespaces: []string{"default", "test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			err := createTestLeases(client, tt.leaseName, tt.namespaces)
			require.NoError(t, err)

			leases, _ := leases(ctx, client, tt.namespaces)
			assert.Equal(t, len(tt.namespaces), len(leases))

			for _, ns := range tt.namespaces {
				assert.NotEmpty(t, leases[ns+".json"])
			}
		})
	}
}

func createTestLeases(client kubernetes.Interface, leaseName string, namespaces []string) error {
	for _, ns := range namespaces {
		_, err := client.CoordinationV1().Leases(ns).Create(context.Background(), &v1.Lease{
			ObjectMeta: metav1.ObjectMeta{
				Name: leaseName,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func Test_ServiceAccounts(t *testing.T) {
	tests := []struct {
		name               string
		serviceAccountName string
		namespaces         []string
	}{
		{
			name:               "single namespace",
			serviceAccountName: "default",
			namespaces:         []string{"default"},
		},
		{
			name:               "multiple namespaces",
			serviceAccountName: "default",
			namespaces:         []string{"default", "test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			err := createTestServiceAccounts(client, tt.serviceAccountName, tt.namespaces)
			require.NoError(t, err)

			servicesAccounts, _ := serviceAccounts(ctx, client, tt.namespaces)
			assert.Equal(t, len(tt.namespaces), len(servicesAccounts))

			for _, ns := range tt.namespaces {
				assert.NotEmpty(t, servicesAccounts[ns+".json"])
			}
		})
	}
}

func createTestServiceAccounts(client kubernetes.Interface, serviceName string, namespaces []string) error {
	for _, ns := range namespaces {
		_, err := client.CoreV1().ServiceAccounts(ns).Create(context.Background(), &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name: serviceName,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func Test_SelectCRDVersionByPriority(t *testing.T) {
	assert.Equal(t, "v1alpha3", selectCRDVersionByPriority([]string{"v1alpha2", "v1alpha3"}))
	assert.Equal(t, "v1alpha3", selectCRDVersionByPriority([]string{"v1alpha3", "v1alpha2"}))
	assert.Equal(t, "v1", selectCRDVersionByPriority([]string{"v1alpha2", "v1alpha3", "v1"}))
	assert.Equal(t, "v1", selectCRDVersionByPriority([]string{"v1", "v1alpha2", "v1alpha3"}))
}

func TestClusterResources_Merge(t *testing.T) {
	tests := []struct {
		name       string
		Collectors []troubleshootv1beta2.Collect
		want       *CollectClusterResources
	}{
		{
			name: "single cluster resources collector with multiple unique namespaces",
			Collectors: []troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello", "hello2"},
					},
				},
			},
			want: &CollectClusterResources{
				Collector: &troubleshootv1beta2.ClusterResources{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "collectorname",
					},
					Namespaces: []string{"hello", "hello2"},
				},
			},
		},
		{
			name: "multiple cluster resources collectors with unique namespaces",
			Collectors: []troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello2"},
					},
				},
			},
			want: &CollectClusterResources{
				Collector: &troubleshootv1beta2.ClusterResources{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "collectorname",
					},
					Namespaces: []string{"hello", "hello2"},
				},
			},
		},
		{
			name: "multiple cluster resources collectors with duplicate namespaces",
			Collectors: []troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello2"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello"},
					},
				},
			},
			want: &CollectClusterResources{
				Collector: &troubleshootv1beta2.ClusterResources{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "collectorname",
					},
					Namespaces: []string{"hello", "hello2"},
				},
			},
		},
		{
			name: "multiple cluster resource collectors with a empty string namespace provided",
			Collectors: []troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello2"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{""},
					},
				},
			},
			want: &CollectClusterResources{
				Collector: &troubleshootv1beta2.ClusterResources{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "collectorname",
					},
					Namespaces: nil,
				},
			},
		},
		{
			name: "multiple cluster resource collectors with a nil namespace provided",
			Collectors: []troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: []string{"hello2"},
					},
				},
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "collectorname",
						},
						Namespaces: nil,
					},
				},
			},
			want: &CollectClusterResources{
				Collector: &troubleshootv1beta2.ClusterResources{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: "collectorname",
					},
					Namespaces: nil,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			var mergedCollectors []Collector
			allCollectors := make(map[reflect.Type][]Collector)
			collectorType := reflect.TypeOf(CollectClusterResources{})

			for _, collector := range tt.Collectors {
				collectorInterface, _ := GetCollector(&collector, "", "", nil, nil, nil)
				if mergeCollector, ok := collectorInterface.(MergeableCollector); ok {
					allCollectors[collectorType] = append(allCollectors[collectorType], mergeCollector)
				}
			}

			for _, collectors := range allCollectors {
				if mergeCollector, ok := collectors[0].(MergeableCollector); ok {
					mergedCollectors, _ = mergeCollector.Merge(collectors)
				}
			}

			clusterResourceCollector, _ := mergedCollectors[0].(*CollectClusterResources)

			req.EqualValues(tt.want, clusterResourceCollector)
		})
	}
}
