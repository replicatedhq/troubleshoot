package collect

import (
	"path/filepath"
	"testing"

	"github.com/replicatedhq/troubleshoot/internal/testutils"
	"github.com/stretchr/testify/assert"
)

func TestCollectorResult_AddResult(t *testing.T) {
	r := CollectorResult{"a": []byte("a")}

	other := CollectorResult{"b": []byte("b")}
	r.AddResult(other)

	assert.Equal(t, 2, len(r))
	assert.Equal(t, []byte("a"), r["a"])
	assert.Equal(t, []byte("b"), r["b"])
}

func TestCollectorResultFromBundle(t *testing.T) {
	tests := []struct {
		name      string
		bundleDir string
		want      CollectorResult
		wantErr   bool
	}{
		{
			name:      "creates collector results from a bundle successfully",
			bundleDir: filepath.Join(testutils.FileDir(), "../../testdata/supportbundle/extracted-sb"),
			want: CollectorResult{
				"cluster-resources/pods/logs/default/static-hi/static-hi.log": nil,
				"static-hi.log": nil,
				"version.yaml":  nil,
			},
			wantErr: false,
		},
		{
			name:      "fails to create collector results from a directory that is not a bundle",
			bundleDir: filepath.Join(testutils.FileDir(), "../../testdata/supportbundle"),
			want:      nil,
			wantErr:   true,
		},
		{
			name:      "fails to create collector results from a missing directory",
			bundleDir: "gibberish",
			want:      nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CollectorResultFromBundle(tt.bundleDir)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, (err != nil), tt.wantErr)
		})
	}
}
