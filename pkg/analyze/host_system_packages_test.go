package analyzer

import (
	"encoding/json"
	"fmt"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeSystemPackages(t *testing.T) {
	tests := []struct {
		name         string
		info         collect.SystemPackagesInfo
		hostAnalyzer *troubleshootv1beta2.SystemPackagesAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name: "basic",
			info: collect.SystemPackagesInfo{
				OS:        "ubuntu",
				OSVersion: "18.04",
				Packages: []collect.SystemPackage{
					{
						Name:     "libzstd",
						Details:  "installed",
						ExitCode: "0",
						Error:    "",
					},
					{
						Name:     "nfs-common",
						Details:  "not installed",
						ExitCode: "1",
						Error:    "package 'nfs-common' is not installed and no information is available",
					},
					{
						Name:     "iscsi-initiator-utils",
						Details:  "whatever",
						ExitCode: "1",
						Error:    "whatever",
					},
					{
						Name:     "open-iscsi",
						Details:  "No matching Packages for 'open-iscsi'",
						ExitCode: "0",
						Error:    "whatever",
					},
				},
			},
			hostAnalyzer: &troubleshootv1beta2.SystemPackagesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "{{ not .IsInstalled }}",
							Message: "Package {{ .Name }} is not installed.",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Package {{ .Name }} is installed.",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "System Packages",
					Message: "Package libzstd is installed.",
					IsPass:  true,
				},
				{
					Title:   "System Packages",
					Message: "Package nfs-common is not installed.",
					IsFail:  true,
				},
				{
					Title:   "System Packages",
					Message: "Package iscsi-initiator-utils is not installed.",
					IsFail:  true,
				},
				{
					Title:   "System Packages",
					Message: "Package open-iscsi is not installed.",
					IsFail:  true,
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)
			b, err := json.Marshal(test.info)
			if err != nil {
				t.Fatal(err)
			}

			getCollectedFileContents := func(filename string) ([]byte, error) {
				return b, nil
			}

			result, err := (&AnalyzeHostSystemPackages{test.hostAnalyzer}).Analyze(getCollectedFileContents, nil)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			for _, v := range result {
				c, _ := json.Marshal(v)
				fmt.Println(string(c))
			}

			assert.Equal(t, test.result, result)
		})
	}
}
