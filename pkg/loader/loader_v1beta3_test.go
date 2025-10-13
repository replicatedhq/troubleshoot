package loader

import (
	"context"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestLoadSpecs_V1Beta3WithSecretRef(t *testing.T) {
	// Create test secret
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"uri": []byte("postgresql://user:password@localhost:5432/mydb"),
		},
	}

	client := fake.NewSimpleClientset(secret)

	spec := `
apiVersion: troubleshoot.sh/v1beta3
kind: SupportBundle
metadata:
  name: test-bundle
spec:
  collectors:
    - postgres:
        collectorName: main-db
        uri:
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: uri
`

	kinds, err := LoadSpecs(context.Background(), LoadOptions{
		RawSpec:   spec,
		Client:    client,
		Namespace: "default",
	})

	require.NoError(t, err)
	require.NotNil(t, kinds)
	require.Len(t, kinds.SupportBundlesV1Beta2, 1)

	bundle := kinds.SupportBundlesV1Beta2[0]
	assert.Equal(t, "test-bundle", bundle.Name)
	assert.Equal(t, "troubleshoot.sh/v1beta2", bundle.APIVersion)

	require.Len(t, bundle.Spec.Collectors, 1)
	require.NotNil(t, bundle.Spec.Collectors[0].Postgres)
	assert.Equal(t, "main-db", bundle.Spec.Collectors[0].Postgres.CollectorName)
	assert.Equal(t, "postgresql://user:password@localhost:5432/mydb", bundle.Spec.Collectors[0].Postgres.URI)
}

func TestLoadSpecs_V1Beta3WithTLSSecrets(t *testing.T) {
	// Create test secret with TLS certs
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tls-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"ca.crt":     []byte("CA_CERT_DATA"),
			"client.crt": []byte("CLIENT_CERT_DATA"),
			"client.key": []byte("CLIENT_KEY_DATA"),
		},
	}

	client := fake.NewSimpleClientset(secret)

	spec := `
apiVersion: troubleshoot.sh/v1beta3
kind: SupportBundle
metadata:
  name: test-bundle
spec:
  collectors:
    - postgres:
        uri:
          value: "postgresql://localhost:5432/db"
        tls:
          cacert:
            valueFrom:
              secretKeyRef:
                name: tls-secret
                key: ca.crt
          clientCert:
            valueFrom:
              secretKeyRef:
                name: tls-secret
                key: client.crt
          clientKey:
            valueFrom:
              secretKeyRef:
                name: tls-secret
                key: client.key
`

	kinds, err := LoadSpecs(context.Background(), LoadOptions{
		RawSpec:   spec,
		Client:    client,
		Namespace: "default",
	})

	require.NoError(t, err)
	require.NotNil(t, kinds)
	require.Len(t, kinds.SupportBundlesV1Beta2, 1)

	bundle := kinds.SupportBundlesV1Beta2[0]
	require.Len(t, bundle.Spec.Collectors, 1)
	require.NotNil(t, bundle.Spec.Collectors[0].Postgres)
	require.NotNil(t, bundle.Spec.Collectors[0].Postgres.TLS)

	assert.Equal(t, "CA_CERT_DATA", bundle.Spec.Collectors[0].Postgres.TLS.CACert)
	assert.Equal(t, "CLIENT_CERT_DATA", bundle.Spec.Collectors[0].Postgres.TLS.ClientCert)
	assert.Equal(t, "CLIENT_KEY_DATA", bundle.Spec.Collectors[0].Postgres.TLS.ClientKey)
}

func TestLoadSpecs_V1Beta3MultipleCollectors(t *testing.T) {
	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"uri": []byte("postgresql://localhost:5432/db"),
		},
	}

	mysqlSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"uri": []byte("mysql://localhost:3306/db"),
		},
	}

	client := fake.NewSimpleClientset(pgSecret, mysqlSecret)

	spec := `
apiVersion: troubleshoot.sh/v1beta3
kind: SupportBundle
metadata:
  name: test-bundle
spec:
  collectors:
    - postgres:
        uri:
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: uri
    - mysql:
        uri:
          valueFrom:
            secretKeyRef:
              name: mysql-secret
              key: uri
`

	kinds, err := LoadSpecs(context.Background(), LoadOptions{
		RawSpec:   spec,
		Client:    client,
		Namespace: "default",
	})

	require.NoError(t, err)
	require.NotNil(t, kinds)
	require.Len(t, kinds.SupportBundlesV1Beta2, 1)

	bundle := kinds.SupportBundlesV1Beta2[0]
	require.Len(t, bundle.Spec.Collectors, 2)

	require.NotNil(t, bundle.Spec.Collectors[0].Postgres)
	assert.Equal(t, "postgresql://localhost:5432/db", bundle.Spec.Collectors[0].Postgres.URI)

	require.NotNil(t, bundle.Spec.Collectors[1].Mysql)
	assert.Equal(t, "mysql://localhost:3306/db", bundle.Spec.Collectors[1].Mysql.URI)
}

func TestLoadSpecs_V1Beta3WithoutClient(t *testing.T) {
	spec := `
apiVersion: troubleshoot.sh/v1beta3
kind: SupportBundle
metadata:
  name: test-bundle
spec:
  collectors:
    - postgres:
        uri:
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: uri
`

	_, err := LoadSpecs(context.Background(), LoadOptions{
		RawSpec: spec,
		Strict:  true, // Enable strict mode to get error instead of warning
		// No client provided
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "kubernetes client required")
}

func TestLoadSpecs_V1Beta3MixedWithV1Beta2(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"uri": []byte("postgresql://localhost:5432/db"),
		},
	}

	client := fake.NewSimpleClientset(secret)

	specs := `
---
apiVersion: troubleshoot.sh/v1beta3
kind: SupportBundle
metadata:
  name: v1beta3-bundle
spec:
  collectors:
    - postgres:
        uri:
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: uri
---
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: v1beta2-bundle
spec:
  collectors:
    - clusterInfo: {}
`

	kinds, err := LoadSpecs(context.Background(), LoadOptions{
		RawSpec:   specs,
		Client:    client,
		Namespace: "default",
	})

	require.NoError(t, err)
	require.NotNil(t, kinds)
	require.Len(t, kinds.SupportBundlesV1Beta2, 2)

	// Find the v1beta3-converted bundle
	var v3Bundle *troubleshootv1beta2.SupportBundle
	var v2Bundle *troubleshootv1beta2.SupportBundle
	for i := range kinds.SupportBundlesV1Beta2 {
		if kinds.SupportBundlesV1Beta2[i].Name == "v1beta3-bundle" {
			v3Bundle = &kinds.SupportBundlesV1Beta2[i]
		}
		if kinds.SupportBundlesV1Beta2[i].Name == "v1beta2-bundle" {
			v2Bundle = &kinds.SupportBundlesV1Beta2[i]
		}
	}

	require.NotNil(t, v3Bundle, "v1beta3 bundle should be converted and loaded")
	require.NotNil(t, v2Bundle, "v1beta2 bundle should be loaded")

	// Verify v1beta3 bundle was resolved correctly
	require.Len(t, v3Bundle.Spec.Collectors, 1)
	require.NotNil(t, v3Bundle.Spec.Collectors[0].Postgres)
	assert.Equal(t, "postgresql://localhost:5432/db", v3Bundle.Spec.Collectors[0].Postgres.URI)

	// Verify v1beta2 bundle was loaded correctly
	require.Len(t, v2Bundle.Spec.Collectors, 1)
	require.NotNil(t, v2Bundle.Spec.Collectors[0].ClusterInfo)
}
