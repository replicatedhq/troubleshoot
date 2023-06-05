package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestSplitMultidoc_Succees(t *testing.T) {
	multidoc := `apiVersion: v1
kind: ConfigMap
metadata:
  name: sb-spec-2-configmap
  labels:
    troubleshoot.io/kind: support-bundle
data:
  support-bundle-spec: |-
    apiVersion: troubleshoot.sh/v1beta2
    kind: SupportBundle
    spec:
      collectors:
        - logs:
            name: all-logs
    ---
    apiVersion: troubleshoot.sh/v1beta2
    kind: SupportBundle
    spec:
      collectors:
        - clusterResources: {}
---
kind: Tree
---
data:
  value: "5"
  val2: "6"
---
string
---
2
---
`
	docs, err := SplitMultiDoc(multidoc)
	require.NoError(t, err)
	require.Len(t, docs, 6)

	assert.Equal(t, "kind: Tree\n", string(docs[1]))
	assert.Equal(t, "null\n", string(docs[5]))

	// Check the configmap
	out := map[string]any{}
	require.NoError(t, yaml.Unmarshal([]byte(docs[0]), out))

	assert.Equal(t, map[string]any{
		"apiVersion": "v1",
		"data": map[any]any{
			"support-bundle-spec": `apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
spec:
  collectors:
    - logs:
        name: all-logs
---
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
spec:
  collectors:
    - clusterResources: {}`},
		"kind": "ConfigMap",
		"metadata": map[any]any{
			"labels": map[any]any{
				"troubleshoot.io/kind": "support-bundle",
			},
			"name": "sb-spec-2-configmap",
		},
	}, out)
}
