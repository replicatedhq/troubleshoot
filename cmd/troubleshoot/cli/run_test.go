package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/troubleshoot/pkg/httputil"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var orig = `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: sb-1
spec:
  uri: $MY_URI
  collectors:
    - configMap:
        name: kube-root-ca.crt
        namespace: default
`

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

	orig := strings.ReplaceAll(orig, "$MY_URI", srv.URL)

	ctx := context.Background()
	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{RawSpec: orig})
	require.NoError(t, err)
	require.NotNil(t, kinds)

	moreKinds, err := loadSupportBundleSpecsFromURIs(ctx, kinds)
	require.NoError(t, err)

	require.Len(t, moreKinds.SupportBundlesV1Beta2, 1)
	assert.NotNil(t, moreKinds.SupportBundlesV1Beta2[0].Spec.Collectors[0].ClusterInfo)
}

func Test_loadSupportBundleSpecsFromURIs_TimeoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // this will cause a timeout
	}))
	defer srv.Close()
	ctx := context.Background()

	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{
		RawSpec: strings.ReplaceAll(orig, "$MY_URI", srv.URL),
	})
	require.NoError(t, err)

	// Set the timeout on the http client to 10ms
	// supportbundle.LoadSupportBundleSpec does not yet use the context
	before := httputil.GetHttpClient().Timeout
	httputil.GetHttpClient().Timeout = 10 * time.Millisecond
	defer func() {
		// Reinstate the original timeout. Its a global var so we need to reset it
		httputil.GetHttpClient().Timeout = before
	}()

	kindsAfter, err := loadSupportBundleSpecsFromURIs(ctx, kinds)
	require.NoError(t, err)

	assert.Equal(t, kinds, kindsAfter)
}
