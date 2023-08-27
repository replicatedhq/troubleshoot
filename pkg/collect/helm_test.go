package collect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_HelmReleaseInfoByNamespaces(t *testing.T) {
	tests := []struct {
		name        string
		releaseInfo []ReleaseInfo
		want        map[string][]ReleaseInfo
	}{
		{
			name:        "Test case 1: Empty slice",
			releaseInfo: []ReleaseInfo{},
			want:        map[string][]ReleaseInfo{},
		},
		{
			name: "Test case 2: Single namespace",
			releaseInfo: []ReleaseInfo{
				{ReleaseName: "release1", Namespace: "default"},
				{ReleaseName: "release2", Namespace: "default"},
			},
			want: map[string][]ReleaseInfo{
				"default": {
					{ReleaseName: "release1", Namespace: "default"},
					{ReleaseName: "release2", Namespace: "default"},
				},
			},
		},
		{
			name: "Test case 3: Multiple namespaces",
			releaseInfo: []ReleaseInfo{
				{ReleaseName: "release1", Namespace: "default"},
				{ReleaseName: "release2", Namespace: "kube-system"},
				{ReleaseName: "release3", Namespace: "default"},
			},
			want: map[string][]ReleaseInfo{
				"default": {
					{ReleaseName: "release1", Namespace: "default"},
					{ReleaseName: "release3", Namespace: "default"},
				},
				"kube-system": {
					{ReleaseName: "release2", Namespace: "kube-system"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := helmReleaseInfoByNamespaces(tt.releaseInfo)
			assert.Equal(t, tt.want, got)
		})
	}
}
