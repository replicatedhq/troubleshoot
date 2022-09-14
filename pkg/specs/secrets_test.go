package specs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func Test_LoadFromSecretMatchingLabel(t *testing.T) {
	type args struct {
		ctx    context.Context
		client kubernetes.Interface
	}
	tests := []struct {
		name                 string
		supportBundleSecrets []corev1.Secret
		want                 []string
		wantErr              bool
	}{
		{
			name: "support bundle secret with matching label and key",
			supportBundleSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: test
spec:
  collectors:
	- runPod:
		name: "run-ping"
		namespace: default
		podSpec: 
		  containers:
		  - name: run-ping
			image: busybox:1
			command: ["ping"]
			args: ["-w", "5", "www.google.com"]`),
					},
				},
			},
			want: []string{
				`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: test
spec:
  collectors:
	- runPod:
		name: "run-ping"
		namespace: default
		podSpec: 
		  containers:
		  - name: run-ping
			image: busybox:1
			command: ["ping"]
			args: ["-w", "5", "www.google.com"]`,
			},
		},
		{
			name: "support bundle secret with missing label",
			supportBundleSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: test
spec:
  collectors:
  - data:
      name: static/data.txt
      data: |
	    static data`),
					},
				},
			},
			want: []string(nil),
		},
		{
			name: "support bundle secret with matching label but wrong key",
			supportBundleSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "default",
					},
					Data: map[string][]byte{
						"support-bundle-specc": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: test
spec:
  collectors:
  - data:
      name: static/data.txt
      data: |
	    static data`),
					},
				},
			},
			want: []string(nil),
		},
		{
			name: "multiple support bundle secrets in the same namespace with matching label and key",
			supportBundleSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-info
spec:
  collectors:
  - clusterInfo: {}`),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-2",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-resources
spec:
  collectors:
  - clusterResources: {}`),
					},
				},
			},
			want: []string{
				`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-info
spec:
  collectors:
  - clusterInfo: {}`,
				`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-resources
spec:
  collectors:
  - clusterResources: {}`,
			},
		},
		{
			name: "multiple support bundle secrets in different namespaces with matching label and key",
			supportBundleSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "some-namespace",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-info
spec:
  collectors:
  - clusterInfo: {}`),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-2",
						Namespace: "some-namespace-2",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-resources
spec:
  collectors:
  - clusterResources: {}`),
					},
				},
			},
			want: []string{
				`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-info
spec:
  collectors:
  - clusterInfo: {}`,
				`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-resources
spec:
  collectors:
  - clusterResources: {}`,
			},
		},
		{
			name: "multiple support bundle secrets in different namespaces but only one with correct label and key",
			supportBundleSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "some-namespace",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec-wrong",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec-wrong": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-info
spec:
  collectors:
  - clusterInfo: {}`),
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-2",
						Namespace: "some-namespace-2",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-resources
spec:
  collectors:
  - clusterResources: {}`),
					},
				},
			},
			want: []string{
				`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-resources
spec:
  collectors:
  - clusterResources: {}`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := testclient.NewSimpleClientset()
			for _, secret := range tt.supportBundleSecrets {
				_, err := client.CoreV1().Secrets(secret.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			got, err := LoadFromSecretMatchingLabel(client, "troubleshoot.io/kind=supportbundle-spec", "", "support-bundle-spec")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestUserProvidedNamespace_LoadFromSecretMatchingLabel(t *testing.T) {
	type args struct {
		ctx    context.Context
		client kubernetes.Interface
	}
	tests := []struct {
		name                 string
		supportBundleSecrets []corev1.Secret
		want                 []string
		wantErr              bool
	}{
		{
			name: "support bundle secret with matching label and key in user provided namespace",
			supportBundleSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "some-namespace",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: test
spec:
  collectors:
  - data:
      name: static/data.txt
      data: |
	    static data`),
					},
				},
			},
			want: []string{
				`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: test
spec:
  collectors:
  - data:
      name: static/data.txt
      data: |
	    static data`,
			},
		},
		{
			name: "support bundle secret with matching label and key outside of user provided namespace",
			supportBundleSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "not-your-namespace",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: test
spec:
  collectors:
  - data:
      name: static/data.txt
      data: |
	    static data`),
					},
				},
			},
			want: []string(nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := testclient.NewSimpleClientset()
			for _, secret := range tt.supportBundleSecrets {
				_, err := client.CoreV1().Secrets(secret.Namespace).Create(ctx, &secret, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			got, err := LoadFromSecretMatchingLabel(client, "troubleshoot.io/kind=supportbundle-spec", "some-namespace", "support-bundle-spec")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
