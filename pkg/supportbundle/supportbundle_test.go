package supportbundle

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func Test_LoadAndConcatSpec(t *testing.T) {

	bundle1doc, err := LoadSupportBundleSpec("test/supportbundle1.yaml")
	if err != nil {
		t.Error("couldn't load bundle1 from file")
	}

	bundle2doc, err := LoadSupportBundleSpec("test/supportbundle2.yaml")
	if err != nil {
		t.Error("couldn't load bundle2 from file")
	}

	bundle1, err := ParseSupportBundleFromDoc(bundle1doc)
	if err != nil {
		t.Error("couldn't parse bundle 1")
	}

	bundle2, err := ParseSupportBundleFromDoc(bundle2doc)
	if err != nil {
		t.Error("couldn't parse bundle 2")
	}

	fulldoc, err := LoadSupportBundleSpec("test/completebundle.yaml")
	if err != nil {
		t.Error("couldn't load full bundle from file")
	}

	fullbundle, err := ParseSupportBundleFromDoc(fulldoc)
	if err != nil {
		t.Error("couldn't parse full bundle")
	}

	bundle3 := ConcatSpec(bundle1, bundle2)

	if reflect.DeepEqual(fullbundle, bundle3) == false {
		t.Error("Full bundle and concatenated bundle are not the same.")
	}

}

func Test_LoadAndConcatSpec_WithNil(t *testing.T) {
	var bundle *troubleshootv1beta2.SupportBundle
	// both function arguments are nil
	bundle4 := ConcatSpec(bundle, bundle)
	if reflect.DeepEqual(bundle4, (*troubleshootv1beta2.SupportBundle)(nil)) == false {
		t.Error("concatenating nil pointer with nil pointer has error.")
	}

	fulldoc, _ := LoadSupportBundleSpec("test/completebundle.yaml")
	bundle, _ = ParseSupportBundleFromDoc(fulldoc)

	// targetBundle is nil pointer
	bundle5 := ConcatSpec((*troubleshootv1beta2.SupportBundle)(nil), bundle)
	if reflect.DeepEqual(bundle5, bundle) == false {
		t.Error("concatenating targetBundle of nil pointer has error.")
	}

	// sourceBundle is nil pointer
	bundle6 := ConcatSpec(bundle, (*troubleshootv1beta2.SupportBundle)(nil))
	if reflect.DeepEqual(bundle6, bundle) == false {
		t.Error("concatenating sourceBundle of nil pointer has error.")
	}
}

func Test_getNodeList(t *testing.T) {
	tests := []struct {
		name        string
		clientset   kubernetes.Interface
		opts        SupportBundleCreateOpts
		expected    *NodeList
		expectError bool
	}{
		{
			name: "successful node list",
			clientset: testclient.NewSimpleClientset(
				&corev1.NodeList{
					Items: []corev1.Node{
						{ObjectMeta: metav1.ObjectMeta{Name: "node1"}},
						{ObjectMeta: metav1.ObjectMeta{Name: "node2"}},
					},
				},
			),
			opts: SupportBundleCreateOpts{},
			expected: &NodeList{
				Nodes: []string{"node1", "node2"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodeList, err := getNodeList(tt.clientset, tt.opts)
			if (err != nil) != tt.expectError {
				t.Errorf("getNodeList() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if !reflect.DeepEqual(nodeList, tt.expected) {
				t.Errorf("getNodeList() = %v, expected %v", nodeList, tt.expected)
			}
		})
	}
}

func Test_saveAndRedactFinalSpec(t *testing.T) {
	spec := &troubleshootv1beta2.SupportBundleSpec{
		Collectors: []*troubleshootv1beta2.Collect{
			{
				ClusterInfo:      &troubleshootv1beta2.ClusterInfo{},
				ClusterResources: &troubleshootv1beta2.ClusterResources{},
				Postgres: &troubleshootv1beta2.Database{
					URI: "postgresql://user:password@hostname:5432/defaultdb?sslmode=require",
					TLS: &troubleshootv1beta2.TLSParams{
						CACert:     `CA CERT`,
						ClientCert: `CLIENT CERT`,
						ClientKey:  `PRIVATE KEY`,
					},
				},
				HTTP: &troubleshootv1beta2.HTTP{
					Get: &troubleshootv1beta2.Get{
						URL: "http:api:3000/healthz",
						TLS: &troubleshootv1beta2.TLSParams{
							ClientKey: `PRIVATE KEY`,
						},
					},
				},
			},
		},
	}

	result := make(collect.CollectorResult)
	bundlePath := t.TempDir()
	expectedYAML := `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  creationTimestamp: null
spec:
  collectors:
  - clusterInfo: {}
    clusterResources: {}
    postgres:
      uri: postgresql://***HIDDEN***:***HIDDEN***@***HIDDEN***:5432/***HIDDEN***
      tls:
        cacert: CA CERT
        clientCert: CLIENT CERT
        clientKey: "***HIDDEN***"
    http:
      get:
        url: http:api:3000/healthz
        tls:
          clientKey: "***HIDDEN***"
status: {}
`

	err := saveAndRedactFinalSpec(spec, &result, bundlePath, nil)
	if err != nil {
		t.Fatal(err)
	}

	actualYAML, err := os.ReadFile(filepath.Join(bundlePath, constants.SPEC_FILENAME))
	require.NoError(t, err)

	var expectedData, actualData interface{}
	err = yaml.Unmarshal([]byte(expectedYAML), &expectedData)
	require.NoError(t, err)
	err = yaml.Unmarshal(actualYAML, &actualData)
	require.NoError(t, err)
	assert.Equal(t, expectedData, actualData)
}
