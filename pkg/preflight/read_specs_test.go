package preflight

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PreflightSpecsReadTests []PreflightSpecsReadTest

type PreflightSpecsReadTest struct {
	name          string
	args          []string
	customStdin   bool
	stdinDataFile string
	//
	wantErr           bool
	wantPreflightSpec *troubleshootv1beta2.Preflight
	// TODOLATER: tests around this
	wantHostPreflightSpec *troubleshootv1beta2.HostPreflight
	// TODOLATER: tests around this
	wantUploadResultSpecs []troubleshootv1beta2.Preflight
}

// TODO: Simplify tests and rely on the loader tests
func TestPreflightSpecsRead(t *testing.T) {
	// NOTE: don't use t.Parallel(), these tests manipulate os.Stdin

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
				{
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
				{
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
							{
								Fail: &troubleshootv1beta2.SingleOutcome{
									When:    "false",
									Message: "The collected data does not match the value.",
								},
							},
							{
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

	// A HostPreflight spec
	hostpreflightFile := filepath.Join(testutils.FileDir(), "../../examples/preflight/host/block-devices.yaml")
	//
	expectHostPreflightSpec := troubleshootv1beta2.HostPreflight{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HostPreflight",
			APIVersion: "troubleshoot.sh/v1beta2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "block",
		},
		Spec: troubleshootv1beta2.HostPreflightSpec{
			Collectors: []*troubleshootv1beta2.HostCollect{
				{
					BlockDevices: &troubleshootv1beta2.HostBlockDevices{},
				},
			},
			RemoteCollectors: []*troubleshootv1beta2.RemoteCollect(nil),
			Analyzers: []*troubleshootv1beta2.HostAnalyze{
				{
					BlockDevices: &troubleshootv1beta2.BlockDevicesAnalyze{
						Outcomes: []*troubleshootv1beta2.Outcome{
							{
								Pass: &troubleshootv1beta2.SingleOutcome{
									When:    ".* == 1",
									Message: "One available block device",
								},
							},
							{
								Pass: &troubleshootv1beta2.SingleOutcome{
									When:    ".* > 1",
									Message: "Multiple available block devices",
								},
							},
							{
								Fail: &troubleshootv1beta2.SingleOutcome{
									Message: "No available block devices",
								},
							},
						},
					},
				},
			},
		},
	}

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
				{
					NodeResources: &troubleshootv1beta2.NodeResources{
						AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
							CheckName: "Node Count Check",
						},
						Outcomes: []*troubleshootv1beta2.Outcome{
							{
								Fail: &troubleshootv1beta2.SingleOutcome{
									When:    "count() < 3",
									Message: "The cluster needs a minimum of 3 nodes.",
								},
							},
							{
								Pass: &troubleshootv1beta2.SingleOutcome{
									Message: "There are not enough nodes to run this application (3 or more)",
								},
							},
						},
					},
				},
				{
					ClusterVersion: &troubleshootv1beta2.ClusterVersion{
						Outcomes: []*troubleshootv1beta2.Outcome{
							{
								Fail: &troubleshootv1beta2.SingleOutcome{
									When:    "< 1.16.0",
									Message: "The application requires at least Kubernetes 1.16.0, and recommends 1.18.0.",
									URI:     "https://kubernetes.io",
								},
							},
							{
								Warn: &troubleshootv1beta2.SingleOutcome{
									When:    "< 1.18.0",
									Message: "Your cluster meets the minimum version of Kubernetes, but we recommend you update to 1.18.0 or later.",
									URI:     "https://kubernetes.io",
								},
							},
							{
								Pass: &troubleshootv1beta2.SingleOutcome{
									Message: "Your cluster meets the recommended and required versions of Kubernetes.",
								},
							},
						},
					},
				},
			},
		},
	}

	tests := PreflightSpecsReadTests{
		// TODOLATER: URL support? local mock webserver? would prefer for these tests to not require internet :)
		// TODOLATER: multidoc fixtures
		PreflightSpecsReadTest{
			name:                  "file-preflight",
			args:                  []string{preflightFile},
			customStdin:           false,
			wantErr:               false,
			wantPreflightSpec:     &expectPreflightSpec,
			wantHostPreflightSpec: nil,
			wantUploadResultSpecs: nil,
		},
		PreflightSpecsReadTest{
			name:                  "file-hostpreflight",
			args:                  []string{hostpreflightFile},
			customStdin:           false,
			wantErr:               false,
			wantPreflightSpec:     nil,
			wantHostPreflightSpec: &expectHostPreflightSpec,
			wantUploadResultSpecs: nil,
		},
		PreflightSpecsReadTest{
			name:                  "file-secret",
			args:                  []string{preflightSecretFile},
			customStdin:           false,
			wantErr:               false,
			wantPreflightSpec:     &expectSecretPreflightSpec,
			wantHostPreflightSpec: nil,
			wantUploadResultSpecs: nil,
		},
		PreflightSpecsReadTest{
			name:                  "stdin-preflight",
			args:                  []string{"-"},
			customStdin:           true,
			stdinDataFile:         preflightFile,
			wantErr:               false,
			wantPreflightSpec:     &expectPreflightSpec,
			wantHostPreflightSpec: nil,
			wantUploadResultSpecs: nil,
		},
		PreflightSpecsReadTest{
			name:                  "stdin-secret",
			args:                  []string{"-"},
			customStdin:           true,
			stdinDataFile:         preflightSecretFile,
			wantErr:               false,
			wantPreflightSpec:     &expectSecretPreflightSpec,
			wantHostPreflightSpec: nil,
			wantUploadResultSpecs: nil,
		},
		PreflightSpecsReadTest{
			name:                  "stdin-secret and file-preflight",
			args:                  []string{"-", preflightFile},
			customStdin:           true,
			stdinDataFile:         preflightSecretFile,
			wantErr:               false,
			wantPreflightSpec:     concatSpecs(expectSecretPreflightSpec, expectPreflightSpec),
			wantHostPreflightSpec: nil,
			wantUploadResultSpecs: nil,
		},
		PreflightSpecsReadTest{
			name: "stdin-secret and support-bundle secret",
			args: []string{
				"-",
				filepath.Join(testutils.FileDir(), "../../testdata/supportbundle/labelled-specs/sb-spec-1.yaml"),
			},
			customStdin:           true,
			stdinDataFile:         preflightSecretFile,
			wantErr:               false,
			wantPreflightSpec:     &expectSecretPreflightSpec,
			wantHostPreflightSpec: nil,
			wantUploadResultSpecs: nil,
		},
		PreflightSpecsReadTest{
			name: "stdin-secret and redact secret",
			args: []string{
				"-",
				filepath.Join(testutils.FileDir(), "../../testdata/supportbundle/labelled-specs/redact-spec-1.yaml"),
			},
			customStdin:           true,
			stdinDataFile:         preflightSecretFile,
			wantErr:               false,
			wantPreflightSpec:     &expectSecretPreflightSpec,
			wantHostPreflightSpec: nil,
			wantUploadResultSpecs: nil,
		},
		/*
			/* TODOLATER: needs a cluster with a spec installed?
			PreflightSpecsReadTest{
				name:     "cluster-secret",
				args:     []string{"/secret/some-secret-spec"},
				customStdin: false,
				wantErr:  false,
				wantPreflightSpec:     &expectSecretPreflightSpec,
				wantHostPreflightSpec: &expectSecretHostPreflightSpec,
				wantUploadResultsSpecs: expectSecretUploadResultSpecs,
			},
		*/
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt := tt // pin

			specs, tErr := singleTestPreflightSpecsRead(t, &tt)
			require.Equal(t, tt.wantErr, tErr != nil)

			if tt.wantPreflightSpec != nil {
				assert.Truef(t,
					contains(specs.PreflightsV1Beta2, *tt.wantPreflightSpec),
					"expected %v to contain %v", specs.PreflightsV1Beta2, *tt.wantPreflightSpec,
				)
			}
			for _, wantUploadResultSpec := range tt.wantUploadResultSpecs {
				assert.Truef(t,
					contains(specs.PreflightsV1Beta2, wantUploadResultSpec),
					"expected %v to contain %v", specs.PreflightsV1Beta2, wantUploadResultSpec,
				)
			}
			if tt.wantHostPreflightSpec != nil {
				assert.Truef(t,
					contains(specs.HostPreflightsV1Beta2, *tt.wantHostPreflightSpec),
					"expected %v to contain %v", testutils.AsJSON(t, specs.HostPreflightsV1Beta2), testutils.AsJSON(t, specs.HostPreflightsV1Beta2),
				)
			}

			assert.Len(t, specs.SupportBundlesV1Beta2, 0)
			assert.Len(t, specs.RedactorsV1Beta2, 0)
			assert.Len(t, specs.AnalyzersV1Beta2, 0)
			assert.Len(t, specs.CollectorsV1Beta2, 0)
			assert.Len(t, specs.RemoteCollectorsV1Beta2, 0)
			assert.Len(t, specs.HostCollectorsV1Beta2, 0)
		})
	}
}

func contains[T any](list []T, obj T) bool {
	for _, item := range list {
		if assert.ObjectsAreEqual(item, obj) {
			return true
		}
	}
	return false
}

func concatSpecs(target troubleshootv1beta2.Preflight, source troubleshootv1beta2.Preflight) *troubleshootv1beta2.Preflight {
	newSpec := target.DeepCopy()
	newSpec.Spec.Collectors = append(newSpec.Spec.Collectors, source.Spec.Collectors...)
	newSpec.Spec.RemoteCollectors = append(newSpec.Spec.RemoteCollectors, source.Spec.RemoteCollectors...)
	newSpec.Spec.Analyzers = append(newSpec.Spec.Analyzers, source.Spec.Analyzers...)

	return newSpec
}

// Structured as a separate function so we can use defer appropriately
func singleTestPreflightSpecsRead(t *testing.T, tt *PreflightSpecsReadTest) (*loader.TroubleshootKinds, error) {
	var tmpfile *os.File
	var err error
	if tt.customStdin == true {
		tmpfile, err = os.CreateTemp("", "singleTestPreflightSpecsRead")
		if err != nil {
			t.Fatal(err)
		}

		defer os.Remove(tmpfile.Name()) // clean up

		// Read in data file
		// TODOLATER: just copy the file...?
		stdinData, err := os.ReadFile(tt.stdinDataFile)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := tmpfile.Write(stdinData); err != nil {
			t.Fatal(err)
		}

		if _, err := tmpfile.Seek(0, 0); err != nil {
			t.Fatal(err)
		}

		oldStdin := os.Stdin
		defer func() { os.Stdin = oldStdin }() // Restore original Stdin

		os.Stdin = tmpfile
	}

	kinds, err := readSpecs(tt.args)

	if tt.customStdin == true {
		if err = tmpfile.Close(); err != nil {
			t.Fatal(err)
		}
	}

	return kinds, err
}

func TestPreprocessV1Beta3Specs_RequiresValues(t *testing.T) {
	// Save and restore viper state
	oldValues := viper.Get("values")
	oldSet := viper.Get("set")
	defer func() {
		viper.Set("values", oldValues)
		viper.Set("set", oldSet)
	}()

	t.Run("v1beta3 without values should error", func(t *testing.T) {
		// Clear viper values
		viper.Set("values", []string{})
		viper.Set("set", []string{})

		v1beta3File := filepath.Join(testutils.FileDir(), "../../examples/preflight/simple-v1beta3.yaml")
		_, _, err := preprocessV1Beta3Specs([]string{v1beta3File})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "v1beta3 specs with Helm templates require a values file")
	})

	t.Run("v1beta3 with values file should succeed", func(t *testing.T) {
		valuesFile := filepath.Join(testutils.FileDir(), "../../examples/preflight/values-v1beta3-1.yaml")
		viper.Set("values", []string{valuesFile})
		viper.Set("set", []string{})

		v1beta3File := filepath.Join(testutils.FileDir(), "../../examples/preflight/simple-v1beta3.yaml")
		processedArgs, tempFiles, err := preprocessV1Beta3Specs([]string{v1beta3File})

		// Clean up temp files
		defer func() {
			for _, f := range tempFiles {
				_ = os.Remove(f)
			}
		}()

		require.NoError(t, err)
		assert.NotNil(t, processedArgs)
	})

	t.Run("v1beta3 with --set values should succeed", func(t *testing.T) {
		viper.Set("values", []string{})
		viper.Set("set", []string{"kubernetes.enabled=true"})

		v1beta3File := filepath.Join(testutils.FileDir(), "../../examples/preflight/simple-v1beta3.yaml")
		processedArgs, tempFiles, err := preprocessV1Beta3Specs([]string{v1beta3File})

		// Clean up temp files
		defer func() {
			for _, f := range tempFiles {
				_ = os.Remove(f)
			}
		}()

		require.NoError(t, err)
		assert.NotNil(t, processedArgs)
	})

	t.Run("v1beta2 without values should succeed", func(t *testing.T) {
		viper.Set("values", []string{})
		viper.Set("set", []string{})

		v1beta2File := filepath.Join(testutils.FileDir(), "../../testdata/preflightspec/troubleshoot_v1beta2_preflight_gotest.yaml")
		processedArgs, tempFiles, err := preprocessV1Beta3Specs([]string{v1beta2File})

		require.NoError(t, err)
		assert.NotNil(t, processedArgs)
		assert.Empty(t, tempFiles)
		assert.Equal(t, []string{v1beta2File}, processedArgs)
	})
}
