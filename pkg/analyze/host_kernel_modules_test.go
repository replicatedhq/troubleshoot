package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeKernelModules(t *testing.T) {
	tests := []struct {
		name         string
		info         map[string]collect.KernelModuleInfo
		hostAnalyzer *troubleshootv1beta2.KernelModulesAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name: "module 'abc' is not loaded",
			info: map[string]collect.KernelModuleInfo{},
			hostAnalyzer: &troubleshootv1beta2.KernelModulesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "abc != loaded",
							Message: "the module 'abc' is not loaded",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Kernel Modules",
					IsFail:  true,
					Message: "the module 'abc' is not loaded",
				},
			},
		},
		{
			name: "module 'abc' is loaded",
			info: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "loaded",
				},
			},
			hostAnalyzer: &troubleshootv1beta2.KernelModulesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "abc != loaded",
							Message: "the module 'abc' is not loaded",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "abc == loaded",
							Message: "the module 'abc' is loaded",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Kernel Modules",
					IsPass:  true,
					Message: "the module 'abc' is loaded",
				},
			},
		},
		{
			name: "multiple results",
			info: map[string]collect.KernelModuleInfo{
				"xyz": {
					Status: "loaded",
				},
			},
			hostAnalyzer: &troubleshootv1beta2.KernelModulesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "abc != loaded,unloaded",
							Message: "the module 'abc' is not loaded or loadable",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "def != loaded,unloaded",
							Message: "the module 'def' is not loaded or loadable",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "ghi != loaded",
							Message: "the module 'def' is not loaded",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "xyz == loaded",
							Message: "the module 'xyz' is loaded",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Kernel Modules",
					IsFail:  true,
					Message: "the module 'abc' is not loaded or loadable",
				},
				{
					Title:   "Kernel Modules",
					IsFail:  true,
					Message: "the module 'def' is not loaded or loadable",
				},
				{
					Title:   "Kernel Modules",
					IsWarn:  true,
					Message: "the module 'def' is not loaded",
				},
				{
					Title:   "Kernel Modules",
					IsPass:  true,
					Message: "the module 'xyz' is loaded",
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			b, err := json.Marshal(test.info)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := (&AnalyzeHostKernelModules{test.hostAnalyzer}).Analyze(getCollectedFileContents)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}

func Test_compareKernelModuleConditionalToActual(t *testing.T) {
	tests := []struct {
		name        string
		conditional string
		modules     map[string]collect.KernelModuleInfo
		wantRes     bool
		wantErr     bool
	}{
		{
			name:        "match second item",
			conditional: "def = loaded",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "unloading",
				},
				"def": {
					Status: "loaded",
				},
			},
			wantRes: true,
		},
		{
			name:        "match multiple items",
			conditional: "abc,def = loaded,loadable",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "loadable",
				},
				"def": {
					Status: "loaded",
				},
			},
			wantRes: true,
		},
		{
			name:        "match multiple items, one not ok",
			conditional: "abc,def,ghi = loaded,loadable",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "loadable",
				},
				"def": {
					Status: "unloading",
				},
				"ghi": {
					Status: "loaded",
				},
			},
			wantRes: false,
		},
		{
			name:        "item not in list",
			conditional: "abc = loaded",
			modules: map[string]collect.KernelModuleInfo{
				"def": {
					Status: "unloading",
				},
			},
			wantRes: false,
		},
		{
			name:        "item does not match",
			conditional: "abc = loaded",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "unloading",
				},
			},
			wantRes: false,
		},
		{
			name:        "other operator",
			conditional: "abc * loaded",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "unloading",
				},
			},
			wantErr: true,
		},
		{
			name:        "item matches one of multiple",
			conditional: "abc = loaded,loadable",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "loadable",
				},
			},
			wantRes: true,
		},
		{
			name:        "item matches with !=",
			conditional: "abc != unloading",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "loadable",
				},
			},
			wantRes: true,
		},
		{
			name:        "item not found matches with !=",
			conditional: "abc != unloading",
			modules:     map[string]collect.KernelModuleInfo{},
			wantRes:     true,
		},
		{
			name:        "item does not match with !=",
			conditional: "abc != loadable",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "loadable",
				},
			},
			wantRes: false,
		},
		{
			name:        "item matches one of multiple with !=",
			conditional: "abc != loaded,loadable",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "unloading",
				},
			},
			wantRes: true,
		},
		{
			name:        "item does not match one of multiple with !=",
			conditional: "abc != loaded,loadable",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "loadable",
				},
			},
			wantRes: false,
		},
		{
			name:        "match multiple items with !=",
			conditional: "abc,def != loading,unloading",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "loadable",
				},
				"def": {
					Status: "loaded",
				},
			},
			wantRes: true,
		},
		{
			name:        "match multiple items with !=, one not ok",
			conditional: "abc,def,ghi != loading,unloading",
			modules: map[string]collect.KernelModuleInfo{
				"abc": {
					Status: "loadable",
				},
				"def": {
					Status: "unloading",
				},
				"ghi": {
					Status: "loaded",
				},
			},
			wantRes: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			gotRes, err := compareKernelModuleConditionalToActual(tt.conditional, tt.modules)
			if tt.wantErr {
				req.Error(err)
			} else {
				req.NoError(err)
				req.Equal(tt.wantRes, gotRes)
			}
		})
	}
}
