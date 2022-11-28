package collect

import (
	"testing"

	v1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_extractServerVersion(t *testing.T) {
	tests := []struct {
		name string
		info string
		want string
	}{
		{
			name: "extracts version successfully",
			info: `
			# Server
			redis_version:7.0.5
			redis_git_sha1:00000000
			redis_git_dirty:0
			redis_build_id:eb3578384289228a
			`,
			want: "7.0.5",
		},
		{
			name: "extracts version but fails",
			info: "",
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractServerVersion(tt.info)
			assert.Equalf(t, tt.want, got, "extractServerVersion() = %v, want %v", got, tt.want)
		})
	}
}

func TestCollectRedis_createPlainTextClient(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		hasError bool
	}{
		{
			name: "valid uri creates redis client successfully",
			uri:  "redis://localhost:6379",
		},
		{
			name:     "empty uri fails to create client with error",
			uri:      "",
			hasError: true,
		},
		{
			name:     "invalid redis protocol fails to create client with error",
			uri:      "http://localhost:6379",
			hasError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &CollectRedis{
				Collector: &v1beta2.Database{
					URI: tt.uri,
				},
			}

			client, err := c.createClient()
			assert.Equal(t, err != nil, tt.hasError)
			if err == nil {
				require.NotNil(t, client)
				assert.Equal(t, client.Options().Addr, "localhost:6379")
			} else {
				t.Log(err)
				assert.Nil(t, client)
			}
		})
	}
}
