package analyzer

import (
	"context"
	"errors"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_GetExcludeFlag(t *testing.T) {
	tests := []struct {
		name     string
		analyzer *troubleshootv1beta2.Analyze
		want     bool
	}{
		{
			name:     "nil case",
			analyzer: nil,
			want:     false,
		},
		{
			name: "true is set",
			analyzer: &troubleshootv1beta2.Analyze{
				TextAnalyze: &troubleshootv1beta2.TextAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						Exclude: multitype.FromBool(true),
					},
				},
			},
			want: true,
		},
		{
			name: "false is set",
			analyzer: &troubleshootv1beta2.Analyze{
				ClusterVersion: &troubleshootv1beta2.ClusterVersion{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						Exclude: multitype.FromBool(false),
					},
				},
			},
			want: false,
		},
		{
			name: "nothing is set",
			analyzer: &troubleshootv1beta2.Analyze{
				Postgres: &troubleshootv1beta2.DatabaseAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{},
				},
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			gotWrapped := GetExcludeFlag(test.analyzer)
			got, err := gotWrapped.Bool()
			req.NoError(err)

			assert.Equal(t, test.want, got)
		})
	}
}

func TestAnalyzeWithNilAnalyzer(t *testing.T) {
	got, err := Analyze(context.Background(), nil, nil, nil)
	assert.Error(t, err)
	assert.Nil(t, got)
}

func TestCollector_DedupCollectors(t *testing.T) {
	tests := []struct {
		name      string
		Analyzers []*troubleshootv1beta2.Analyze
		want      []*troubleshootv1beta2.Analyze
	}{
		{
			name: "multiple ClusterVersion",
			Analyzers: []*troubleshootv1beta2.Analyze{
				{
					ClusterVersion: &troubleshootv1beta2.ClusterVersion{},
				},
				{
					ClusterVersion: &troubleshootv1beta2.ClusterVersion{},
				},
			},
			want: []*troubleshootv1beta2.Analyze{
				{
					ClusterVersion: &troubleshootv1beta2.ClusterVersion{},
				},
			},
		},
		{
			name: "multiple TextAnalyze",
			Analyzers: []*troubleshootv1beta2.Analyze{
				{
					TextAnalyze: &troubleshootv1beta2.TextAnalyze{
						AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
							CheckName: "hi",
						},
					},
				},
				{
					TextAnalyze: &troubleshootv1beta2.TextAnalyze{
						AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
							CheckName: "hi",
						},
					},
				},
			},
			want: []*troubleshootv1beta2.Analyze{
				{
					TextAnalyze: &troubleshootv1beta2.TextAnalyze{
						AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
							CheckName: "hi",
						},
					},
				},
			},
		},
		{
			name: "multiple TextAnalyze with different CheckName",
			Analyzers: []*troubleshootv1beta2.Analyze{
				{
					TextAnalyze: &troubleshootv1beta2.TextAnalyze{
						AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
							CheckName: "hi",
						},
					},
				},
				{
					TextAnalyze: &troubleshootv1beta2.TextAnalyze{
						AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
							CheckName: "test",
						},
					},
				},
			},
			want: []*troubleshootv1beta2.Analyze{
				{
					TextAnalyze: &troubleshootv1beta2.TextAnalyze{
						AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
							CheckName: "hi",
						},
					},
				},
				{
					TextAnalyze: &troubleshootv1beta2.TextAnalyze{
						AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
							CheckName: "test",
						},
					},
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := DedupAnalyzers(tc.Analyzers)
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_GetStrictFlag(t *testing.T) {
	tests := []struct {
		name     string
		analyzer *troubleshootv1beta2.Analyze
		want     bool
	}{
		{
			name:     "nil case",
			analyzer: nil,
			want:     false,
		},
		{
			name: "strict true",
			analyzer: &troubleshootv1beta2.Analyze{
				ClusterVersion: &troubleshootv1beta2.ClusterVersion{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						Strict: multitype.FromBool(true),
					},
				},
			},
			want: true,
		},
		{
			name: "strict false",
			analyzer: &troubleshootv1beta2.Analyze{
				TextAnalyze: &troubleshootv1beta2.TextAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						Strict: multitype.FromBool(false),
					},
				},
			},
			want: false,
		},
		{
			name: "strict unset",
			analyzer: &troubleshootv1beta2.Analyze{
				Postgres: &troubleshootv1beta2.DatabaseAnalyze{},
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetStrictFlag(test.analyzer).BoolOrDefaultFalse()
			assert.Equal(t, test.want, got)
		})
	}
}

func Test_GetHostStrictFlag(t *testing.T) {
	tests := []struct {
		name     string
		analyzer *troubleshootv1beta2.HostAnalyze
		want     bool
	}{
		{
			name:     "nil case",
			analyzer: nil,
			want:     false,
		},
		{
			name: "strict true",
			analyzer: &troubleshootv1beta2.HostAnalyze{
				Memory: &troubleshootv1beta2.MemoryAnalyze{
					AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
						Strict: multitype.FromBool(true),
					},
				},
			},
			want: true,
		},
		{
			name: "strict unset",
			analyzer: &troubleshootv1beta2.HostAnalyze{
				CPU: &troubleshootv1beta2.CPUAnalyze{},
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := GetHostStrictFlag(test.analyzer).BoolOrDefaultFalse()
			assert.Equal(t, test.want, got)
		})
	}
}

// The event analyzer does not copy strict into its results itself, so this
// pins the central stamping in Analyze() for analyzers that never did.
func TestAnalyze_SetsStrictOnResults(t *testing.T) {
	getFile := func(_ string) ([]byte, error) {
		return []byte(`{"kind":"EventList","apiVersion":"v1","items":[]}`), nil
	}

	for _, strict := range []bool{true, false} {
		analyzer := &troubleshootv1beta2.Analyze{
			Event: &troubleshootv1beta2.EventAnalyze{
				AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
					CheckName: "event-check",
					Strict:    multitype.FromBool(strict),
				},
				Reason: "Unhealthy",
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "true",
							Message: "event found",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "false",
							Message: "no event found",
						},
					},
				},
			},
		}

		results, err := Analyze(context.Background(), analyzer, getFile, nil)
		require.NoError(t, err)
		require.NotEmpty(t, results)
		for _, result := range results {
			assert.Equal(t, strict, result.Strict)
		}
	}
}

// The host memory analyzer does not copy strict into its results itself, so
// this pins the central stamping in HostAnalyze(), including the
// analyzer-error path.
func TestHostAnalyze_SetsStrictOnResults(t *testing.T) {
	hostAnalyzer := &troubleshootv1beta2.HostAnalyze{
		Memory: &troubleshootv1beta2.MemoryAnalyze{
			AnalyzeMeta: troubleshootv1beta2.AnalyzeMeta{
				Strict: multitype.FromBool(true),
			},
			Outcomes: []*troubleshootv1beta2.Outcome{
				{
					Fail: &troubleshootv1beta2.SingleOutcome{
						When:    "< 16Gi",
						Message: "not enough memory",
					},
				},
				{
					Pass: &troubleshootv1beta2.SingleOutcome{
						Message: "enough memory",
					},
				},
			},
		},
	}

	getFile := func(_ string) ([]byte, error) {
		return []byte(`{"total": 8589934592}`), nil
	}

	results := HostAnalyze(context.Background(), hostAnalyzer, getFile, nil)
	require.NotEmpty(t, results)
	for _, result := range results {
		assert.True(t, result.IsFail)
		assert.True(t, result.Strict)
	}

	// analyzer-error path: the error result must be strict too
	getFileErr := func(_ string) ([]byte, error) {
		return nil, errors.New("file not collected")
	}

	results = HostAnalyze(context.Background(), hostAnalyzer, getFileErr, nil)
	require.NotEmpty(t, results)
	for _, result := range results {
		assert.True(t, result.IsFail)
		assert.True(t, result.Strict)
	}
}
