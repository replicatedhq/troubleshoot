package preflight

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPreflightSpecsRead(t *testing.T) {
	t.Parallel()

	// A very simple preflight spec
	preflightFile := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_gotest.yaml")
	//
	expectPreflightSpec := troubleshootv1beta2.Preflight{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Preflight",
			APIVersion: "troubleshoot.sh/v1beta2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "go-test-preflight",
		},
		Spec: troubleshootv1beta2.PreflightSpec{
			UploadResultsTo: "",
			Collectors: []*troubleshootv1beta2.Collect{
				&troubleshootv1beta2.Collect{
					Data: &troubleshootv1beta2.Data{
						CollectorMeta: troubleshootv1beta2.CollectorMeta{
							CollectorName: "",
						},
						Name: "example.json",
						Data: `{
  "foo": "bar",
  "stuff": {
    "foo": "bar",
    "bar": true
  },
  "morestuff": [
    {
      "foo": {
        "bar": 123
      }
    }
  ]
}
`,
					},
				},
			},
			RemoteCollectors: []*troubleshootv1beta2.RemoteCollect(nil),
			Analyzers: []*troubleshootv1beta2.Analyze{
				&troubleshootv1beta2.Analyze{
					JsonCompare: &troubleshootv1beta2.JsonCompare{
						AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
							CheckName: "Compare JSON Example",
						},
						CollectorName: "",
						FileName:      "example.json",
						Path:          "morestuff.[0].foo.bar",
						Value: `123
`,
						Outcomes: []*troubleshootv1beta2.Outcome{
							&troubleshootv1beta2.Outcome{
								Fail: &troubleshootv1beta2.SingleOutcome{
									When:    "false",
									Message: "The collected data does not match the value.",
								},
							},
							&troubleshootv1beta2.Outcome{
								Pass: &troubleshootv1beta2.SingleOutcome{
									When:    "true",
									Message: "The collected data matches the value.",
								},
							},
						},
					},
				},
			},
		},
	}
	expectHostPreflightSpec := troubleshootv1beta2.HostPreflight{}
	/*
			TypeMeta: metav1.TypeMeta{
				Kind:       "HostPreflight",
				APIVersion: "troubleshoot.sh/troubleshootv1beta2",
			},
			Spec: troubleshootv1beta2.HostPreflightSpec{
				Collectors:       []*troubleshootv1beta2.HostCollect(nil),
				RemoteCollectors: []*troubleshootv1beta2.RemoteCollect(nil),
				Analyzers:        []*troubleshootv1beta2.HostAnalyze(nil),
			},
		}
	*/
	expectUploadResultSpecs := []*troubleshootv1beta2.Preflight{}

	// A more complexed preflight spec, which resides in a secret
	preflightSecretFile := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_secret_gotest.yaml")
	//
	expectSecretPreflightSpec := troubleshootv1beta2.Preflight{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Preflight",
			APIVersion: "troubleshoot.sh/v1beta2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "preflight-sample",
		},
		Spec: troubleshootv1beta2.PreflightSpec{
			UploadResultsTo:  "",
			RemoteCollectors: []*troubleshootv1beta2.RemoteCollect(nil),
			Analyzers: []*troubleshootv1beta2.Analyze{
				&troubleshootv1beta2.Analyze{
					ClusterVersion: &troubleshootv1beta2.ClusterVersion{
						Outcomes: []*troubleshootv1beta2.Outcome{
							&troubleshootv1beta2.Outcome{
								Fail: &troubleshootv1beta2.SingleOutcome{
									When:    "< 1.16.0",
									Message: "The application requires at least Kubernetes 1.16.0, and recommends 1.18.0.",
									//Uri:     "https://kubernetes.io",
								},
							},
							&troubleshootv1beta2.Outcome{
								Warn: &troubleshootv1beta2.SingleOutcome{
									When:    "< 1.18.0",
									Message: "Your cluster meets the minimum version of Kubernetes, but we recommend you update to 1.18.0 or later.",
									//Uri:     "https://kubernetes.io",
								},
							},
							&troubleshootv1beta2.Outcome{
								Pass: &troubleshootv1beta2.SingleOutcome{
									Message: "Your cluster meets the recommended and required versions of Kubernetes.",
								},
							},
						},
					},
				},
				&troubleshootv1beta2.Analyze{
					NodeResources: &troubleshootv1beta2.NodeResources{
						AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
							CheckName: "Node Count Check",
						},
						Outcomes: []*troubleshootv1beta2.Outcome{
							&troubleshootv1beta2.Outcome{
								Fail: &troubleshootv1beta2.SingleOutcome{
									When:    "count() < 3",
									Message: "The cluster needs a minimum of 3 nodes.",
								},
							},
							&troubleshootv1beta2.Outcome{
								Pass: &troubleshootv1beta2.SingleOutcome{
									Message: "There are not enough nodes to run this application (3 or more)",
								},
							},
						},
					},
				},
			},
		},
	}
	expectSecretHostPreflightSpec := troubleshootv1beta2.HostPreflight{}
	expectSecretUploadResultSpecs := []*troubleshootv1beta2.Preflight{}

	tests := []struct {
		name string
		args []string
		//
		wantErr               bool
		wantPreflightSpec     *troubleshootv1beta2.Preflight
		wantHostPreflightSpec *troubleshootv1beta2.HostPreflight
		wantUploadResultSpecs []*troubleshootv1beta2.Preflight
	}{
		// TODOLATER: URL support? local mock webserver? would prefer for these tests to not require internet :)
		// TODOLATER: multidoc fixtures
		{
			name:                  "file-preflight",
			args:                  []string{preflightFile},
			wantErr:               false,
			wantPreflightSpec:     &expectPreflightSpec,
			wantHostPreflightSpec: &expectHostPreflightSpec,
			wantUploadResultSpecs: expectUploadResultSpecs,
		},
		{
			name:                  "file-secret",
			args:                  []string{preflightSecretFile},
			wantErr:               false,
			wantPreflightSpec:     &expectSecretPreflightSpec,
			wantHostPreflightSpec: &expectSecretHostPreflightSpec,
			wantUploadResultSpecs: expectSecretUploadResultSpecs,
		},
		/*
			{
				name: "stdin-preflight",
				args: []string{"-"},
				// TODO: how do we feed in stdin? contents of preflightFile
				wantErr:               false,
				wantPreflightSpec:     &expectPreflightSpec,
				wantHostPreflightSpec: &expectHostPreflightSpec,
				wantUploadResultSpecs: expectUploadResultSpecs,
			},
			{
				name: "stdin-secret",
				args: []string{"-"},
				// TODO: how do we feed in stdin? contents of preflightSecretFile
				wantErr:               false,
				wantPreflightSpec:     &expectSecretPreflightSpec,
				wantHostPreflightSpec: &expectSecretHostPreflightSpec,
				wantUploadResultSpecs: expectSecretUploadResultSpecs,
			},
		*/
		/* TODOLATER: needs a cluster with a spec installed?
		{
			name:     "cluster-secret",
			args:     []string{"/secret/some-secret-spec"},
			wantErr:  false,
			wantPreflightSpec:     &expectSecretPreflightSpec,
			wantHostPreflightSpec: &expectSecretHostPreflightSpec,
			wantUploadResultsSpecs: expectSecretUploadResultSpecs,
		},
		*/
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := PreflightSpecs{}
			tErr := specs.Read(tt.args)

			for _, v := range specs.PreflightSpec.Spec.Analyzers {
				fmt.Printf("%+v\n", *v)
			}

			if tt.wantErr {
				assert.Error(t, tErr)
			} else {
				require.NoError(t, tErr)
			}

			assert.Equal(t, specs.PreflightSpec, tt.wantPreflightSpec)
			assert.Equal(t, specs.HostPreflightSpec, tt.wantHostPreflightSpec)
			assert.Equal(t, specs.UploadResultSpecs, tt.wantUploadResultSpecs)
		})
	}
}
