package specs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
							"troubleshoot.io/kind": "support-bundle",
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
			name: "mutlidoc support bundle secret with matching label and key",
			supportBundleSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "support-bundle",
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
			args: ["-w", "5", "www.google.com"]
---
apiVersion: troubleshoot.sh/v1beta2
kind: Redactor
metadata:
  name: Usernames
spec:
  redactors:
  - name: Redact usernames in multiline JSON
    removals:
      regex:
      - selector: '(?i)"name": *".*user[^\"]*"'
        redactor: '(?i)("value": *")(?P<mask>.*[^\"]*)(")'`),
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
			args: ["-w", "5", "www.google.com"]
---
apiVersion: troubleshoot.sh/v1beta2
kind: Redactor
metadata:
  name: Usernames
spec:
  redactors:
  - name: Redact usernames in multiline JSON
    removals:
      regex:
      - selector: '(?i)"name": *".*user[^\"]*"'
        redactor: '(?i)("value": *")(?P<mask>.*[^\"]*)(")'`,
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
							"troubleshoot.io/kind": "support-bundle",
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
							"troubleshoot.io/kind": "support-bundle",
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
							"troubleshoot.io/kind": "support-bundle",
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
							"troubleshoot.io/kind": "support-bundle",
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
							"troubleshoot.io/kind": "support-bundle-wrong",
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
							"troubleshoot.io/kind": "support-bundle",
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
			got, err := LoadFromSecretMatchingLabel(client, "troubleshoot.io/kind=support-bundle", "", "support-bundle-spec")
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
							"troubleshoot.io/kind": "support-bundle",
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
							"troubleshoot.io/kind": "support-bundle",
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
			got, err := LoadFromSecretMatchingLabel(client, "troubleshoot.io/kind=support-bundle", "some-namespace", "support-bundle-spec")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestRedactors_LoadFromSecretMatchingLabel(t *testing.T) {
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
			name: "redactor secret with matching label and key",
			supportBundleSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "support-bundle",
						},
					},
					Data: map[string][]byte{
						"redactor-spec": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: Redactor
metadata:
  name: redact-some-content
spec:
  redactors:
  - name: replace some-content
    fileSelector:
      file: result.json
    removals:
      values:
      - some-content`),
					},
				},
			},
			want: []string{
				`apiVersion: troubleshoot.sh/v1beta2
kind: Redactor
metadata:
  name: redact-some-content
spec:
  redactors:
  - name: replace some-content
    fileSelector:
      file: result.json
    removals:
      values:
      - some-content`,
			},
		},
		{
			name: "redactor secret with matching label but wrong key",
			supportBundleSecrets: []corev1.Secret{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "support-bundle",
						},
					},
					Data: map[string][]byte{
						"redactor-spec-wrong": []byte(`apiVersion: troubleshoot.sh/v1beta2
kind: Redactor
metadata:
  name: redact-some-content
spec:
  redactors:
  - name: replace some-content
    fileSelector:
      file: result.json
    removals:
      values:
      - some-content`),
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
			got, err := LoadFromSecretMatchingLabel(client, "troubleshoot.io/kind=support-bundle", "", "redactor-spec")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func Test_getSpecSecretsMatchingLabel(t *testing.T) {
	tests := []struct {
		name            string
		secret          []runtime.Object
		targetNamespace string
		targetLabelKey  string
		key             string
		wantErr         bool
	}{
		{
			name: "get secret with matching label",
			secret: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-1",
						Namespace: "default",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Data: map[string][]byte{
						"spec": []byte("spec-1"),
					},
				},
			},
			key:             "spec",
			targetNamespace: "default",
			targetLabelKey:  "foo=bar",
			wantErr:         false,
		},
		{
			name: "get support bundle spec secret with matching label",
			secret: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-2",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "support-bundle",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte("spec-1"),
					},
				},
			},
			key:             "support-bundle-spec",
			targetNamespace: "default",
			targetLabelKey:  "troubleshoot.io/kind=support-bundle",
			wantErr:         false,
		},
		{
			name: "cannot get support bundle spec secret with wrong spec key",
			secret: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-2",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "support-bundle",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte("spec-1"),
					},
				},
			},
			key:             "spec",
			targetNamespace: "default",
			targetLabelKey:  "troubleshoot.io/kind=support-bundle",
			wantErr:         true,
		},
		{
			name: "get support bundle spec secret with multiple label selector",
			secret: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-1",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "support-bundle",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte("spec-1"),
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-2",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "support-bundle",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte("spec-2"),
					},
				},
			},
			key:             "support-bundle-spec",
			targetNamespace: "default",
			targetLabelKey:  "troubleshoot.io/kind=support-bundle",
			wantErr:         false,
		},
		{
			name: "get support bundle spec secret with multiple label selector and one matching",
			secret: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-1",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "support-bundle",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec-wrong": []byte("spec-1"),
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-2",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "support-bundle",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec": []byte("spec-2"),
					},
				},
			},
			key:             "support-bundle-spec",
			targetNamespace: "default",
			targetLabelKey:  "troubleshoot.io/kind=support-bundle",
			wantErr:         false,
		},
		{
			name: "cannot get support bundle spec secret with multiple label selector and none matching",
			secret: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-1",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "support-bundle",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec-wrong": []byte("spec-1"),
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret-2",
						Namespace: "default",
						Labels: map[string]string{
							"troubleshoot.io/kind": "support-bundle",
						},
					},
					Data: map[string][]byte{
						"support-bundle-spec-wrong": []byte("spec-2"),
					},
				},
			},
			key:             "support-bundle-spec",
			targetNamespace: "default",
			targetLabelKey:  "troubleshoot.io/kind=support-bundle",
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := testclient.NewSimpleClientset(tt.secret...)
			_, err := GetSecretMatchingLabel(client, tt.targetLabelKey, tt.targetNamespace, tt.key)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
