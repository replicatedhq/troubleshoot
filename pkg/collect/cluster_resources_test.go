package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/client/troubleshootclientset/scheme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	v1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	apixfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	fakediscovery "k8s.io/client-go/discovery/fake"
	testdynamicclient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"
)

func init() {
	apixfake.AddToScheme(scheme.Scheme)
}

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

func TestCollectClusterResources_CustomResource(t *testing.T) {
	ctx := context.Background()

	// Register supportbundle troubleshoot CRD
	dat, err := os.ReadFile("../../config/crds/troubleshoot.sh_supportbundles.yaml")
	require.NoError(t, err)

	obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(dat, nil, nil)
	require.NoError(t, err)
	apixClient := apixfake.NewSimpleClientset(obj)

	// Create a CR
	sbObject := troubleshootv1beta2.SupportBundle{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "supportbundle",
			Namespace: "default",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "SupportBundle",
			APIVersion: "troubleshoot.sh/v1beta2",
		},
		Spec: troubleshootv1beta2.SupportBundleSpec{
			Collectors: []*troubleshootv1beta2.Collect{
				{
					ClusterResources: &troubleshootv1beta2.ClusterResources{},
				},
			},
		},
	}

	dynamicClient := testdynamicclient.NewSimpleDynamicClient(scheme.Scheme, &sbObject)

	// Fetch the CR from cluster
	res, errs := crsV1(ctx, dynamicClient, apixClient.ApiextensionsV1(), []string{"default"})
	assert.Empty(t, errs)
	require.Equal(t, 2, len(res))
	assert.Equal(t, fromJSON(t, res["supportbundles.troubleshoot.sh/default.json"]), sbObject)
	assert.Equal(t, fromYAML(t, res["supportbundles.troubleshoot.sh/default.yaml"]), sbObject)
}

func fromYAML(t *testing.T, dat []byte) troubleshootv1beta2.SupportBundle {
	sb := []troubleshootv1beta2.SupportBundle{}
	err := yaml.Unmarshal(dat, &sb)
	require.NoError(t, err)
	require.Equal(t, 1, len(sb))
	return sb[0]
}

func fromJSON(t *testing.T, dat []byte) troubleshootv1beta2.SupportBundle {
	sb := []troubleshootv1beta2.SupportBundle{}
	err := json.Unmarshal(dat, &sb)
	require.NoError(t, err)
	require.Equal(t, 1, len(sb))
	return sb[0]
}

func Test_getPodDisruptionBudgets(t *testing.T) {
	tests := []struct {
		name       string
		pdbNames   []string
		namespaces []string
	}{
		{
			name:       "single namespace",
			pdbNames:   []string{"test-pdb"},
			namespaces: []string{"default"},
		},
		{
			name:       "multiple namespaces",
			pdbNames:   []string{"test-pdb"},
			namespaces: []string{"default", "test"},
		},
		{
			name:       "multiple pdbs in different namespaces",
			pdbNames:   []string{"test-pdb", "another-pdb"},
			namespaces: []string{"default", "test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewClientset()
			ctx := context.Background()
			err := createTestPodDisruptionBudgets(client, tt.pdbNames, tt.namespaces)
			assert.NoError(t, err)

			fakeDiscovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
			if !ok {
				t.Fatalf("could not convert Discovery() to *FakeDiscovery")
			}
			fakeDiscovery.Resources = []*metav1.APIResourceList{
				{
					GroupVersion: "policy/v1",
					APIResources: []metav1.APIResource{
						{
							Kind: "PodDisruptionBudget",
						},
					},
				},
			}

			pdbs, errors := getPodDisruptionBudgets(ctx, client, tt.namespaces)
			assert.Empty(t, errors)
			assert.Equal(t, len(tt.namespaces), len(pdbs))

			for _, ns := range tt.namespaces {
				assert.NotEmpty(t, pdbs[ns+".json"])
				var pdbList policyv1.PodDisruptionBudgetList
				err := json.Unmarshal(pdbs[ns+".json"], &pdbList)
				assert.NoError(t, err)
				assert.Equal(t, len(tt.pdbNames), len(pdbList.Items))
				for _, pdb := range pdbList.Items {
					assert.Contains(t, tt.pdbNames, pdb.ObjectMeta.Name)
				}
			}
		})
	}
}

func Test_getPodDisruptionBudgets_v1beta1(t *testing.T) {
	tests := []struct {
		name       string
		pdbNames   []string
		namespaces []string
	}{
		{
			name:       "single namespace v1beta1",
			pdbNames:   []string{"test-pdb-beta"},
			namespaces: []string{"default"},
		},
		{
			name:       "multiple namespaces v1beta1",
			pdbNames:   []string{"test-pdb-beta"},
			namespaces: []string{"default", "test"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewClientset()
			ctx := context.Background()
			err := createTestPodDisruptionBudgetsV1beta1(client, tt.pdbNames, tt.namespaces)
			assert.NoError(t, err)

			fakeDiscovery, ok := client.Discovery().(*fakediscovery.FakeDiscovery)
			if !ok {
				t.Fatalf("could not convert Discovery() to *FakeDiscovery")
			}
			// Mock discovery to only have v1beta1 PodDisruptionBudget
			fakeDiscovery.Resources = []*metav1.APIResourceList{
				{
					GroupVersion: "policy/v1beta1",
					APIResources: []metav1.APIResource{
						{
							Kind: "PodDisruptionBudget",
						},
					},
				},
			}

			pdbs, errors := getPodDisruptionBudgets(ctx, client, tt.namespaces)
			assert.Empty(t, errors)
			assert.Equal(t, len(tt.namespaces), len(pdbs))

			for _, ns := range tt.namespaces {
				assert.NotEmpty(t, pdbs[ns+".json"])
				var pdbList policyv1beta1.PodDisruptionBudgetList
				err := json.Unmarshal(pdbs[ns+".json"], &pdbList)
				assert.NoError(t, err)
				assert.Equal(t, len(tt.pdbNames), len(pdbList.Items))
				for _, pdb := range pdbList.Items {
					assert.Contains(t, tt.pdbNames, pdb.ObjectMeta.Name)
				}
			}
		})
	}
}

func createTestPodDisruptionBudgets(client kubernetes.Interface, pdbNames []string, namespaces []string) error {
	for _, ns := range namespaces {
		for _, pdbName := range pdbNames {
			minAvailable := intstr.FromInt32(1)
			_, err := client.PolicyV1().PodDisruptionBudgets(ns).Create(context.Background(), &policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name: pdbName,
				},
				Spec: policyv1.PodDisruptionBudgetSpec{
					MinAvailable: &minAvailable,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app",
						},
					},
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func createTestPodDisruptionBudgetsV1beta1(client kubernetes.Interface, pdbNames []string, namespaces []string) error {
	for _, ns := range namespaces {
		for _, pdbName := range pdbNames {
			minAvailable := intstr.FromInt32(1)
			_, err := client.PolicyV1beta1().PodDisruptionBudgets(ns).Create(context.Background(), &policyv1beta1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{
					Name: pdbName,
				},
				Spec: policyv1beta1.PodDisruptionBudgetSpec{
					MinAvailable: &minAvailable,
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test-app",
						},
					},
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func Test_CertificateSigningRequests(t *testing.T) {
	tests := []struct {
		name     string
		csrNames []string
	}{
		{
			name:     "single certificate signing request",
			csrNames: []string{"test-csr"},
		},
		{
			name:     "multiple certificate signing requests",
			csrNames: []string{"test-csr-1", "test-csr-2", "test-csr-3"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			err := createTestCertificateSigningRequests(client, tt.csrNames)
			assert.NoError(t, err)

			csrs, csrErrors := certificateSigningRequests(ctx, client)
			assert.Empty(t, csrErrors)
			assert.NotEmpty(t, csrs)

			var csrList certificatesv1.CertificateSigningRequestList
			err = json.Unmarshal(csrs, &csrList)
			assert.NoError(t, err)
			assert.Equal(t, len(tt.csrNames), len(csrList.Items))
			for _, csr := range csrList.Items {
				assert.Contains(t, tt.csrNames, csr.ObjectMeta.Name)
			}
		})
	}
}

func Test_CertificateSigningRequests_PermissionDenied(t *testing.T) {
	client := testclient.NewSimpleClientset()
	ctx := context.Background()

	// Add a reactor to simulate permission denied error
	client.PrependReactor("list", "certificatesigningrequests", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("certificatesigningrequests.certificates.k8s.io is forbidden: User \"system:serviceaccount:default:default\" cannot list resource \"certificatesigningrequests\" in API group \"certificates.k8s.io\" at the cluster scope")
	})

	csrs, csrErrors := certificateSigningRequests(ctx, client)

	// Verify fail-safe behavior: returns nil data + error string (not panic)
	assert.Nil(t, csrs)
	assert.NotEmpty(t, csrErrors)
	assert.Len(t, csrErrors, 1)
	// Verify the error is captured as a string
	assert.IsType(t, "", csrErrors[0])
	assert.Contains(t, csrErrors[0], "forbidden")
}

func createTestCertificateSigningRequests(client kubernetes.Interface, csrNames []string) error {
	for _, csrName := range csrNames {
		_, err := client.CertificatesV1().CertificateSigningRequests().Create(context.Background(), &certificatesv1.CertificateSigningRequest{
			ObjectMeta: metav1.ObjectMeta{
				Name: csrName,
			},
			Spec: certificatesv1.CertificateSigningRequestSpec{
				Request:    []byte("-----BEGIN CERTIFICATE REQUEST-----\ntest\n-----END CERTIFICATE REQUEST-----"),
				SignerName: "kubernetes.io/kube-apiserver-client",
				Usages:     []certificatesv1.KeyUsage{certificatesv1.UsageClientAuth},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func Test_ValidatingWebhookConfigurations(t *testing.T) {
	tests := []struct {
		name     string
		vwcNames []string
	}{
		{
			name:     "single validating webhook configuration",
			vwcNames: []string{"test-vwc"},
		},
		{
			name:     "multiple validating webhook configurations",
			vwcNames: []string{"vwc-1", "vwc-2", "vwc-3"},
		},
		{
			name:     "empty list",
			vwcNames: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			err := createTestValidatingWebhookConfigurations(client, tt.vwcNames)
			require.NoError(t, err)

			data, errs := validatingWebhookConfigurations(ctx, client)
			assert.Empty(t, errs)
			assert.NotNil(t, data)

			var list admissionregistrationv1.ValidatingWebhookConfigurationList
			err = json.Unmarshal(data, &list)
			require.NoError(t, err)
			assert.Len(t, list.Items, len(tt.vwcNames))
			for _, item := range list.Items {
				assert.Contains(t, tt.vwcNames, item.Name)
			}
		})
	}
}

func Test_ValidatingWebhookConfigurations_PermissionDenied(t *testing.T) {
	client := testclient.NewSimpleClientset()
	ctx := context.Background()

	client.PrependReactor("list", "validatingwebhookconfigurations", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("validatingwebhookconfigurations.admissionregistration.k8s.io is forbidden: User \"system:serviceaccount:default:default\" cannot list resource \"validatingwebhookconfigurations\" in API group \"admissionregistration.k8s.io\" at the cluster scope")
	})

	data, errs := validatingWebhookConfigurations(ctx, client)

	assert.Nil(t, data)
	require.NotEmpty(t, errs)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "forbidden")
}

func createTestValidatingWebhookConfigurations(client kubernetes.Interface, names []string) error {
	for _, name := range names {
		_, err := client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(context.Background(), &admissionregistrationv1.ValidatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Webhooks: []admissionregistrationv1.ValidatingWebhook{
				{
					Name: "test-webhook.example.com",
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Namespace: "default",
							Name:      "webhook-service",
						},
					},
					AdmissionReviewVersions: []string{"v1"},
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

func Test_MutatingWebhookConfigurations(t *testing.T) {
	tests := []struct {
		name     string
		mwcNames []string
	}{
		{
			name:     "single mutating webhook configuration",
			mwcNames: []string{"test-mwc"},
		},
		{
			name:     "multiple mutating webhook configurations",
			mwcNames: []string{"mwc-1", "mwc-2"},
		},
		{
			name:     "empty list",
			mwcNames: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset()
			ctx := context.Background()
			err := createTestMutatingWebhookConfigurations(client, tt.mwcNames)
			require.NoError(t, err)

			data, errs := mutatingWebhookConfigurations(ctx, client)
			assert.Empty(t, errs)
			assert.NotNil(t, data)

			var list admissionregistrationv1.MutatingWebhookConfigurationList
			err = json.Unmarshal(data, &list)
			require.NoError(t, err)
			assert.Len(t, list.Items, len(tt.mwcNames))
			for _, item := range list.Items {
				assert.Contains(t, tt.mwcNames, item.Name)
			}
		})
	}
}

func Test_MutatingWebhookConfigurations_PermissionDenied(t *testing.T) {
	client := testclient.NewSimpleClientset()
	ctx := context.Background()

	client.PrependReactor("list", "mutatingwebhookconfigurations", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, fmt.Errorf("mutatingwebhookconfigurations.admissionregistration.k8s.io is forbidden: User \"system:serviceaccount:default:default\" cannot list resource \"mutatingwebhookconfigurations\" in API group \"admissionregistration.k8s.io\" at the cluster scope")
	})

	data, errs := mutatingWebhookConfigurations(ctx, client)

	assert.Nil(t, data)
	require.NotEmpty(t, errs)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0], "forbidden")
}

func createTestMutatingWebhookConfigurations(client kubernetes.Interface, names []string) error {
	for _, name := range names {
		_, err := client.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.Background(), &admissionregistrationv1.MutatingWebhookConfiguration{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
			Webhooks: []admissionregistrationv1.MutatingWebhook{
				{
					Name: "test-mutating-webhook.example.com",
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						Service: &admissionregistrationv1.ServiceReference{
							Namespace: "default",
							Name:      "webhook-service",
						},
					},
					AdmissionReviewVersions: []string{"v1"},
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}
