package loader

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestV1Beta3SupportBundlePreservesSharedSpec(t *testing.T) {
	const databaseV2 = `    - postgres:
        uri: connection-string
        tls:
          cacert: ca-cert
          clientCert: client-cert
          clientKey: client-key
`
	const databaseV3 = `    - postgres:
        uri:
          valueFrom:
            secretKeyRef:
              name: database
              key: uri
        tls:
          cacert:
            valueFrom:
              configMapKeyRef:
                name: database
                key: ca
          clientCert:
            value: client-cert
          clientKey:
            valueFrom:
              secretKeyRef:
                name: database
                key: key
`
	const spec = `apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: preservation
spec:
  collectors:
    - data:
        name: proof.txt
        data: marker
    - logs:
        name: app
        namespace: default
        selector: [app=test]
` + databaseV2 + `  analyzers:
    - textAnalyze:
        checkName: marker
        fileName: proof.txt
        regex: marker
        outcomes:
          - pass:
              when: "true"
              message: found
  hostCollectors:
    - cpu: {}
  hostAnalyzers:
    - cpu:
        checkName: cpu
        outcomes:
          - pass:
              when: "count > 0"
              message: found
  afterCollection:
    - uploadResultsTo:
        uri: https://example.com/upload
        method: POST
  uri: https://example.com/spec
  runHostCollectorsInPod: true
`
	l := specLoader{
		ctx:       context.Background(),
		namespace: "test",
		client: fake.NewSimpleClientset(
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "test"},
				Data:       map[string][]byte{"uri": []byte("connection-string"), "key": []byte("client-key")},
			},
			&corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "test"},
				Data:       map[string]string{"ca": "ca-cert"},
			},
		),
	}
	v2, err := l.loadFromStrings(spec)
	require.NoError(t, err)
	v3spec := strings.Replace(spec, "troubleshoot.sh/v1beta2", "troubleshoot.sh/v1beta3", 1)
	v3spec = strings.Replace(v3spec, databaseV2, databaseV3, 1)
	v3, err := l.loadFromStrings(v3spec)
	require.NoError(t, err)
	require.Len(t, v2.SupportBundlesV1Beta2, 1)
	require.Len(t, v3.SupportBundlesV1Beta2, 1)
	// Assert v1beta3-only reference resolution independently of the v1beta2 fixture.
	require.Len(t, v3.SupportBundlesV1Beta2[0].Spec.Collectors, 3)
	db := v3.SupportBundlesV1Beta2[0].Spec.Collectors[2].Postgres
	require.NotNil(t, db)
	require.Equal(t, "connection-string", db.URI)
	require.NotNil(t, db.TLS)
	require.Equal(t, "ca-cert", db.TLS.CACert)
	require.Equal(t, "client-cert", db.TLS.ClientCert)
	require.Equal(t, "client-key", db.TLS.ClientKey)
	require.Equal(t, v2.SupportBundlesV1Beta2[0].Spec, v3.SupportBundlesV1Beta2[0].Spec)
}
