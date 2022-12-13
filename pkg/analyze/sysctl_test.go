package analyzer

import (
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
)

func TestParseSysctlParameters(t *testing.T) {
	parameters := `
/proc/sys/net/ipv4/ip_forward = 1
/proc/sys/net/ipv4/ip_local_port_range = 32768 60999
/proc/sys/net/bridge/bridge-nf-call-iptables = 0
/proc/sys/vm/max_map_count = 65530
`
	got := parseSysctlParameters([]byte(parameters))
	expect := map[string]string{
		"net.ipv4.ip_forward":                "1",
		"net.ipv4.ip_local_port_range":       "32768 60999",
		"net.bridge.bridge-nf-call-iptables": "0",
		"vm.max_map_count":                   "65530",
	}

	assert.Equal(t, expect, got)
}

func TestEvalSysctlWhen(t *testing.T) {
	tests := []struct {
		name       string
		when       string
		nodeParams map[string]map[string]string
		expect     []string
		expectErr  bool
	}{
		{
			name: "One node with IP forwarding disabled",
			when: "net.ipv4.ip_forward = 0",
			nodeParams: map[string]map[string]string{
				"node-a": {"net.ipv4.ip_forward": "0"},
			},
			expect:    []string{"node-a"},
			expectErr: false,
		},
		{
			name: "All nodes have IP forwarding disabled",
			when: "net.ipv4.ip_forward = 0",
			nodeParams: map[string]map[string]string{
				"node-a": {"net.ipv4.ip_forward": "0"},
				"node-b": {"net.ipv4.ip_forward": "0"},
			},
			expect:    []string{"node-a", "node-b"},
			expectErr: false,
		},
		{
			name: "No nodes have net.ipv4.ip_forward",
			when: "net.ipv4.ip_forward = 0",
			nodeParams: map[string]map[string]string{
				"node-a": {},
			},
			expect:    []string{},
			expectErr: false,
		},
		{
			name: "One node has IP forwarding enabled, one disabled",
			when: "net.ipv4.ip_forward = 0",
			nodeParams: map[string]map[string]string{
				"node-a": {"net.ipv4.ip_forward": "1"},
				"node-b": {"net.ipv4.ip_forward": "0"},
			},
			expect:    []string{"node-b"},
			expectErr: false,
		},
		{
			name: "No nodes have max map count > 65530",
			when: "vm.max_map_count > 65530",
			nodeParams: map[string]map[string]string{
				"node-a": {"vm.max_map_count": "65530"},
				"node-b": {"vm.max_map_count": "65530"},
			},
			expect:    []string{},
			expectErr: false,
		},
		{
			name: "One node has max map count > 65530",
			when: "vm.max_map_count > 65530",
			nodeParams: map[string]map[string]string{
				"node-a": {"vm.max_map_count": "65530"},
				"node-b": {"vm.max_map_count": "262144"},
			},
			expect:    []string{"node-b"},
			expectErr: false,
		},
		{
			name: "All nodes have max map count > 65530",
			when: "vm.max_map_count > 65530",
			nodeParams: map[string]map[string]string{
				"node-a": {"vm.max_map_count": "262144"},
				"node-b": {"vm.max_map_count": "262144"},
			},
			expect:    []string{"node-a", "node-b"},
			expectErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := evalSysctlWhen(test.nodeParams, test.when)
			if test.expectErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			assert.ElementsMatch(t, test.expect, got)
		})
	}
}

func TestAnalyzeSysctl(t *testing.T) {
	var tests = []struct {
		name     string
		files    map[string][]byte
		analyzer *troubleshootv1beta2.SysctlAnalyze
		expect   *AnalyzeResult
	}{
		{
			name: "Fail IP forwarding disabled on one node",
			files: map[string][]byte{
				"a": []byte(`
/proc/sys/net/ipv4/ip_forward = 1
`),
				"b": []byte(`
/proc/sys/net/ipv4/ip_forward = 0
`),
			},
			analyzer: &troubleshootv1beta2.SysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.ip_forward = 0 ",
							Message: "IP forwarding disabled",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "IP forwarding enabled",
						},
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Sysctl",
				IsFail:  true,
				Message: "Node b: IP forwarding disabled",
			},
		},
		{
			name: "Fail IP forwarding disabled on all nodes",
			files: map[string][]byte{
				"a": []byte(`
/proc/sys/net/ipv4/ip_forward = 0
`),
				"b": []byte(`
/proc/sys/net/ipv4/ip_forward = 0
`),
			},
			analyzer: &troubleshootv1beta2.SysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.ip_forward = 0",
							Message: "IP forwarding disabled",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "IP forwarding enabled",
						},
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Sysctl",
				IsFail:  true,
				Message: "Nodes a, b: IP forwarding disabled",
			},
		},
		{
			name: "Pass IP forwarding disabled on all nodes",
			files: map[string][]byte{
				"a": []byte(`
/proc/sys/net/ipv4/ip_forward = 1
`),
				"b": []byte(`
/proc/sys/net/ipv4/ip_forward = 1
`),
			},
			analyzer: &troubleshootv1beta2.SysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.ip_forward = 0",
							Message: "IP forwarding disabled",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.ip_forward = 1",
							Message: "IP forwarding enabled",
						},
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Sysctl",
				IsPass:  true,
				Message: "Nodes a, b: IP forwarding enabled",
			},
		},
		{
			name: "Pass IP forwarding enabled on one node",
			files: map[string][]byte{
				"a": []byte{},
				"b": []byte(`
/proc/sys/net/ipv4/ip_forward = 1
`),
			},
			analyzer: &troubleshootv1beta2.SysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.ip_forward = 0",
							Message: "IP forwarding disabled",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.ip_forward = 1",
							Message: "IP forwarding enabled",
						},
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Sysctl",
				IsPass:  true,
				Message: "Node b: IP forwarding enabled",
			},
		},
		{
			name: "Default warn with empty when",
			files: map[string][]byte{
				"a": []byte{},
				"b": []byte{},
			},
			analyzer: &troubleshootv1beta2.SysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.ip_forward = 0",
							Message: "IP forwarding disabled",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.ip_forward = 1",
							Message: "IP forwarding enabled",
						},
					},
					{
						Warn: &troubleshootv1beta2.SingleOutcome{
							When:    "",
							Message: "IP forwarding kernel parameters not found",
						},
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Sysctl",
				IsWarn:  true,
				Message: "IP forwarding kernel parameters not found",
			},
		},
		{
			name: "No result",
			files: map[string][]byte{
				"a": []byte(`
/proc/sys/net/ipv4/ip_forward = 1
`),
			},
			analyzer: &troubleshootv1beta2.SysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "net.ipv4.ip_forward = 0",
							Message: "IP forwarding disabled",
						},
					},
				},
			},
			expect: nil,
		},
		{
			name: "Fail one node too low on max map count",
			files: map[string][]byte{
				"a": []byte(`
/proc/sys/vm/max_map_count = 65530
`),
				"b": []byte(`
/proc/sys/vm/max_map_count = 262144
`),
			},
			analyzer: &troubleshootv1beta2.SysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "vm.max_map_count < 262144 ",
							Message: "Max map count too low",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Max map count sufficient",
						},
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Sysctl",
				IsFail:  true,
				Message: "Node a: Max map count too low",
			},
		},
		{
			name: "Fail two nodes too low on max map count",
			files: map[string][]byte{
				"a": []byte(`
/proc/sys/vm/max_map_count = 65530
`),
				"b": []byte(`
/proc/sys/vm/max_map_count = 65530
`),
			},
			analyzer: &troubleshootv1beta2.SysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "vm.max_map_count < 262144 ",
							Message: "Max map count too low",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							Message: "Max map count sufficient",
						},
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Sysctl",
				IsFail:  true,
				Message: "Nodes a, b: Max map count too low",
			},
		},
		{
			name: "Pass sufficient max map count on both nodes",
			files: map[string][]byte{
				"a": []byte(`
/proc/sys/vm/max_map_count = 262144
`),
				"b": []byte(`
/proc/sys/vm/max_map_count = 262144
`),
			},
			analyzer: &troubleshootv1beta2.SysctlAnalyze{
				Outcomes: []*troubleshootv1beta2.Outcome{
					{
						Fail: &troubleshootv1beta2.SingleOutcome{
							When:    "vm.max_map_count < 262144 ",
							Message: "Max map count too low",
						},
					},
					{
						Pass: &troubleshootv1beta2.SingleOutcome{
							When:    "vm.max_map_count >= 262144 ",
							Message: "Max map count sufficient",
						},
					},
				},
			},
			expect: &AnalyzeResult{
				Title:   "Sysctl",
				IsPass:  true,
				Message: "Nodes a, b: Max map count sufficient",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var findFiles = func(glob string, _ []string) (map[string][]byte, error) {
				return test.files, nil
			}
			got, err := analyzeSysctl(test.analyzer, findFiles)

			assert.NoError(t, err)

			assert.Equal(t, test.expect, got)
		})
	}
}
