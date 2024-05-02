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

	assert.Len(t, kinds.SupportBundlesV1Beta2, 1)
	assert.NotNil(t, kinds.SupportBundlesV1Beta2[0].Spec.Collectors[0].ConfigMap)
	err = loadSupportBundleSpecsFromURIs(ctx, kinds)
	require.NoError(t, err)

	require.Len(t, kinds.SupportBundlesV1Beta2, 1)
	assert.NotNil(t, kinds.SupportBundlesV1Beta2[0].Spec.Collectors[0].ClusterInfo)
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
