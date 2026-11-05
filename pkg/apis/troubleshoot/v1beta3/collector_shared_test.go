package v1beta3

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	v1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSharedCollectorsSurviveConversion(t *testing.T) {
	// Derive the cases from the canonical schema so newly added collectors
	// automatically participate in the preservation check.
	typ := reflect.TypeOf(v1beta2.Collect{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		switch name {
		case "postgres", "mysql", "mssql", "redis", "clickhouse":
			continue
		}
		t.Run(name, func(t *testing.T) {
			input := []byte(`{"` + name + `":{}}`)
			var original v1beta2.Collect
			require.NoError(t, json.Unmarshal(input, &original))
			var collector Collect
			require.NoError(t, json.Unmarshal(input, &collector))
			converted, err := convertCollector(context.Background(), &collector, nil, "")
			require.NoError(t, err)
			require.Equal(t, &original, converted)
			encoded, err := json.Marshal(collector.DeepCopy())
			require.NoError(t, err)
			var roundTrip Collect
			require.NoError(t, json.Unmarshal(encoded, &roundTrip))
			require.Equal(t, collector, roundTrip)
		})
	}
}

func TestCollectorDatabaseAndSharedFieldsRoundTrip(t *testing.T) {
	for _, name := range []string{"postgres", "mysql", "mssql", "redis", "clickhouse"} {
		t.Run(name, func(t *testing.T) {
			input := `{"data":{"name":"proof.txt","data":"marker"},"` + name + `":{"uri":{"value":"connection-string"}}}`
			var collector Collect
			require.NoError(t, json.Unmarshal([]byte(input), &collector))
			encoded, err := json.Marshal(collector.DeepCopy())
			require.NoError(t, err)
			require.JSONEq(t, input, string(encoded))
			converted, err := convertCollector(context.Background(), &collector, nil, "")
			require.NoError(t, err)
			encoded, err = json.Marshal(converted)
			require.NoError(t, err)
			require.JSONEq(t, `{"data":{"name":"proof.txt","data":"marker"},"`+name+`":{"uri":"connection-string"}}`, string(encoded))
		})
	}
}

func TestCollectorDatabaseReferencesRoundTrip(t *testing.T) {
	for _, name := range []string{"postgres", "mysql", "mssql", "redis", "clickhouse"} {
		t.Run(name, func(t *testing.T) {
			client := fake.NewSimpleClientset(
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "test"},
					Data:       map[string][]byte{"uri": []byte("connection-string"), "key": []byte("client-key")},
				},
				&corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{Name: "database", Namespace: "test"},
					Data:       map[string]string{"ca": "ca-cert"},
				},
			)
			input := `{"data":{"name":"proof.txt","data":"marker"},"` + name + `":{
				"uri":{"valueFrom":{"secretKeyRef":{"name":"database","key":"uri"}}},
				"tls":{
					"cacert":{"valueFrom":{"configMapKeyRef":{"name":"database","key":"ca"}}},
					"clientCert":{"value":"client-cert"},
					"clientKey":{"valueFrom":{"secretKeyRef":{"name":"database","key":"key"}}}
				}
			}}`
			var collector Collect
			require.NoError(t, json.Unmarshal([]byte(input), &collector))
			encoded, err := json.Marshal(collector.DeepCopy())
			require.NoError(t, err)
			// Compare to the original v1beta3 input before resolving references.
			require.JSONEq(t, input, string(encoded))
			var roundTrip Collect
			require.NoError(t, json.Unmarshal(encoded, &roundTrip))
			converted, err := convertCollector(context.Background(), &roundTrip, client, "test")
			require.NoError(t, err)
			encoded, err = json.Marshal(converted)
			require.NoError(t, err)
			require.JSONEq(t, `{"data":{"name":"proof.txt","data":"marker"},"`+name+`":{
				"uri":"connection-string",
				"tls":{"cacert":"ca-cert","clientCert":"client-cert","clientKey":"client-key"}
			}}`, string(encoded))
		})
	}
}
