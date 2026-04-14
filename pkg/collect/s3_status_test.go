package collect

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectS3Status_Collect(t *testing.T) {
	tests := []struct {
		name            string
		handler         http.HandlerFunc
		collectorName   string
		wantConnected   bool
		wantErrContains string
	}{
		{
			name: "bucket exists",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodHead && r.URL.Path == "/test-bucket" {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusNotFound)
			}),
			collectorName: "mybucket",
			wantConnected: true,
		},
		{
			name: "bucket not found",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}),
			collectorName:   "mybucket",
			wantConnected:   false,
			wantErrContains: "StatusCode: 404",
		},
		{
			name: "access denied",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			}),
			collectorName:   "mybucket",
			wantConnected:   false,
			wantErrContains: "StatusCode: 403",
		},
		{
			name: "default collector name",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}),
			collectorName: "",
			wantConnected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()

			collector := &CollectS3Status{
				Collector: &troubleshootv1beta2.S3Status{
					CollectorMeta: troubleshootv1beta2.CollectorMeta{
						CollectorName: tt.collectorName,
					},
					BucketName:      "test-bucket",
					Endpoint:        ts.URL,
					Region:          "us-east-1",
					AccessKeyID:     "test-key",
					SecretAccessKey: "test-secret",
					UsePathStyle:    true,
				},
				BundlePath: "",
			}

			result, err := collector.Collect(nil)
			require.NoError(t, err)

			expectedName := tt.collectorName
			if expectedName == "" {
				expectedName = "s3Status"
			}

			key := "s3Status/" + expectedName + ".json"
			raw, ok := result[key]
			require.True(t, ok, "expected key %s in result, got keys: %v", key, result)

			var s3Result S3StatusResult
			err = json.Unmarshal(raw, &s3Result)
			require.NoError(t, err)

			assert.Equal(t, tt.wantConnected, s3Result.IsConnected)
			assert.Equal(t, "test-bucket", s3Result.BucketName)

			if tt.wantErrContains != "" {
				assert.Contains(t, s3Result.Error, tt.wantErrContains)
			} else {
				assert.Empty(t, s3Result.Error)
			}
		})
	}
}
