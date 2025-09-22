package loader

import (
	"context"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLoadingHelmTemplate_Succeeds(t *testing.T) {
	s := testutils.GetTestFixture(t, "yamldocs/helm-template.yaml")
	l := specLoader{}
	kinds, err := l.loadFromStrings(s)
	require.NoError(t, err)
	require.NotNil(t, kinds)

	assert.Len(t, kinds.AnalyzersV1Beta2, 0)
	assert.Len(t, kinds.CollectorsV1Beta2, 0)
	assert.Len(t, kinds.HostCollectorsV1Beta2, 0)
	assert.Len(t, kinds.HostPreflightsV1Beta2, 0)
	assert.Len(t, kinds.RemoteCollectorsV1Beta2, 0)
	require.Len(t, kinds.PreflightsV1Beta2, 2)
	require.Len(t, kinds.RedactorsV1Beta2, 1)
	require.Len(t, kinds.SupportBundlesV1Beta2, 3)

	// Assert a few fields from the loaded troubleshoot specs
	assert.Equal(t, "redactor-spec-1", kinds.RedactorsV1Beta2[0].ObjectMeta.Name)
	assert.Equal(t, "REDACT SECOND TEXT PLEASE", kinds.RedactorsV1Beta2[0].Spec.Redactors[0].Removals.Values[0])
	assert.Equal(t, "sb-spec-1", kinds.SupportBundlesV1Beta2[0].Name)
	assert.Equal(t, "sb-spec-2", kinds.SupportBundlesV1Beta2[1].Name)
	assert.Equal(t, "sb-spec-3", kinds.SupportBundlesV1Beta2[2].Name)
	assert.Equal(t, false, kinds.SupportBundlesV1Beta2[0].Spec.RunHostCollectorsInPod)
	assert.Equal(t, true, kinds.SupportBundlesV1Beta2[2].Spec.RunHostCollectorsInPod)
	assert.Equal(t, "wg-easy", kinds.SupportBundlesV1Beta2[1].Spec.Collectors[0].Logs.CollectorName)
	assert.Equal(t, "Node Count Check", kinds.PreflightsV1Beta2[0].Spec.Analyzers[0].NodeResources.CheckName)
	assert.Len(t, kinds.PreflightsV1Beta2[0].Spec.Collectors, 0)
	assert.Equal(t, true, kinds.PreflightsV1Beta2[1].Spec.Collectors[0].ClusterResources.IgnoreRBAC)
}

func TestLoadingRandomValidYaml_IgnoreDoc(t *testing.T) {
	tests := []string{
		"",
		"---",
		"configVersion: v1",
		`
array:
  - 1
  - 2
`,
	}

	for _, ts := range tests {
		ctx := context.Background()
		kinds, err := LoadSpecs(ctx, LoadOptions{RawSpecs: []string{ts}})
		assert.NoError(t, err)
		assert.Equal(t, NewTroubleshootKinds(), kinds)
	}
}

func TestLoadingInvalidYaml_ReturnError(t *testing.T) {
	tests := []string{
		"@",
		"-",
		`
array:- 1
  - 2
`,
	}

	for _, ts := range tests {
		t.Run(ts, func(t *testing.T) {
			ctx := context.Background()
			kinds, err := LoadSpecs(ctx, LoadOptions{RawSpec: ts, Strict: true})
			assert.Error(t, err)
			assert.Nil(t, kinds)

			kinds, err = LoadSpecs(ctx, LoadOptions{RawSpec: ts, Strict: false})
			assert.NoError(t, err)
			assert.Equal(t, NewTroubleshootKinds(), kinds)
		})
	}
}

func TestLoadingInvalidYaml_IgnoreDocs(t *testing.T) {
	s := testutils.GetTestFixture(t, "yamldocs/multidoc-spec-with-invalids.yaml")
	ctx := context.Background()
	kinds, err := LoadSpecs(ctx, LoadOptions{RawSpec: s})
	require.NoError(t, err)
	require.NotNil(t, kinds)

	assert.Equal(t, &TroubleshootKinds{
		CollectorsV1Beta2: []troubleshootv1beta2.Collector{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Collector",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				Spec: troubleshootv1beta2.CollectorSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							ClusterResources: &troubleshootv1beta2.ClusterResources{
								CollectorMeta: troubleshootv1beta2.CollectorMeta{
									CollectorName: "my-cluster-resources",
								},
							},
						},
					},
				},
			},
		},
		SupportBundlesV1Beta2: []troubleshootv1beta2.SupportBundle{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SupportBundle",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				Spec: troubleshootv1beta2.SupportBundleSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							Logs: &troubleshootv1beta2.Logs{
								Name: "all-logs",
							},
						},
					},
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SupportBundle",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				Spec: troubleshootv1beta2.SupportBundleSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
						},
					},
				},
			},
		},
		PreflightsV1Beta2: []troubleshootv1beta2.Preflight{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Preflight",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "preflight-1",
				},
				Spec: troubleshootv1beta2.PreflightSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							ClusterResources: &troubleshootv1beta2.ClusterResources{},
						},
					},
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Preflight",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "preflight-2",
				},
				Spec: troubleshootv1beta2.PreflightSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							ClusterResources: &troubleshootv1beta2.ClusterResources{},
						},
					},
				},
			},
		},
	}, kinds)
}

func TestLoadingMultidocsWithTroubleshootSpecs(t *testing.T) {
	s := testutils.GetTestFixture(t, "yamldocs/multidoc-spec-1.yaml")
	ctx := context.Background()
	kinds, err := LoadSpecs(ctx, LoadOptions{RawSpec: s})
	require.NoError(t, err)
	require.NotNil(t, kinds)

	assert.Equal(t, []troubleshootv1beta2.Analyzer{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Analyzer",
				APIVersion: "troubleshoot.sh/v1beta2",
			},
			Spec: troubleshootv1beta2.AnalyzerSpec{
				Analyzers: []*troubleshootv1beta2.Analyze{
					{
						ClusterVersion: &troubleshootv1beta2.ClusterVersion{},
					},
				},
				HostAnalyzers: []*troubleshootv1beta2.HostAnalyze{
					{
						TCPLoadBalancer: &troubleshootv1beta2.TCPLoadBalancerAnalyze{},
					},
				},
			},
		},
	}, kinds.AnalyzersV1Beta2)

	assert.Equal(t, []troubleshootv1beta2.Collector{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Collector",
				APIVersion: "troubleshoot.sh/v1beta2",
			},
			Spec: troubleshootv1beta2.CollectorSpec{
				Collectors: []*troubleshootv1beta2.Collect{
					{
						ClusterResources: &troubleshootv1beta2.ClusterResources{
							CollectorMeta: troubleshootv1beta2.CollectorMeta{
								CollectorName: "my-cluster-resources",
							},
						},
					},
				},
			},
		},
	}, kinds.CollectorsV1Beta2)

	assert.Equal(t, []troubleshootv1beta2.HostCollector{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "HostCollector",
				APIVersion: "troubleshoot.sh/v1beta2",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-host-collector",
			},
			Spec: troubleshootv1beta2.HostCollectorSpec{
				Collectors: []*troubleshootv1beta2.HostCollect{
					{
						CPU: &troubleshootv1beta2.CPU{},
					},
				},
			},
		},
	}, kinds.HostCollectorsV1Beta2)

	assert.Equal(t, []troubleshootv1beta2.HostPreflight{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "HostPreflight",
				APIVersion: "troubleshoot.sh/v1beta2",
			},
			Spec: troubleshootv1beta2.HostPreflightSpec{
				RemoteCollectors: []*troubleshootv1beta2.RemoteCollect{
					{
						Certificate: &troubleshootv1beta2.RemoteCertificate{
							CertificatePath: "/etc/ssl/corp.crt",
							KeyPath:         "/etc/ssl/corp.key",
						},
					},
				},
				Analyzers: []*troubleshootv1beta2.HostAnalyze{
					{
						Certificate: &troubleshootv1beta2.CertificateAnalyze{
							Outcomes: []*troubleshootv1beta2.Outcome{
								{
									Pass: &troubleshootv1beta2.SingleOutcome{
										Message: "Certificate key pair is valid",
									},
								},
							},
						},
					},
				},
			},
		},
	}, kinds.HostPreflightsV1Beta2)

	assert.Equal(t, []troubleshootv1beta2.Preflight{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Preflight",
				APIVersion: "troubleshoot.sh/v1beta2",
			},
			Spec: troubleshootv1beta2.PreflightSpec{
				Collectors: []*troubleshootv1beta2.Collect{
					{
						Data: &troubleshootv1beta2.Data{
							Data: "5",
						},
					},
				},
				Analyzers: []*troubleshootv1beta2.Analyze{
					{
						ClusterVersion: &troubleshootv1beta2.ClusterVersion{},
					},
				},
			},
		},
	}, kinds.PreflightsV1Beta2)

	assert.Equal(t, []troubleshootv1beta2.Redactor{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Redactor",
				APIVersion: "troubleshoot.sh/v1beta2",
			},
			Spec: troubleshootv1beta2.RedactorSpec{
				Redactors: []*troubleshootv1beta2.Redact{
					{
						Name: "replace password",
						FileSelector: troubleshootv1beta2.FileSelector{
							File: "data/my-password-dump",
						},
						Removals: troubleshootv1beta2.Removals{
							Values: []string{"abc123"},
						},
					},
				},
			},
		},
	}, kinds.RedactorsV1Beta2)

	assert.Equal(t, []troubleshootv1beta2.RemoteCollector{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "RemoteCollector",
				APIVersion: "troubleshoot.sh/v1beta2",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "certificate",
			},
			Spec: troubleshootv1beta2.RemoteCollectorSpec{
				Collectors: []*troubleshootv1beta2.RemoteCollect{
					{
						CPU: &troubleshootv1beta2.RemoteCPU{},
					},
				},
			},
		},
	}, kinds.RemoteCollectorsV1Beta2)

	assert.Equal(t, []troubleshootv1beta2.SupportBundle{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "SupportBundle",
				APIVersion: "troubleshoot.sh/v1beta2",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "my-support-bundle",
			},
			Spec: troubleshootv1beta2.SupportBundleSpec{
				Collectors: []*troubleshootv1beta2.Collect{
					{
						Logs: &troubleshootv1beta2.Logs{
							Name: "all-logs",
						},
					},
				},
				HostCollectors: []*troubleshootv1beta2.HostCollect{
					{
						HostOS: &troubleshootv1beta2.HostOS{},
					},
				},
				Analyzers: []*troubleshootv1beta2.Analyze{
					{
						ClusterVersion: &troubleshootv1beta2.ClusterVersion{},
					},
				},
			},
		},
	}, kinds.SupportBundlesV1Beta2)
}

func TestLoadingV1Beta1CollectorSpec(t *testing.T) {
	kinds, err := LoadSpecs(context.Background(), LoadOptions{RawSpec: `kind: Collector
apiVersion: troubleshoot.replicated.com/v1beta1
metadata:
  name: collector-sample
spec:
  collectors:
    - clusterInfo: {}
`})
	require.NoError(t, err)
	require.NotNil(t, kinds)

	assert.Equal(t, []troubleshootv1beta2.Collector{
		{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Collector",
				APIVersion: "troubleshoot.sh/v1beta2",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "collector-sample",
			},
			Spec: troubleshootv1beta2.CollectorSpec{
				Collectors: []*troubleshootv1beta2.Collect{
					{
						ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
					},
				},
			},
		},
	}, kinds.CollectorsV1Beta2)
}

func TestLoadingConfigMapWithMultipleSpecs_PreflightSupportBundleAndRedactorDataKeys(t *testing.T) {
	s := testutils.GetTestFixture(t, "yamldocs/multidoc-spec-2.yaml")
	l := specLoader{}
	kinds, err := l.loadFromStrings(s)
	require.NoError(t, err)
	require.NotNil(t, kinds)

	assert.Equal(t, &TroubleshootKinds{
		SupportBundlesV1Beta2: []troubleshootv1beta2.SupportBundle{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SupportBundle",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				Spec: troubleshootv1beta2.SupportBundleSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							Logs: &troubleshootv1beta2.Logs{
								Name: "all-logs",
							},
						},
					},
				},
			},
		},
		RedactorsV1Beta2: []troubleshootv1beta2.Redactor{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Redactor",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				Spec: troubleshootv1beta2.RedactorSpec{
					Redactors: []*troubleshootv1beta2.Redact{
						{
							Name: "redact-text-1",
							Removals: troubleshootv1beta2.Removals{
								Values: []string{"abc123"},
							},
						},
					},
				},
			},
		},
		PreflightsV1Beta2: []troubleshootv1beta2.Preflight{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Preflight",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				Spec: troubleshootv1beta2.PreflightSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							ClusterResources: &troubleshootv1beta2.ClusterResources{
								IgnoreRBAC: true,
							},
						},
					},
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{
								Outcomes: []*troubleshootv1beta2.Outcome{
									{
										Pass: &troubleshootv1beta2.SingleOutcome{
											Message: "Cluster is up to date",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}, kinds)
}

func TestLoadingConfigMapWithMultipleSpecs_SupportBundleMultidoc(t *testing.T) {
	s := testutils.GetTestFixture(t, "yamldocs/multidoc-spec-3.yaml")
	l := specLoader{}
	kinds, err := l.loadFromStrings(s)
	require.NoError(t, err)
	require.NotNil(t, kinds)

	assert.Equal(t, &TroubleshootKinds{
		SupportBundlesV1Beta2: []troubleshootv1beta2.SupportBundle{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SupportBundle",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				Spec: troubleshootv1beta2.SupportBundleSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							Logs: &troubleshootv1beta2.Logs{
								Name: "all-logs",
							},
						},
					},
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SupportBundle",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				Spec: troubleshootv1beta2.SupportBundleSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							ClusterResources: &troubleshootv1beta2.ClusterResources{},
						},
					},
				},
			},
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SupportBundle",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				Spec: troubleshootv1beta2.SupportBundleSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							ClusterInfo: &troubleshootv1beta2.ClusterInfo{},
						},
					},
				},
			},
		},
	}, kinds)
}

func TestKindsIsEmpty(t *testing.T) {
	assert.True(t, NewTroubleshootKinds().IsEmpty())
	kinds := NewTroubleshootKinds()
	kinds.AnalyzersV1Beta2 = append(kinds.AnalyzersV1Beta2, troubleshootv1beta2.Analyzer{})
	assert.False(t, kinds.IsEmpty())
}

func TestAddingKinds(t *testing.T) {
	a := troubleshootv1beta2.Analyzer{
		Spec: troubleshootv1beta2.AnalyzerSpec{},
	}
	k1 := NewTroubleshootKinds()
	k1.Add(&TroubleshootKinds{
		AnalyzersV1Beta2: []troubleshootv1beta2.Analyzer{a},
	})

	k2 := &TroubleshootKinds{
		AnalyzersV1Beta2: []troubleshootv1beta2.Analyzer{a},
	}
	assert.Equal(t, k2, k1)
}

func TestToYaml(t *testing.T) {
	k := &TroubleshootKinds{
		AnalyzersV1Beta2: []troubleshootv1beta2.Analyzer{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Analyzer",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				Spec: troubleshootv1beta2.AnalyzerSpec{
					Analyzers: []*troubleshootv1beta2.Analyze{
						{
							ClusterVersion: &troubleshootv1beta2.ClusterVersion{
								Outcomes: []*troubleshootv1beta2.Outcome{
									{
										Pass: &troubleshootv1beta2.SingleOutcome{
											Message: "Cluster is up to date",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		SupportBundlesV1Beta2: []troubleshootv1beta2.SupportBundle{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SupportBundle",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				Spec: troubleshootv1beta2.SupportBundleSpec{
					Collectors: []*troubleshootv1beta2.Collect{
						{
							ClusterResources: &troubleshootv1beta2.ClusterResources{
								IgnoreRBAC: true,
							},
						},
					},
				},
			},
		},
	}

	y, err := k.ToYaml()
	require.NoError(t, err)
	assert.Contains(t, string(y), `apiVersion: troubleshoot.sh/v1beta2
kind: SupportBundle
metadata: {}
spec:
  collectors:
  - clusterResources:
      ignoreRBAC: true`)
	assert.Contains(t, string(y), "message: Cluster is up to date")
}

func TestLoadingEmptySpec(t *testing.T) {
	s := testutils.GetTestFixture(t, "supportbundle/empty.yaml")
	kinds, err := LoadSpecs(context.Background(), LoadOptions{RawSpec: s})
	require.NoError(t, err)
	require.NotNil(t, kinds)

	assert.Equal(t, &TroubleshootKinds{
		SupportBundlesV1Beta2: []troubleshootv1beta2.SupportBundle{
			{
				TypeMeta: metav1.TypeMeta{
					Kind:       "SupportBundle",
					APIVersion: "troubleshoot.sh/v1beta2",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "empty",
				},
			},
		},
	}, kinds)
}
