package analyzer

import (
	"encoding/json"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeHostServices(t *testing.T) {
	tests := []struct {
		name         string
		info         []collect.ServiceInfo
		hostAnalyzer *troubleshootv1beta2.HostServicesAnalyze
		result       []*AnalyzeResult
		expectErr    bool
	}{
		{
			name: "service 'a' is active",
			info: []collect.ServiceInfo{
				{
					Unit:   "a.service",
					Active: "active",
				},
			},
			hostAnalyzer: &troubleshootv1beta2.HostServicesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "a.service == active",
							Message: "the service 'a' is active",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host Services",
					IsFail:  true,
					Message: "the service 'a' is active",
				},
			},
		},
		{
			name: "connected, fail",
			info: []collect.ServiceInfo{
				{
					Unit:   "a.service",
					Active: "active",
				},
				{
					Unit:   "b.service",
					Active: "inactive",
				},
			},
			hostAnalyzer: &troubleshootv1beta2.HostServicesAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "a.service != active",
							Message: "service 'a' is active",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "b.service != active",
							Message: "service 'b' is not active",
						},
					},
				},
			},
			result: []*AnalyzeResult{
				{
					Title:   "Host Services",
					IsPass:  true,
					Message: "service 'b' is not active",
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

			result, err := (&AnalyzeHostServices{test.hostAnalyzer}).Analyze(getCollectedFileContents)
			if test.expectErr {
				req.Error(err)
			} else {
				req.NoError(err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}

func Test_compareHostServicesConditionalToActual(t *testing.T) {
	tests := []struct {
		name        string
		conditional string
		services    []collect.ServiceInfo
		wantRes     bool
		wantErr     bool
	}{
		{
			name:        "match second item",
			conditional: "abc.service = active",
			services: []collect.ServiceInfo{
				{
					Unit: "first",
				},
				{
					Unit:   "abc.service",
					Active: "active",
					Sub:    "running",
				},
			},
			wantRes: true,
		},
		{
			name:        "item not in list",
			conditional: "abc = active",
			services: []collect.ServiceInfo{
				{
					Unit: "first",
				},
			},
			wantRes: false,
		},
		{
			name:        "item does not match",
			conditional: "abc = active",
			services: []collect.ServiceInfo{
				{
					Unit:   "abc.service",
					Active: "inactive",
					Sub:    "exited",
				},
			},
			wantRes: false,
		},
		{
			name:        "other operator",
			conditional: "abc * active",
			services: []collect.ServiceInfo{
				{
					Unit:   "abc.service",
					Active: "inactive",
					Sub:    "exited",
				},
			},
			wantErr: true,
		},
		{
			name:        "item active matches but not sub",
			conditional: "abc = active,running",
			services: []collect.ServiceInfo{
				{
					Unit:   "abc.service",
					Active: "active",
					Sub:    "exited",
				},
			},
			wantRes: false,
		},
		{
			name:        "item active,sub,load matches",
			conditional: "abc = active,*,loaded",
			services: []collect.ServiceInfo{
				{
					Unit:   "abc.service",
					Active: "active",
					Sub:    "exited",
					Load:   "loaded",
				},
			},
			wantRes: true,
		},
		{
			name:        "one item active,sub,load does not match with !=",
			conditional: "abc != active,running,loaded",
			services: []collect.ServiceInfo{
				{
					Unit:   "abc.service",
					Active: "active",
					Sub:    "exited",
					Load:   "loaded",
				},
			},
			wantRes: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			gotRes, err := compareHostServicesConditionalToActual(tt.conditional, tt.services)
			if tt.wantErr {
				req.Error(err)
			} else {
				req.NoError(err)
				req.Equal(tt.wantRes, gotRes)
			}
		})
	}
}
