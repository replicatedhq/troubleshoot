package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_loadSupportBundleSpecsFromURIs(t *testing.T) {
	// Run a webserver to serve the spec
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: sb-2
spec:
  collectors:
    - clusterInfo: {}`))
	}))
	defer srv.Close()

	orig := `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: sb-1
spec:
  uri: ` + srv.URL + `
  collectors:
    - configMap:
        name: kube-root-ca.crt
        namespace: default
`

	ctx := context.Background()
	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{RawSpec: orig})
	require.NoError(t, err)
	require.NotNil(t, kinds)

	moreKinds, err := loadSupportBundleSpecsFromURIs(ctx, kinds)
	require.NoError(t, err)

	require.Len(t, moreKinds.SupportBundlesV1Beta2, 1)
	assert.NotNil(t, moreKinds.SupportBundlesV1Beta2[0].Spec.Collectors[0].ClusterInfo)
}
