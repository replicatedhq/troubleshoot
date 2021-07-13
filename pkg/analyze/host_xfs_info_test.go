package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeXFSInfo(t *testing.T) {
	tests := []struct {
		name     string
		xfsInfo  string
		analyzer *troubleshootv1beta2.XFSInfoAnalyze
		expect   []*AnalyzeResult
	}{
		{
			name: "not xfs",
			xfsInfo: `
isXFS: false
isFtypeEnabled: false`,
			analyzer: &troubleshootv1beta2.XFSInfoAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    NOT_XFS,
							Message: "Not an xfs filesystem",
						},
					},
				},
			},
			expect: []*AnalyzeResult{
				{
					Title:   "XFS Info",
					IsWarn:  true,
					Message: "Not an xfs filesystem",
				},
			},
		},
		{
			name: "ftype enabled",
			xfsInfo: `
isXFS: true
isFtypeEnabled: true`,
			analyzer: &troubleshootv1beta2.XFSInfoAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    XFS_FTYPE_ENABLED,
							Message: "Filesystem is compatible with overlay2",
						},
					},
				},
			},
			expect: []*AnalyzeResult{
				{
					Title:   "XFS Info",
					IsPass:  true,
					Message: "Filesystem is compatible with overlay2",
				},
			},
		},
		{
			name: "ftype disable",
			xfsInfo: `
isXFS: true
isFtypeEnabled: false`,
			analyzer: &troubleshootv1beta2.XFSInfoAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    XFS_FTYPE_DISABLED,
							Message: "Filesystem is not compatible with overlay2",
						},
					},
				},
			},
			expect: []*AnalyzeResult{
				{
					Title:   "XFS Info",
					IsFail:  true,
					Message: "Filesystem is not compatible with overlay2",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getFile := func(filename string) ([]byte, error) {
				return []byte(test.xfsInfo), nil
			}

			got, err := (&AnalyzeXFSInfo{test.analyzer}).Analyze(getFile)
			assert.NoError(t, err)
			assert.Equal(t, test.expect, got)
		})
	}
}
