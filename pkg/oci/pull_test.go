package oci

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_parseURI(t *testing.T) {
	tests := []struct {
		name      string
		uri       string
		imageName string
		wantUri   string
		wantErr   bool
	}{
		{
			name:      "happy path",
			uri:       "oci://registry.replicated.com/app-slug/unstable",
			imageName: "replicated-preflight",
			wantUri:   "registry.replicated.com/app-slug/unstable/replicated-preflight:latest",
		},
		{
			name:      "no path in uri",
			uri:       "oci://registry.replicated.com",
			imageName: "replicated-preflight",
			wantUri:   "registry.replicated.com/replicated-preflight:latest",
		},
		{
			name:      "hostname with port",
			uri:       "oci://localhost:5000/some/path",
			imageName: "replicated-preflight",
			wantUri:   "localhost:5000/some/path/replicated-preflight:latest",
		},
		{
			name:      "uri with tags",
			uri:       "oci://localhost:5000/some/path:tag",
			imageName: "replicated-preflight",
			wantUri:   "localhost:5000/some/path/replicated-preflight:tag",
		},
		{
			name:    "empty uri",
			wantErr: true,
		},
		{
			name:      "invalid uri",
			uri:       "registry.replicated.com/app-slug/unstable",
			imageName: "replicated-preflight",
			wantErr:   true,
		},
		{
			name:      "invalid uri",
			uri:       "https://registry.replicated.com/app-slug/unstable",
			imageName: "replicated-preflight",
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseURI(tt.uri, tt.imageName)
			require.Equalf(t, tt.wantErr, err != nil, "parseURI() error = %v, wantErr %v", err, tt.wantErr)
			assert.Equalf(t, tt.wantUri, got, "parseURI() = %v, want %v", got, tt.wantUri)
		})
	}
}
