package v1beta3

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

func TestConvertToV1Beta2WithResolution_PostgresWithLiteralValue(t *testing.T) {
	client := fake.NewSimpleClientset()
	uri := "postgresql://user:pass@localhost:5432/db"

	v3spec := &SupportBundleSpec{
		Collectors: []*Collect{
			{
				Postgres: &Database{
					URI: StringOrValueFrom{
						Value: &uri,
					},
				},
			},
		},
	}

	v2spec, err := ConvertToV1Beta2WithResolution(context.Background(), v3spec, client, "default")
	require.NoError(t, err)
	require.NotNil(t, v2spec)
	require.Len(t, v2spec.Collectors, 1)
	require.NotNil(t, v2spec.Collectors[0].Postgres)
	assert.Equal(t, "postgresql://user:pass@localhost:5432/db", v2spec.Collectors[0].Postgres.URI)
}

func TestConvertToV1Beta2WithResolution_PostgresWithSecretRef(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"uri": []byte("postgresql://user:secret-pass@db.example.com:5432/mydb"),
		},
	}

	client := fake.NewSimpleClientset(secret)

	v3spec := &SupportBundleSpec{
		Collectors: []*Collect{
			{
				Postgres: &Database{
					URI: StringOrValueFrom{
						ValueFrom: &ValueFromSource{
							SecretKeyRef: &SecretKeyRef{
								Name: "db-secret",
								Key:  "uri",
							},
						},
					},
				},
			},
		},
	}

	v2spec, err := ConvertToV1Beta2WithResolution(context.Background(), v3spec, client, "default")
	require.NoError(t, err)
	require.NotNil(t, v2spec)
	require.Len(t, v2spec.Collectors, 1)
	require.NotNil(t, v2spec.Collectors[0].Postgres)
	assert.Equal(t, "postgresql://user:secret-pass@db.example.com:5432/mydb", v2spec.Collectors[0].Postgres.URI)
}

func TestConvertToV1Beta2WithResolution_PostgresWithTLS(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tls-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"ca.crt":     []byte("-----BEGIN CERTIFICATE-----\nCA_CERT_DATA\n-----END CERTIFICATE-----"),
			"client.crt": []byte("-----BEGIN CERTIFICATE-----\nCLIENT_CERT_DATA\n-----END CERTIFICATE-----"),
			"client.key": []byte("-----BEGIN PRIVATE KEY-----\nCLIENT_KEY_DATA\n-----END PRIVATE KEY-----"),
		},
	}

	client := fake.NewSimpleClientset(secret)
	uri := "postgresql://user:pass@localhost:5432/db"

	v3spec := &SupportBundleSpec{
		Collectors: []*Collect{
			{
				Postgres: &Database{
					URI: StringOrValueFrom{
						Value: &uri,
					},
					TLS: &TLSParams{
						CACert: StringOrValueFrom{
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name: "tls-secret",
									Key:  "ca.crt",
								},
							},
						},
						ClientCert: StringOrValueFrom{
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name: "tls-secret",
									Key:  "client.crt",
								},
							},
						},
						ClientKey: StringOrValueFrom{
							ValueFrom: &ValueFromSource{
								SecretKeyRef: &SecretKeyRef{
									Name: "tls-secret",
									Key:  "client.key",
								},
							},
						},
					},
				},
			},
		},
	}

	v2spec, err := ConvertToV1Beta2WithResolution(context.Background(), v3spec, client, "default")
	require.NoError(t, err)
	require.NotNil(t, v2spec)
	require.Len(t, v2spec.Collectors, 1)
	require.NotNil(t, v2spec.Collectors[0].Postgres)
	require.NotNil(t, v2spec.Collectors[0].Postgres.TLS)

	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nCA_CERT_DATA\n-----END CERTIFICATE-----", v2spec.Collectors[0].Postgres.TLS.CACert)
	assert.Equal(t, "-----BEGIN CERTIFICATE-----\nCLIENT_CERT_DATA\n-----END CERTIFICATE-----", v2spec.Collectors[0].Postgres.TLS.ClientCert)
	assert.Equal(t, "-----BEGIN PRIVATE KEY-----\nCLIENT_KEY_DATA\n-----END PRIVATE KEY-----", v2spec.Collectors[0].Postgres.TLS.ClientKey)
}

func TestConvertToV1Beta2WithResolution_MultipleDatabases(t *testing.T) {
	pgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "postgres-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"uri": []byte("postgresql://user:pass@pg.example.com:5432/db"),
		},
	}

	mysqlSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"uri": []byte("mysql://user:pass@mysql.example.com:3306/db"),
		},
	}

	redisSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"uri": []byte("redis://redis.example.com:6379"),
		},
	}

	client := fake.NewSimpleClientset(pgSecret, mysqlSecret, redisSecret)

	v3spec := &SupportBundleSpec{
		Collectors: []*Collect{
			{
				Postgres: &Database{
					URI: StringOrValueFrom{
						ValueFrom: &ValueFromSource{
							SecretKeyRef: &SecretKeyRef{
								Name: "postgres-secret",
								Key:  "uri",
							},
						},
					},
				},
			},
			{
				Mysql: &Database{
					URI: StringOrValueFrom{
						ValueFrom: &ValueFromSource{
							SecretKeyRef: &SecretKeyRef{
								Name: "mysql-secret",
								Key:  "uri",
							},
						},
					},
				},
			},
			{
				Redis: &Database{
					URI: StringOrValueFrom{
						ValueFrom: &ValueFromSource{
							SecretKeyRef: &SecretKeyRef{
								Name: "redis-secret",
								Key:  "uri",
							},
						},
					},
				},
			},
		},
	}

	v2spec, err := ConvertToV1Beta2WithResolution(context.Background(), v3spec, client, "default")
	require.NoError(t, err)
	require.NotNil(t, v2spec)
	require.Len(t, v2spec.Collectors, 3)

	require.NotNil(t, v2spec.Collectors[0].Postgres)
	assert.Equal(t, "postgresql://user:pass@pg.example.com:5432/db", v2spec.Collectors[0].Postgres.URI)

	require.NotNil(t, v2spec.Collectors[1].Mysql)
	assert.Equal(t, "mysql://user:pass@mysql.example.com:3306/db", v2spec.Collectors[1].Mysql.URI)

	require.NotNil(t, v2spec.Collectors[2].Redis)
	assert.Equal(t, "redis://redis.example.com:6379", v2spec.Collectors[2].Redis.URI)
}

func TestConvertToV1Beta2WithResolution_SecretNotFound(t *testing.T) {
	client := fake.NewSimpleClientset()

	v3spec := &SupportBundleSpec{
		Collectors: []*Collect{
			{
				Postgres: &Database{
					URI: StringOrValueFrom{
						ValueFrom: &ValueFromSource{
							SecretKeyRef: &SecretKeyRef{
								Name: "nonexistent-secret",
								Key:  "uri",
							},
						},
					},
				},
			},
		},
	}

	_, err := ConvertToV1Beta2WithResolution(context.Background(), v3spec, client, "default")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to convert collector")
	assert.Contains(t, err.Error(), "failed to resolve database URI")
}

func TestConvertToV1Beta2WithResolution_PreservesCollectorMeta(t *testing.T) {
	client := fake.NewSimpleClientset()
	uri := "postgresql://user:pass@localhost:5432/db"

	v3spec := &SupportBundleSpec{
		Collectors: []*Collect{
			{
				Postgres: &Database{
					CollectorMeta: CollectorMeta{
						CollectorName: "my-postgres-collector",
					},
					URI: StringOrValueFrom{
						Value: &uri,
					},
					Parameters: []string{"sslmode=require"},
				},
			},
		},
	}

	v2spec, err := ConvertToV1Beta2WithResolution(context.Background(), v3spec, client, "default")
	require.NoError(t, err)
	require.NotNil(t, v2spec)
	require.Len(t, v2spec.Collectors, 1)
	require.NotNil(t, v2spec.Collectors[0].Postgres)

	assert.Equal(t, "my-postgres-collector", v2spec.Collectors[0].Postgres.CollectorName)
	assert.Equal(t, []string{"sslmode=require"}, v2spec.Collectors[0].Postgres.Parameters)
}

func TestConvertToV1Beta2WithResolution_TLSBackwardCompatibility(t *testing.T) {
	client := fake.NewSimpleClientset()
	uri := "postgresql://user:pass@localhost:5432/db"

	v3spec := &SupportBundleSpec{
		Collectors: []*Collect{
			{
				Postgres: &Database{
					URI: StringOrValueFrom{
						Value: &uri,
					},
					TLS: &TLSParams{
						SkipVerify: true,
						Secret: &TLSSecret{
							Name:      "old-tls-secret",
							Namespace: "default",
						},
					},
				},
			},
		},
	}

	v2spec, err := ConvertToV1Beta2WithResolution(context.Background(), v3spec, client, "default")
	require.NoError(t, err)
	require.NotNil(t, v2spec)
	require.Len(t, v2spec.Collectors, 1)
	require.NotNil(t, v2spec.Collectors[0].Postgres)
	require.NotNil(t, v2spec.Collectors[0].Postgres.TLS)

	assert.True(t, v2spec.Collectors[0].Postgres.TLS.SkipVerify)
	require.NotNil(t, v2spec.Collectors[0].Postgres.TLS.Secret)
	assert.Equal(t, "old-tls-secret", v2spec.Collectors[0].Postgres.TLS.Secret.Name)
	assert.Equal(t, "default", v2spec.Collectors[0].Postgres.TLS.Secret.Namespace)
}

func TestConvertToV1Beta2WithResolution_EmptySpec(t *testing.T) {
	client := fake.NewSimpleClientset()

	v3spec := &SupportBundleSpec{}

	v2spec, err := ConvertToV1Beta2WithResolution(context.Background(), v3spec, client, "default")
	require.NoError(t, err)
	require.NotNil(t, v2spec)
	assert.Nil(t, v2spec.Collectors)
}

func TestConvertDatabase_AllDatabaseTypes(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"uri": []byte("test-uri"),
		},
	}

	client := fake.NewSimpleClientset(secret)

	v3db := &Database{
		URI: StringOrValueFrom{
			ValueFrom: &ValueFromSource{
				SecretKeyRef: &SecretKeyRef{
					Name: "db-secret",
					Key:  "uri",
				},
			},
		},
	}

	// Test that the same database struct works for all DB types
	ctx := context.Background()

	pgDB, err := convertDatabase(ctx, v3db, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "test-uri", pgDB.URI)

	mysqlDB, err := convertDatabase(ctx, v3db, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "test-uri", mysqlDB.URI)

	mssqlDB, err := convertDatabase(ctx, v3db, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "test-uri", mssqlDB.URI)

	redisDB, err := convertDatabase(ctx, v3db, client, "default")
	require.NoError(t, err)
	assert.Equal(t, "test-uri", redisDB.URI)
}

// Helper function to convert v2spec back to ensure type compatibility
func ensureV2SpecCompatibility(v2spec *troubleshootv1beta2.SupportBundleSpec) {
	// This function just exists to ensure the types are compatible
	// If this compiles, we know the conversion produces valid v1beta2 types
	_ = v2spec.Uri
	_ = v2spec.Collectors
	_ = v2spec.Analyzers
}
