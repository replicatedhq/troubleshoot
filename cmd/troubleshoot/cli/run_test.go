package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/httputil"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	testclient "k8s.io/client-go/kubernetes/fake"
)

func templSpec() string {
	return `
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
}

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

	orig := strings.ReplaceAll(templSpec(), "$MY_URI", srv.URL)

	ctx := context.Background()
	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{RawSpec: orig})
	require.NoError(t, err)
	require.NotNil(t, kinds)

	assert.Len(t, kinds.SupportBundlesV1Beta2, 1)
	assert.NotNil(t, kinds.SupportBundlesV1Beta2[0].Spec.Collectors[0].ConfigMap)
	err = loadSupportBundleSpecsFromURIs(ctx, kinds)
	require.NoError(t, err)

	require.Len(t, kinds.SupportBundlesV1Beta2, 1)
	assert.NotNil(t, kinds.SupportBundlesV1Beta2[0].Spec.Collectors[0].ClusterInfo)
}

func Test_loadMultipleSupportBundleSpecsWithNoURIs(t *testing.T) {
	ctx := context.Background()
	client := testclient.NewSimpleClientset()
	specs := []string{testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: sb-1
spec:
  collectors:
    - clusterInfo:{}`), testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: sb-2
  spec:
    collectors:
      - clusterInfo: {}`)}

	sb, _, err := loadSpecs(ctx, specs, client)
	require.NoError(t, err)
	require.Len(t, sb.Spec.Collectors, 2)
}

func Test_loadMultipleSupportBundleSpecsWithURIs(t *testing.T) {
	ctx := context.Background()
	client := testclient.NewSimpleClientset()

	specFile := testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: sb-file
spec:
  collectors:
    - logs:
        name: podlogs/kotsadm
        selector:
          - app=kotsadm
`)

	// Run a webserver to serve the spec
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: sb-uri
spec:
  collectors:
    - clusterInfo: {}`))
	}))
	defer srv.Close()

	orig := strings.ReplaceAll(templSpec(), "$MY_URI", srv.URL)
	specUri := testutils.ServeFromFilePath(t, orig)
	specs := []string{specFile, specUri}

	sb, _, err := loadSpecs(ctx, specs, client)
	require.NoError(t, err)
	assert.NotNil(t, sb.Spec.Collectors[0].Logs)
	assert.Nil(t, sb.Spec.Collectors[1].ConfigMap)      // original spec gone
	assert.NotNil(t, sb.Spec.Collectors[1].ClusterInfo) // new spec from URI
}

func Test_loadSupportBundleSpecsFromURIs_TimeoutError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // this will cause a timeout
	}))
	defer srv.Close()
	ctx := context.Background()

	kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{
		RawSpec: strings.ReplaceAll(templSpec(), "$MY_URI", srv.URL),
	})
	require.NoError(t, err)

	// Set the timeout on the http client to 500ms
	// The server sleeps for 2 seconds, so this should still timeout
	// supportbundle.LoadSupportBundleSpec does not yet use the context
	before := httputil.GetHttpClient().Timeout
	httputil.GetHttpClient().Timeout = 500 * time.Millisecond
	defer func() {
		// Reinstate the original timeout. Its a global var so we need to reset it
		httputil.GetHttpClient().Timeout = before
	}()

	beforeJSON, err := json.Marshal(kinds)
	require.NoError(t, err)

	err = loadSupportBundleSpecsFromURIs(ctx, kinds)
	require.NoError(t, err)

	afterJSON, err := json.Marshal(kinds)
	require.NoError(t, err)

	assert.JSONEq(t, string(beforeJSON), string(afterJSON))
}

func Test_loadSupportBundleSpecs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected troubleshootv1beta2.SupportBundleSpec
	}{
		{
			name: "empty collectors array in spec, default collectors added",
			args: []string{testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
spec:
  collectors: []
`)},
			expected: troubleshootv1beta2.SupportBundleSpec{
				Collectors: []*troubleshootv1beta2.Collect{
					{
						ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
					},
					{
						ClusterResources: &troubleshootv1beta2.ClusterResources{},
					},
				},
			},
		},
		{
			name: "no collectors defined in spec, default collectors added",
			args: []string{testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
spec:
  analyzers: []
`)},
			expected: troubleshootv1beta2.SupportBundleSpec{
				Collectors: []*troubleshootv1beta2.Collect{
					{
						ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
					},
					{
						ClusterResources: &troubleshootv1beta2.ClusterResources{},
					},
				},
				Analyzers: []*troubleshootv1beta2.Analyze{},
			},
		},
		{
			name: "collectors present but defaults missing, default collectors added",
			args: []string{testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
spec:
  collectors:
  - logs: {}
`)},
			expected: troubleshootv1beta2.SupportBundleSpec{
				Collectors: []*troubleshootv1beta2.Collect{
					{
						Logs: &troubleshootv1beta2.Logs{},
					},
					{
						ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
					},
					{
						ClusterResources: &troubleshootv1beta2.ClusterResources{},
					},
				},
			},
		},
		{
			name: "empty support bundle spec adds default collectors",
			args: []string{testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
`)},
			expected: troubleshootv1beta2.SupportBundleSpec{
				Collectors: []*troubleshootv1beta2.Collect{
					{
						ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
					},
					{
						ClusterResources: &troubleshootv1beta2.ClusterResources{},
					},
				},
			},
		},
		{
			name: "sb spec with host collectors does not add default collectors",
			args: []string{testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
spec:
  hostCollectors:
  - cpu: {}
`)},
			expected: troubleshootv1beta2.SupportBundleSpec{
				HostCollectors: []*troubleshootv1beta2.HostCollect{
					{
						CPU: &troubleshootv1beta2.CPU{},
					},
				},
			},
		},
		{
			name: "host collector spec with collectors does not add default collectors",
			args: []string{testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: HostCollector
spec:
  collectors:
  - cpu: {}
`)},
			expected: troubleshootv1beta2.SupportBundleSpec{
				HostCollectors: []*troubleshootv1beta2.HostCollect{
					{
						CPU: &troubleshootv1beta2.CPU{},
					},
				},
			},
		},
		{
			name: "sb spec with host and in-cluster collectors adds default collectors",
			args: []string{testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
spec:
  hostCollectors:
  - cpu: {}
  collectors:
  - logs: {}
`)},
			expected: troubleshootv1beta2.SupportBundleSpec{
				HostCollectors: []*troubleshootv1beta2.HostCollect{
					{
						CPU: &troubleshootv1beta2.CPU{},
					},
				},
				Collectors: []*troubleshootv1beta2.Collect{
					{
						Logs: &troubleshootv1beta2.Logs{},
					},
					{
						ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
					},
					{
						ClusterResources: &troubleshootv1beta2.ClusterResources{},
					},
				},
			},
		},
		{
			name: "sb spec with host collectors and empty in-cluster collectors does not default collectors",
			args: []string{testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
spec:
  hostCollectors:
  - cpu: {}
  collectors: []
`)},
			expected: troubleshootv1beta2.SupportBundleSpec{
				HostCollectors: []*troubleshootv1beta2.HostCollect{
					{
						CPU: &troubleshootv1beta2.CPU{},
					},
				},
				Collectors: []*troubleshootv1beta2.Collect{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			client := testclient.NewSimpleClientset()

			sb, _, err := loadSpecs(ctx, test.args, client)
			require.NoError(t, err)

			assert.Equal(t, test.expected, sb.Spec)
		})
	}
}

func Test_loadDuplicatedBundleSpecs(t *testing.T) {
	spec := testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: sb
spec:
  collectors:
  - helm: {}
  analyzers:
  - clusterVersion: {}
  hostCollectors:
  - cpu: {}
  hostAnalyzers:
  - cpu: {}
`)
	args := []string{spec, spec}

	ctx := context.Background()
	client := testclient.NewSimpleClientset()
	sb, _, err := loadSpecs(ctx, args, client)
	require.NoError(t, err)
	assert.Len(t, sb.Spec.Collectors, 1+2) // default clusterInfo + clusterResources
	assert.Len(t, sb.Spec.Analyzers, 1)
	assert.Len(t, sb.Spec.HostCollectors, 1)
	assert.Len(t, sb.Spec.HostAnalyzers, 1)
}

func Test_loadSpecsFromURL(t *testing.T) {
	// Run a webserver to serve the URI spec
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: sb-2
spec:
  collectors:
    - logs:
        name: podlogs/kotsadm
        selector:
          - app=kotsadm`))
	}))
	defer srv.Close()

	// update URI spec with the server URL
	orig := strings.ReplaceAll(templSpec(), "$MY_URI", srv.URL)

	// now create a webserver to serve the spec with URI
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(orig))
	}))
	defer srv.Close()

	fileSpec := testutils.ServeFromFilePath(t, `
apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata:
  name: sb
spec:
  collectors:
    - helm: {}`)

	// test and ensure that URI spec is not loaded
	ctx := context.Background()
	client := testclient.NewSimpleClientset()
	sb, _, err := loadSpecs(ctx, []string{fileSpec, srv.URL}, client)
	require.NoError(t, err)
	assert.Len(t, sb.Spec.Collectors, 2+2)            // default + clusterInfo + clusterResources
	assert.NotNil(t, sb.Spec.Collectors[0].Helm)      // come from the file spec
	assert.NotNil(t, sb.Spec.Collectors[1].ConfigMap) // come from the original spec
	assert.Nil(t, sb.Spec.Collectors[1].Logs)         // come from the URI spec
}

func Test_loadInvalidURISpec(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`invalid spec`))
	}))
	defer srv.Close()

	// update URI spec with the server URL
	orig := strings.ReplaceAll(templSpec(), "$MY_URI", srv.URL)
	specFile := testutils.ServeFromFilePath(t, orig)

	ctx := context.Background()
	client := testclient.NewSimpleClientset()
	sb, _, err := loadSpecs(ctx, []string{specFile}, client)
	require.NoError(t, err)
	assert.Len(t, sb.Spec.Collectors, 3)              // default + clusterInfo + clusterResources
	assert.NotNil(t, sb.Spec.Collectors[0].ConfigMap) // come from the original spec
}
