package docrewrite

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestConvertToV1Beta2(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		isError bool
	}{
		{
			name: "rewrite v1beta1",
			input: `kind: Collector
apiVersion: troubleshoot.replicated.com/v1beta1
metadata:
  name: collector-sample
spec:
  collectors:
  - clusterInfo: {}
`,
			want: `kind: Collector
apiVersion: troubleshoot.sh/v1beta2
metadata:
  name: collector-sample
spec:
  collectors:
  - clusterInfo: {}`,
			isError: false,
		},
		{
			name: "do not rewrite v1beta2",
			input: `kind: Collector
apiVersion: troubleshoot.sh/v1beta2
metadata:
  name: collector-sample
spec:
  collectors:
  - clusterInfo: {}
`,
			want: `kind: Collector
apiVersion: troubleshoot.sh/v1beta2
metadata:
  name: collector-sample
spec:
  collectors:
  - clusterInfo: {}`,
			isError: false,
		},
		{
			name: "fail rewrite",
			input: `kind: Collector
apiVersion: not.troubleshoot.replicated.com/v1beta1
metadata:
  name: collector-sample
spec:
  collectors:
  - clusterInfo: {}
`,
			want:    ``,
			isError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)

			got, err := ConvertToV1Beta2([]byte(tt.input))
			if tt.isError {
				req.Error(err)
				return
			}

			var wantParsed, gotParsed interface{}

			_ = yaml.Unmarshal([]byte(tt.want), &wantParsed)
			_ = yaml.Unmarshal(got, &gotParsed)

			req.NoError(err)
			req.Equal(wantParsed, gotParsed)
		})
	}
}
