package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_doCompareHostCPU(t *testing.T) {
	tests := []struct {
		name     string
		operator string
		desired  string
		actual   int
		expected bool
	}{
		{
			name:     "< 16",
			operator: "<",
			desired:  "16",
			actual:   8,
			expected: true,
		},
		{
			name:     "< 8 when actual is 8",
			operator: "<",
			desired:  "8",
			actual:   8,
			expected: false,
		},
		{
			name:     "<= 8 when actual is 8",
			operator: "<=",
			desired:  "8",
			actual:   8,
			expected: true,
		},
		{
			name:     "<= 8 when actual is 16",
			operator: "<=",
			desired:  "8",
			actual:   16,
			expected: false,
		},
		{
			name:     "== 8 when actual is 16",
			operator: "==",
			desired:  "8",
			actual:   16,
			expected: false,
		},
		{
			name:     "== 8 when actual is 8",
			operator: "==",
			desired:  "8",
			actual:   8,
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := doCompareHostCPU(test.operator, test.desired, test.actual)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)

		})
	}
}

func Test_compareHostCPUConditionalToActual(t *testing.T) {
	tests := []struct {
		name          string
		when          string
		logicalCount  int
		physicalCount int
		flags         []string
		machineArch   string
		expected      bool
	}{
		{
			name:          "physical > 4, when physical is 8",
			when:          "physical > 4",
			logicalCount:  0,
			physicalCount: 8,
			expected:      true,
		},
		{
			name:          "physical > 4, when physical is 4",
			when:          "physical > 4",
			logicalCount:  0,
			physicalCount: 4,
			expected:      false,
		},
		{
			name:          "physical > 4, when physical is 3, logical is 6",
			when:          "physical > 4",
			logicalCount:  6,
			physicalCount: 3,
			expected:      false,
		},
		{
			name:          "logical > 4, when physical is 4, logical is 8",
			when:          "logical > 4",
			logicalCount:  8,
			physicalCount: 4,
			expected:      true,
		},
		{
			name:          ">= 4, when physical is 2, logical is 4",
			when:          ">= 4",
			logicalCount:  4,
			physicalCount: 2,
			expected:      true,
		},
		{
			name:          "count < 4, when physical is 2, logical is 4",
			when:          "count < 4",
			logicalCount:  4,
			physicalCount: 2,
			expected:      false,
		},
		{
			name:          "count <= 4, when physical is 2, logical is 4",
			when:          "count <= 4",
			logicalCount:  4,
			physicalCount: 2,
			expected:      true,
		},
		{
			name:          "== 4, physical is 4, logical is 4",
			when:          "== 4",
			logicalCount:  4,
			physicalCount: 4,
			expected:      true,
		},
		{
			name:     "supports x86-64-v2 microarchitecture",
			when:     "supports x86-64-v2",
			flags:    []string{""},
			expected: false,
		},
		{
			name:     "supports x86-64-v2 microarchitecture",
			when:     "supports x86-64-v2",
			flags:    []string{"cmov", "cx8", "fpu", "fxsr", "mmx", "syscall", "sse", "sse2", "cx16", "lahf_lm", "popcnt", "ssse3", "sse4_1", "sse4_2", "ssse3"},
			expected: true,
		},
		{
			name:     "has a non existent flag",
			when:     "hasFlags cmov,doesNotExist",
			flags:    []string{"cmov", "cx8", "fpu", "fxsr", "mmx", "syscall", "sse", "sse2", "cx16", "lahf_lm", "popcnt", "ssse3", "sse4_1", "sse4_2", "ssse3"},
			expected: false,
		},
		{
			name:     "has flags",
			when:     "hasFlags a,b,c",
			flags:    []string{"a", "b", "c", "d", "e"},
			expected: true,
		},
		{
			name:     "machine arch matches",
			when:     "machineArch == x86_64",
			machineArch: "x86_64",
			expected: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			actual, err := compareHostCPUConditionalToActual(test.when, test.logicalCount, test.physicalCount, test.flags, test.machineArch)
			req.NoError(err)

			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestHostCpuAnalyze(t *testing.T) {
	tt := []struct {
		name     string
		cpuInfo  collect.CPUInfo
		outcomes []*troubleshootv1beta2.Outcome
		results  []*AnalyzeResult
		wantErr  bool
	}{
		{
			name: "fix for passing test with empty when expr",
			cpuInfo: collect.CPUInfo{
				LogicalCount:  16,
				PhysicalCount: 8,
			},
			outcomes: []*troubleshootv1beta2.Outcome{
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "logical < 8",
						Message: "oops",
					},
				},
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						When:    "",
						Message: "it passed",
					},
				},
			},
			results: []*AnalyzeResult{
				{
					IsPass:  true,
					Message: "it passed",
					Title:   "Number of CPUs",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fn := func(_ string) ([]byte, error) {
				return json.Marshal(&tc.cpuInfo)
			}

			analyzer := AnalyzeHostCPU{
				hostAnalyzer: &troubleshootv1beta2.CPUAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						CheckName: "Number of CPUs",
					},
					Outcomes: tc.outcomes,
				},
			}
			results, err := analyzer.Analyze(fn, nil)
			if tc.wantErr {
				require.NotNil(t, err)
				return
			}
			require.Nil(t, err)
			require.Equal(t, tc.results, results)
		})
	}
}
