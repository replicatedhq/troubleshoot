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
	}

	tests := []struct {
		name      string
		kConfigs  collect.KConfigs
		outcomes  []*troubleshootv1beta2.Outcome
		results   []*AnalyzeResult
		expectErr bool
	}{
		{
			name:     "all pass",
			kConfigs: kConfigs,
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						When:    "CONFIG_CGROUP_FREEZER=y",
						Message: "Freezer cgroup subsystem built-in",
					},
				},
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						When:    "CONFIG_NETFILTER_XTABLES=m",
						Message: "Netfilter Xtables support module",
					},
				},
			},
			results: []*AnalyzeResult{
				{
					Title:   "Kernel Configs",
					IsPass:  true,
					Message: "Freezer cgroup subsystem built-in",
				}, {
					Title:   "Kernel Configs",
					IsPass:  true,
					Message: "Netfilter Xtables support module",
				},
			},
			expectErr: false,
		},
		{
			name:     "has fail",
			kConfigs: kConfigs,
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "CONFIG_NETFILTER_XTABLES=m",
						Message: "Netfilter Xtables support module",
					},
				},
			},
			results: []*AnalyzeResult{
				{
					Title:   "Kernel Configs",
					IsFail:  true,
					Message: "Netfilter Xtables support module",
				},
			},
			expectErr: false,
		},
		{
			name:     "has warn",
			kConfigs: kConfigs,
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Warn: &troubleshootv1beta2.SingleOutcome{
						When:    "CONFIG_NETFILTER_XTABLES=m",
						Message: "Netfilter Xtables support module",
					},
				},
			},
			results: []*AnalyzeResult{
				{
					Title:   "Kernel Configs",
					IsWarn:  true,
					Message: "Netfilter Xtables support module",
				},
			},
			expectErr: false,
		},
		{
			name:     "missing kernel config",
			kConfigs: kConfigs,
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						When:    "CONFIG_NF_NAT_IPV4=y",
						Message: "IPv4 NAT option",
					},
				},
			},
			results: []*AnalyzeResult{
				{
					Title:   "Kernel Configs",
					IsPass:  false,
					Message: "IPv4 NAT option",
				},
			},
			expectErr: false,
		},
		{
			name:     "kernel config disabled",
			kConfigs: kConfigs,
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						When:    "CONFIG_CGROUP_FREEZER=n",
						Message: "CONFIG_CGROUP_FREEZER is disabled",
					},
				},
			},
			results: []*AnalyzeResult{
				{
					Title:   "Kernel Configs",
					IsPass:  false,
					Message: "CONFIG_CGROUP_FREEZER is disabled",
				},
			},
			expectErr: false,
		},
		{
			name: "missing when attribute",
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						Message: "CONFIG_foo is enabled",
						When:    "",
					},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			fn := func(_ string) ([]byte, error) {
				return []byte(`{"CONFIG_CGROUP_FREEZER": "y", "CONFIG_NETFILTER_XTABLES": "m"}`), nil
			}

			analyzer := AnalyzeHostKernelConfigs{
				hostAnalyzer: &troubleshootv1beta2.KernelConfigsAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						CheckName: "Kernel Configs",
					},
					Outcomes: tt.outcomes,
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
