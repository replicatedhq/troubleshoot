package oci

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_toRegistryRef(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		want    string
		wantErr bool
	}{
		{
			name: "valid uri",
			uri:  "oci://localhost/replicated-preflight",
			want: "localhost/replicated-preflight:latest",
		},
		{
			name: "valid uri with port",
			uri:  "oci://localhost:5000/replicated-preflight",
			want: "localhost:5000/replicated-preflight:latest",
		},
		{
			name: "valid uri with tag",
			uri:  "oci://localhost:5000/replicated-preflight:v4",
			want: "localhost:5000/replicated-preflight:v4",
		},
		{
			name:    "invalid uri - missing scheme",
			uri:     "localhost:5000/replicated-preflight:v4",
			wantErr: true,
		},
		{
			name:    "invalid uri - wrong scheme",
			uri:     "https://localhost:5000/replicated-preflight:v4",
			wantErr: true,
		},
		{
			name:    "empty uri",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toRegistryRef(tt.uri)
			require.Equalf(t, (err != nil), tt.wantErr, "toRegistryRef() error = %v, wantErr %v", err, tt.wantErr)

			gotStr := got.String()
			assert.Equalf(t, tt.want, gotStr, "toRegistryRef() = %v, want %v", gotStr, tt.want)
		})
	}
}
