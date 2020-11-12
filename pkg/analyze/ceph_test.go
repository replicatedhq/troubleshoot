package analyzer

import (
	"fmt"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.undefinedlabs.com/scopeagent"
)

func Test_cephStatus(t *testing.T) {
	tests := []struct {
		name           string
		analyzer       troubleshootv1beta2.CephStatusAnalyze
		expectResult   AnalyzeResult
		filePath, file string
	}{
		{
			name:     "pass case",
			analyzer: troubleshootv1beta2.CephStatusAnalyze{},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "Ceph Status",
				Message: "Ceph is healthy",
				IconKey: "rook",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/rook.svg?w=11&h=16",
			},
			filePath: "ceph/status.json",
			file: `{
				"fsid": "96a8178c-6aa2-4adf-a309-9e8869a79611",
				"health": {
					"status": "HEALTH_OK"
				}
			}`,
		},
		{
			name:     "warn case",
			analyzer: troubleshootv1beta2.CephStatusAnalyze{},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  true,
				IsFail:  false,
				Title:   "Ceph Status",
				Message: "Ceph status is HEALTH_WARN",
				URI:     "https://rook.io/docs/rook/v1.4/ceph-common-issues.html",
				IconKey: "rook",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/rook.svg?w=11&h=16",
			},
			filePath: "ceph/status.json",
			file: `{
				"fsid": "96a8178c-6aa2-4adf-a309-9e8869a79611",
				"health": {
					"status": "HEALTH_WARN"
				}
			}`,
		},
		{
			name:     "fail case",
			analyzer: troubleshootv1beta2.CephStatusAnalyze{},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "Ceph Status",
				Message: "Ceph status is HEALTH_ERR",
				URI:     "https://rook.io/docs/rook/v1.4/ceph-common-issues.html",
				IconKey: "rook",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/rook.svg?w=11&h=16",
			},
			filePath: "ceph/status.json",
			file: `{
				"fsid": "96a8178c-6aa2-4adf-a309-9e8869a79611",
				"health": {
					"status": "HEALTH_ERR"
				}
			}`,
		},
		{
			name: "CollectorName and Namespace",
			analyzer: troubleshootv1beta2.CephStatusAnalyze{
				CollectorName: "custom-namespace",
				Namespace:     "namespace",
			},
			expectResult: AnalyzeResult{
				IsPass:  true,
				IsWarn:  false,
				IsFail:  false,
				Title:   "Ceph Status",
				Message: "Ceph is healthy",
				IconKey: "rook",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/rook.svg?w=11&h=16",
			},
			filePath: "custom-namespace/namespace/ceph/status.json",
			file: `{
				"fsid": "96a8178c-6aa2-4adf-a309-9e8869a79611",
				"health": {
					"status": "HEALTH_OK"
				}
			}`,
		},
		{
			name: "outcomes when",
			analyzer: troubleshootv1beta2.CephStatusAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "== HEALTH_OK",
							Message: "custom message OK",
							URI:     "custom uri OK",
						},
					},
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "<= HEALTH_WARN",
							Message: "custom message WARN",
							URI:     "custom uri WARN",
						},
					},
				},
			},
			expectResult: AnalyzeResult{
				IsPass:  false,
				IsWarn:  false,
				IsFail:  true,
				Title:   "Ceph Status",
				Message: "custom message WARN",
				URI:     "custom uri WARN",
				IconKey: "rook",
				IconURI: "https://troubleshoot.sh/images/analyzer-icons/rook.svg?w=11&h=16",
			},
			filePath: "ceph/status.json",
			file: `{
				"fsid": "96a8178c-6aa2-4adf-a309-9e8869a79611",
				"health": {
					"status": "HEALTH_WARN"
				}
			}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scopetest := scopeagent.StartTest(t)
			defer scopetest.End()
			req := require.New(t)

			getFile := func(n string) ([]byte, error) {
				assert.Equal(t, n, test.filePath)
				return []byte(test.file), nil
			}

			actual, err := cephStatus(&test.analyzer, getFile)
			req.NoError(err)

			assert.Equal(t, test.expectResult, *actual)
		})
	}
}

func Test_compareCephStatus(t *testing.T) {
	tests := []struct {
		actual  string
		when    string
		want    bool
		wantErr bool
	}{
		{
			actual: "HEALTH_OK",
			when:   "HEALTH_OK",
			want:   true,
		},
		{
			actual: "HEALTH_OK",
			when:   "HEALTH_WARN",
			want:   false,
		},
		{
			actual: "HEALTH_OK",
			when:   "<= HEALTH_WARN",
			want:   false,
		},
		{
			actual: "HEALTH_OK",
			when:   ">= HEALTH_WARN",
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s %s", tt.actual, tt.when), func(t *testing.T) {
			got, err := compareCephStatus(tt.actual, tt.when)
			if (err != nil) != tt.wantErr {
				t.Errorf("compareCephStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("compareCephStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
