package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeKernelConfigs(t *testing.T) {
	kConfigs := collect.KConfigs{
		"CONFIG_CGROUP_FREEZER":    "y",
		"CONFIG_NETFILTER_XTABLES": "m",
		"CONFIG_BRIDGE":            "y",
	}

	tests := []struct {
		name            string
		kConfigs        collect.KConfigs
		selectedConfigs []string
		outcomes        []*troubleshootv1beta2.Outcome
		results         []*AnalyzeResult
		expectErr       bool
	}{
		{
			name:            "all pass",
			kConfigs:        kConfigs,
			selectedConfigs: []string{"CONFIG_CGROUP_FREEZER=y", "CONFIG_NETFILTER_XTABLES=m"},
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						Message: "required kernel configs are available",
					},
				},
			},
			results: []*AnalyzeResult{
				{
					Title:   "Kernel Configs",
					IsPass:  true,
					Message: "required kernel configs are available",
				},
			},
			expectErr: false,
		},
		{
			name:            "has fail",
			kConfigs:        kConfigs,
			selectedConfigs: []string{"CONFIG_UTS_NS=y"},
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						Message: "missing kernel config(s): {{ .ConfigsNotFound }}",
					},
				},
			},
			results: []*AnalyzeResult{
				{
					Title:   "Kernel Configs",
					IsFail:  true,
					Message: "missing kernel config(s): CONFIG_UTS_NS=y",
				},
			},
			expectErr: false,
		},
		{
			name:            "kernel config disabled",
			kConfigs:        kConfigs,
			selectedConfigs: []string{"CONFIG_CGROUP_FREEZER=n"},
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						Message: "missing kernel config(s): {{ .ConfigsNotFound }}",
					},
				},
			},
			results: []*AnalyzeResult{
				{
					Title:   "Kernel Configs",
					IsFail:  true,
					Message: "missing kernel config(s): CONFIG_CGROUP_FREEZER=n",
				},
			},
			expectErr: false,
		},
		{
			name:            "invalid kernel config",
			kConfigs:        kConfigs,
			selectedConfigs: []string{"foobar=n"},
			expectErr:       true,
		},
		{
			name:            "select multiple kernel config values",
			kConfigs:        kConfigs,
			selectedConfigs: []string{"CONFIG_BRIDGE=my"},
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						Message: "required kernel configs are available",
					},
				},
			},
			results: []*AnalyzeResult{
				{
					Title:   "Kernel Configs",
					IsPass:  true,
					Message: "required kernel configs are available",
				},
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			fn := func(_ string) ([]byte, error) {
				return []byte(`{"CONFIG_CGROUP_FREEZER": "y", "CONFIG_NETFILTER_XTABLES": "m", "CONFIG_BRIDGE": "y"}`), nil
			}

			analyzer := AnalyzeHostKernelConfigs{
				hostAnalyzer: &troubleshootv1beta2.KernelConfigsAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						CheckName: "Kernel Configs",
					},
					SelectedConfigs: tt.selectedConfigs,
					Outcomes:        tt.outcomes,
				},
			}

			results, err := analyzer.Analyze(fn, nil)

			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.results, results)
			}
		})
	}
}
