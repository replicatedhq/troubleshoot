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

func Test_LoadFromConfigMapMatchingLabel(t *testing.T) {
	type args struct {
		ctx    context.Context
		client kubernetes.Interface
	}
	tests := []struct {
		name                    string
		supportBundleConfigMaps []corev1.ConfigMap
		want                    []string
		wantErr                 bool
	}{
		{
			name: "support bundle configmap with matching label and key",
			supportBundleConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string]string{
						"support-bundle-spec": `apiVersion: troubleshoot.sh/v1beta2
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
			name: "support bundle configmap with missing label",
			supportBundleConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configap",
						Namespace: "default",
					},
					Data: map[string]string{
						"support-bundle-spec": `apiVersion: troubleshoot.sh/v1beta2
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
			},
			want: []string(nil),
		},
		{
			name: "support bundle configmap with matching label but wrong key",
			supportBundleConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap",
						Namespace: "default",
					},
					Data: map[string]string{
						"support-bundle-specc": `apiVersion: troubleshoot.sh/v1beta2
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
			},
			want: []string(nil),
		},
		{
			name: "multiple support bundle configmaps in the same namespace with matching label and key",
			supportBundleConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string]string{
						"support-bundle-spec": `apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-info
spec:
  collectors:
  - clusterInfo: {}`,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap-2",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string]string{
						"support-bundle-spec": `apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-resources
spec:
  collectors:
  - clusterResources: {}`,
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
			name: "multiple support bundle configmaps in different namespaces with matching label and key",
			supportBundleConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap",
						Namespace: "some-namespace",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string]string{
						"support-bundle-spec": `apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-info
spec:
  collectors:
  - clusterInfo: {}`,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap-2",
						Namespace: "some-namespace-2",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string]string{
						"support-bundle-spec": `apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-resources
spec:
  collectors:
  - clusterResources: {}`,
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
			name: "multiple support bundle configmaps in different namespaces but only one with correct label and key",
			supportBundleConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap",
						Namespace: "some-namespace",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec-wrong",
						},
					},
					Data: map[string]string{
						"support-bundle-spec-wrong": `apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-info
spec:
  collectors:
  - clusterInfo: {}`,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap-2",
						Namespace: "some-namespace-2",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string]string{
						"support-bundle-spec": `apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: cluster-resources
spec:
  collectors:
  - clusterResources: {}`,
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
			for _, configmap := range tt.supportBundleConfigMaps {
				_, err := client.CoreV1().ConfigMaps(configmap.Namespace).Create(ctx, &configmap, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			got, err := LoadFromConfigMapMatchingLabel(client, "troubleshoot.io/kind=supportbundle-spec", "", "support-bundle-spec")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestUserProvidedNamespace_LoadFromConfigMapMatchingLabel(t *testing.T) {
	type args struct {
		ctx    context.Context
		client kubernetes.Interface
	}
	tests := []struct {
		name                    string
		supportBundleConfigMaps []corev1.ConfigMap
		want                    []string
		wantErr                 bool
	}{
		{
			name: "support bundle configmap with matching label and key in user provided namespace",
			supportBundleConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap",
						Namespace: "some-namespace",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string]string{
						"support-bundle-spec": `apiVersion: troubleshoot.sh/v1beta2
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
			name: "support bundle configmap with matching label and key outside of user provided namespace",
			supportBundleConfigMaps: []corev1.ConfigMap{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "configmap",
						Namespace: "not-your-namespace",
						Labels: map[string]string{
							"troubleshoot.io/kind": "supportbundle-spec",
						},
					},
					Data: map[string]string{
						"support-bundle-spec": `apiVersion: troubleshoot.sh/v1beta2
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
			},
			want: []string(nil),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := testclient.NewSimpleClientset()
			for _, configmap := range tt.supportBundleConfigMaps {
				_, err := client.CoreV1().ConfigMaps(configmap.Namespace).Create(ctx, &configmap, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			got, err := LoadFromConfigMapMatchingLabel(client, "troubleshoot.io/kind=supportbundle-spec", "some-namespace", "support-bundle-spec")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
