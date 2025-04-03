package analyzer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_compareDistributionConditionalToActual(t *testing.T) {
	var unknownDistribution string
	tests := []struct {
		name        string
		conditional string
		input       providers
		expected    bool
	}{
		{
			name:        "== microk8s when microk8s is found",
			conditional: "== microk8s",
			input: providers{
				microk8s: true,
			},
			expected: true,
		},
		{
			name:        "!= microk8s when microk8s is found",
			conditional: "!= microk8s",
			input: providers{
				microk8s: true,
			},
			expected: false,
		},
		{
			name:        "!== eks when gke is found",
			conditional: "!== eks",
			input: providers{
				gke: true,
			},
			expected: true,
		},
		{
			name:        "== kind when kind is found",
			conditional: "== kind",
			input: providers{
				kind: true,
			},
			expected: true,
		},
		{
			name:        "== k0s when k0s is found",
			conditional: "== k0s",
			input: providers{
				k0s: true,
			},
			expected: true,
		},
		{
			name:        "== embedded-cluster when embedded-cluster is found",
			conditional: "== embedded-cluster",
			input: providers{
				embeddedCluster: true,
			},
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := compareDistributionConditionalToActual(test.conditional, test.input, &unknownDistribution)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)
		})
	}
}

func Test_mustNormalizeDistributionName(t *testing.T) {
	tests := []struct {
		raw      string
		expected Provider
	}{
		{
			raw:      "microk8s",
			expected: microk8s,
		},
		{
			raw:      "MICROK8S",
			expected: microk8s,
		},
		{
			raw:      " microk8s ",
			expected: microk8s,
		},
		{
			raw:      "Docker-Desktop",
			expected: dockerDesktop,
		},
		{
			raw:      "embedded-cluster",
			expected: embeddedCluster,
		},
		{
			raw:      "k0s",
			expected: k0s,
		},
		{
			raw:      "kind",
			expected: kind,
		},
		{
			raw:      "k3s",
			expected: k3s,
		},
		{
			raw:      "ibm",
			expected: ibm,
		},
		{
			raw:      "ibmcloud",
			expected: ibm,
		},
		{
			raw:      "ibm cloud",
			expected: ibm,
		},
		{
			raw:      "gke",
			expected: gke,
		},
		{
			raw:      "aks",
			expected: aks,
		},
		{
			raw:      "eks",
			expected: eks,
		},
		{
			raw:      "oke",
			expected: oke,
		},
		{
			raw:      "rke2",
			expected: rke2,
		},
		{
			raw:      "dockerdesktop",
			expected: dockerDesktop,
		},
		{
			raw:      "docker desktop",
			expected: dockerDesktop,
		},
		{
			raw:      "docker-desktop",
			expected: dockerDesktop,
		},
	}

	for _, test := range tests {
		t.Run(test.raw, func(t *testing.T) {
			actual := mustNormalizeDistributionName(test.raw)

			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestParseNodesForProviders(t *testing.T) {
	tests := []struct {
		name               string
		nodes              []corev1.Node
		wantProviders      providers
		wantProviderString string
	}{
		{
			name: "embedded-cluster",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "embedded-cluster",
						Labels: map[string]string{
							"beta.kubernetes.io/arch":               "amd64",
							"beta.kubernetes.io/os":                 "linux",
							"kots.io/embedded-cluster-role":         "total-1",
							"kots.io/embedded-cluster-role-0":       "management",
							"kubernetes.io/arch":                    "amd64",
							"kubernetes.io/hostname":                "evans-vm1",
							"kubernetes.io/os":                      "linux",
							"management":                            "true",
							"node-role.kubernetes.io/control-plane": "true",
							"node.k0sproject.io/role":               "control-plane",
						},
					},
				},
			},
			wantProviders:      providers{embeddedCluster: true, k0s: true},
			wantProviderString: "embedded-cluster",
		},
		{
			name: "tanzu",
			nodes: []corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "t-node",
						Annotations: map[string]string{
							"cluster.x-k8s.io/cluster-name":                          "test-cluster-name",
							"cluster.x-k8s.io/cluster-namespace":                     "dev",
							"cluster.x-k8s.io/labels-from-machine":                   "machinelabel",
							"cluster.x-k8s.io/machine":                               "machinename",
							"cluster.x-k8s.io/owner-kind":                            "KubeadmControlPlane",
							"cluster.x-k8s.io/owner-name":                            "owner-name",
							"csi.volume.kubernetes.io/nodeid":                        "{\"csi.vsphere.vmware.com\":\"test-node-id\"}",
							"kubeadm.alpha.kubernetes.io/cri-socket":                 "unix:///var/run/containerd/containerd.sock",
							"node.alpha.kubernetes.io/ttl":                           "0",
							"volumes.kubernetes.io/controller-managed-attach-detach": "true",
							"creationTimestamp":                                      "2025-02-19T09:16:51Z",
						},
						Labels: map[string]string{
							"beta.kubernetes.io/arch":                                 "amd64",
							"beta.kubernetes.io/os":                                   "linux",
							"failure-domain.beta.kubernetes.io/zone":                  "tanzu-zone-1",
							"kubernetes.io/arch":                                      "amd64",
							"kubernetes.io/hostname":                                  "machinename",
							"kubernetes.io/os":                                        "linux",
							"node-role.kubernetes.io/control-plane":                   "",
							"node.cluster.x-k8s.io/esxi-host":                         "esxi-host",
							"node.kubernetes.io/exclude-from-external-load-balancers": "",
							"run.tanzu.vmware.com/kubernetesDistributionVersion":      "v1.27.11---vmware.1-fips.1-tkg.2",
							"run.tanzu.vmware.com/tkr":                                "v1.27.11---vmware.1-fips.1-tkg.2",
							"topology.kubernetes.io/zone":                             "tanzu-zone-1",
						},
					},
				},
			},
			wantProviders:      providers{tanzu: true},
			wantProviderString: "tanzu",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providers, stringProvider := ParseNodesForProviders(tt.nodes)
			assert.Equalf(t, tt.wantProviders, providers,
				"ParseNodesForProviders() gotProviders = %v, providers %v", providers, tt.wantProviders,
			)
			assert.Equalf(t, tt.wantProviderString, stringProvider,
				"ParseNodesForProviders() gotStringProvider = %v, stringProvider %v", stringProvider, tt.wantProviderString,
			)
		})
	}
}
