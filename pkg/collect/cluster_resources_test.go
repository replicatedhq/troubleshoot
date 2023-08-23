package collect

import (
	"context"
	"encoding/json"
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
		name           string
		configMapNames []string
		namespaces     []string
	}{
		{
			name:           "single namespace",
			configMapNames: []string{"default"},
			namespaces:     []string{"default"},
		},
		{
			name:           "multiple namespaces",
			configMapNames: []string{"default"},
			namespaces:     []string{"default", "test"},
		},
		{
			name:           "multiple in different namespaces",
			configMapNames: []string{"default", "test-cm"},
			namespaces:     []string{"default", "test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			err := createConfigMaps(client, tt.configMapNames, tt.namespaces)
			assert.NoError(t, err)

			configMaps, _ := configMaps(ctx, client, tt.namespaces)
			assert.Equal(t, len(tt.namespaces), len(configMaps))

			for _, ns := range tt.namespaces {
				assert.NotEmpty(t, configMaps[ns+".json"])
				var configmapList corev1.ConfigMapList
				err := json.Unmarshal(configMaps[ns+".json"], &configmapList)
				assert.NoError(t, err)
				// Ensure the ConfigMap names match those in the list
				assert.Equal(t, len(configmapList.Items), len(tt.configMapNames))
				for _, cm := range configmapList.Items {
					assert.Contains(t, tt.configMapNames, cm.ObjectMeta.Name)
				}
			}
		})
	}
}

func createConfigMaps(client kubernetes.Interface, configMapNames []string, namespaces []string) error {
	for _, ns := range namespaces {
		for _, cmName := range configMapNames {
			_, err := client.CoreV1().ConfigMaps(ns).Create(context.Background(), &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmName,
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func Test_VolumeAttachments(t *testing.T) {
	tests := []struct {
		name                  string
		volumeAttachmentNames []string
	}{
		{
			name:                  "single volume attachment",
			volumeAttachmentNames: []string{"default"},
		},

		{
			name:                  "multiple volume attachments",
			volumeAttachmentNames: []string{"default", "test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			err := createTestVolumeAttachments(client, tt.volumeAttachmentNames)
			assert.NoError(t, err)

			volumeAttachments, _ := volumeAttachments(ctx, client)
			assert.NotEmpty(t, volumeAttachments)
			var volumeAttachmentList storagev1.VolumeAttachmentList
			err = json.Unmarshal(volumeAttachments, &volumeAttachmentList)
			assert.NoError(t, err)
			// Ensure the VolumeAttachment names match those in the list
			assert.Equal(t, len(volumeAttachmentList.Items), len(tt.volumeAttachmentNames))
			for _, va := range volumeAttachmentList.Items {
				assert.Contains(t, tt.volumeAttachmentNames, va.ObjectMeta.Name)
			}
		})
	}
}

func createTestVolumeAttachments(client kubernetes.Interface, volumeAttachmentNames []string) error {
	for _, vaName := range volumeAttachmentNames {
		_, err := client.StorageV1().VolumeAttachments().Create(context.Background(), &storagev1.VolumeAttachment{
			ObjectMeta: metav1.ObjectMeta{
				Name: vaName,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func Test_Leases(t *testing.T) {
	tests := []struct {
		name       string
		leaseNames []string
		namespaces []string
	}{
		{
			name:       "single namespace",
			leaseNames: []string{"default"},
			namespaces: []string{"default"},
		},
		{
			name:       "multiple namespaces",
			leaseNames: []string{"default"},
			namespaces: []string{"default", "test"},
		},
		{
			name:       "multiple in different namespaces",
			leaseNames: []string{"default", "test-lease"},
			namespaces: []string{"default", "test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			err := createTestLeases(client, tt.leaseNames, tt.namespaces)
			assert.NoError(t, err)

			leases, _ := leases(ctx, client, tt.namespaces)
			assert.Equal(t, len(tt.namespaces), len(leases))

			for _, ns := range tt.namespaces {
				assert.NotEmpty(t, leases[ns+".json"])
				var leaseList v1.LeaseList
				err := json.Unmarshal(leases[ns+".json"], &leaseList)
				assert.NoError(t, err)
				// Ensure the Lease names match those in the list
				assert.Equal(t, len(leaseList.Items), len(tt.leaseNames))
				for _, lease := range leaseList.Items {
					assert.Contains(t, tt.leaseNames, lease.ObjectMeta.Name)
				}
			}
		})
	}
}

func createTestLeases(client kubernetes.Interface, leaseNames []string, namespaces []string) error {
	for _, ns := range namespaces {
		for _, leaseName := range leaseNames {
			_, err := client.CoordinationV1().Leases(ns).Create(context.Background(), &v1.Lease{
				ObjectMeta: metav1.ObjectMeta{
					Name: leaseName,
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func Test_ServiceAccounts(t *testing.T) {
	tests := []struct {
		name                string
		serviceAccountNames []string
		namespaces          []string
	}{
		{
			name:                "single namespace",
			serviceAccountNames: []string{"default"},
			namespaces:          []string{"default"},
		},
		{
			name:                "multiple namespaces",
			serviceAccountNames: []string{"default"},
			namespaces:          []string{"default", "test"},
		},
		{
			name:                "multiple in different namespaces",
			serviceAccountNames: []string{"default", "test-sa"},
			namespaces:          []string{"default", "test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			err := createTestServiceAccounts(client, tt.serviceAccountNames, tt.namespaces)
			assert.NoError(t, err)

			servicesAccounts, _ := serviceAccounts(ctx, client, tt.namespaces)
			assert.Equal(t, len(tt.namespaces), len(servicesAccounts))

			for _, ns := range tt.namespaces {
				assert.NotEmpty(t, servicesAccounts[ns+".json"])
				var serviceAccountList corev1.ServiceAccountList
				err := json.Unmarshal(servicesAccounts[ns+".json"], &serviceAccountList)
				assert.NoError(t, err)
				// Ensure the ServiceAccount names match those in the list
				assert.Equal(t, len(serviceAccountList.Items), len(tt.serviceAccountNames))
				for _, sa := range serviceAccountList.Items {
					assert.Contains(t, tt.serviceAccountNames, sa.ObjectMeta.Name)
				}
			}
		})
	}
}

func createTestServiceAccounts(client kubernetes.Interface, serviceAccountNames []string, namespaces []string) error {
	for _, ns := range namespaces {
		for _, saName := range serviceAccountNames {
			_, err := client.CoreV1().ServiceAccounts(ns).Create(context.Background(), &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name: saName,
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return err
			}
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
