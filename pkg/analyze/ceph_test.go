package analyzer

import (
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
			},
			filePath: "ceph/status.txt",
			file: `cluster:
  id:     477e46f1-ae41-4e43-9c8f-72c918ab0a20
  health: HEALTH_OK

services:
  mon: 3 daemons, quorum a,b,c
`,
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
			},
			filePath: "ceph/status.txt",
			file: `  cluster:
    id:     ff2062fe-a2d7-477a-8f4f-93f892e70554
    health: HEALTH_WARN
            Degraded data redundancy: 1354/4062 objects degraded (33.333%), 224 pgs degraded, 700 pgs undersized

  services:
    mon: 1 daemons, quorum a (age 23h)
`,
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
			},
			filePath: "ceph/status.txt",
			file: `  cluster:
    id:     b683c5f1-fd15-4805-83c0-add6fbb7faae
    health: HEALTH_ERR
            1 backfillfull osd(s)
            8 pool(s) backfillfull
            50873/1090116 objects misplaced (4.667%)
            Degraded data redundancy: 34149/1090116 objects degraded (3.133%), 3 pgs degraded, 3 pgs undersized
            Degraded data redundancy (low space): 6 pgs backfill_toofull

  services:
    mon: 3 daemons, quorum tb-ceph-2-prod,tb-ceph-4-prod,tb-ceph-3-prod
`,
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
			},
			filePath: "custom-namespace/namespace/ceph/status.txt",
			file: `cluster:
  id:     477e46f1-ae41-4e43-9c8f-72c918ab0a20
  health: HEALTH_OK

services:
  mon: 3 daemons, quorum a,b,c
`,
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
