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
		name  string
		nodes []corev1.Node
		want  providers
		want1 string
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
			want:  providers{embeddedCluster: true},
			want1: "embedded-cluster",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, stringProvider := ParseNodesForProviders(tt.nodes)
			assert.Equalf(t, tt.want, provider, "ParseNodesForProviders() got = %v, provider %v", provider, tt.want)
			assert.Equalf(t, tt.want1, stringProvider, "ParseNodesForProviders() got1 = %v, stringProvider %v", stringProvider, tt.want1)
		})
	}
}
