package analyzer

import (
	"encoding/json"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnmarshallingCheckAllResults(t *testing.T) {
	s := testutils.GetTestFixture(t, "goldpinger/checkall-with-error.json")
	var res checkAllOutput
	err := json.Unmarshal([]byte(s), &res)
	require.NoError(t, err)
	assert.Len(t, res.Hosts, 3)
	assert.Equal(t, "goldpinger-tbdsb", res.Hosts[1].PodName)
	assert.Equal(t, "10.32.2.2", res.Responses["goldpinger-4hctt"].Response.PodResults["goldpinger-jj9mw"].PodIP)
	assert.Equal(t,
		`Get "http://10.32.0.9:80/ping": context deadline exceeded`,
		res.Responses["goldpinger-tbdsb"].Response.PodResults["goldpinger-4hctt"].Error,
	)
}

func TestAnalyzeGoldpinger_podPingsAnalysis(t *testing.T) {
	tests := []struct {
		name string
		cao  *checkAllOutput
		want []*AnalyzeResult
	}{
		{
			name: "no ping errors",
			cao:  caoFixture(t, "goldpinger/checkall-success.json"),
			want: []*AnalyzeResult{
				{
					Title:   "Pings to \"goldpinger-kpz4g\" pod succeeded",
					Message: "Pings to \"goldpinger-kpz4g\" pod from all other pods in the cluster succeeded",
					IconKey: "kubernetes",
					IsPass:  true,
				},
				{
					Title:   "Pings to \"goldpinger-k6d2j\" pod succeeded",
					Message: "Pings to \"goldpinger-k6d2j\" pod from all other pods in the cluster succeeded",
					IconKey: "kubernetes",
					IsPass:  true,
				},
				{
					Title:   "Pings to \"goldpinger-5ck4d\" pod succeeded",
					Message: "Pings to \"goldpinger-5ck4d\" pod from all other pods in the cluster succeeded",
					IconKey: "kubernetes",
					IsPass:  true,
				},
			},
		},
		{
			name: "with some ping errors",
			cao:  caoFixture(t, "goldpinger/checkall-with-error.json"),
			want: []*AnalyzeResult{
				{
					Title:   "Ping from \"goldpinger-jj9mw\" pod to \"goldpinger-4hctt\" pod failed",
					Message: "Ping error: Get \"http://10.32.0.9:80/ping\": context deadline exceeded",
					IconKey: "kubernetes",
					IsFail:  true,
				},
				{
					Title:   "Ping from \"goldpinger-jj9mw\" pod to \"goldpinger-tbdsb\" pod failed",
					Message: "Ping error: Get \"http://10.32.1.2:80/ping\": context deadline exceeded",
					IconKey: "kubernetes",
					IsFail:  true,
				},
				{
					Title:   "Ping from \"goldpinger-4hctt\" pod to \"goldpinger-jj9mw\" pod failed",
					Message: "Ping error: Get \"http://10.32.2.2:80/ping\": context deadline exceeded",
					IconKey: "kubernetes",
					IsFail:  true,
				},
				{
					Title:   "Ping from \"goldpinger-4hctt\" pod to \"goldpinger-tbdsb\" pod failed",
					Message: "Ping error: Get \"http://10.32.1.2:80/ping\": context deadline exceeded",
					IconKey: "kubernetes",
					IsFail:  true,
				},
				{
					Title:   "Ping from \"goldpinger-tbdsb\" pod to \"goldpinger-4hctt\" pod failed",
					Message: "Ping error: Get \"http://10.32.0.9:80/ping\": context deadline exceeded",
					IconKey: "kubernetes",
					IsFail:  true,
				},
				{
					Title:   "Ping from \"goldpinger-tbdsb\" pod to \"goldpinger-jj9mw\" pod failed",
					Message: "Ping error: Get \"http://10.32.2.2:80/ping\": context deadline exceeded",
					IconKey: "kubernetes",
					IsFail:  true,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &AnalyzeGoldpinger{
				analyzer: &troubleshootv1beta2.GoldpingerAnalyze{},
			}

			got := a.podPingsAnalysis(tt.cao)
			// Check existence of each want. Maps are not ordered, so we can't just compare
			for _, want := range tt.want {
				assert.Contains(t, got, want)
			}
			assert.Len(t, got, len(tt.want))
		})
	}
}

func caoFixture(t *testing.T, path string) *checkAllOutput {
	t.Helper()

	s := testutils.GetTestFixture(t, path)
	var res checkAllOutput
	err := json.Unmarshal([]byte(s), &res)
	require.NoError(t, err)

	return &res
}
