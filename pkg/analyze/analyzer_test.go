package analyzer

import (
	"context"
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
