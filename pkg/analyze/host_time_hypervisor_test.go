package analyzer

import (
	"fmt"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/require"
)

func TestHostClock(t *testing.T) {
	a := &AnalyzeHostTime{hostAnalyzer: &troubleshootv1beta2.TimeAnalyze{Outcomes: []*troubleshootv1beta2.Outcome{
		{Pass: &troubleshootv1beta2.SingleOutcome{When: "clock == synchronized"}},
		{Fail: &troubleshootv1beta2.SingleOutcome{Message: "Clock not verified"}},
	}}}
	for _, tc := range []struct {
		name, data string
		pass       bool
	}{
		{"no reference", `{}`, false},
		{"NTP active only", `{"ntp_active":true}`, false},
		{"legacy NTP synchronized", `{"ntp_synchronized":true}`, true},
		{"KVM", `{"hypervisor_clock":{"source":"KVM virtual PTP","offset_nanoseconds":1000000}}`, true},
		{"Hyper-V", `{"hypervisor_clock":{"source":"hyperv","offset_nanoseconds":1000000}}`, true},
		{"VMware", `{"hypervisor_clock":{"source":"ptp_vmw","offset_nanoseconds":1000000}}`, true},
		{"near limit ahead", `{"hypervisor_clock":{"source":"hyperv","offset_nanoseconds":999000000,"read_duration_nanoseconds":100000}}`, true},
		{"near limit behind", `{"hypervisor_clock":{"source":"ptp_vmw","offset_nanoseconds":-999000000,"read_duration_nanoseconds":100000}}`, true},
		{"uncertain boundary", `{"hypervisor_clock":{"source":"KVM virtual PTP","offset_nanoseconds":1000000000,"read_duration_nanoseconds":100000}}`, false},
		{"wrong ahead", `{"hypervisor_clock":{"source":"hyperv","offset_nanoseconds":60000000000}}`, false},
		{"wrong behind", `{"hypervisor_clock":{"source":"ptp_vmw","offset_nanoseconds":-60000000000}}`, false},
		{"NIC", `{"hypervisor_clock":{"source":"nic PTP"}}`, false},
		{"slow read", `{"hypervisor_clock":{"source":"KVM virtual PTP","read_duration_nanoseconds":200000000}}`, false},
		{"negative duration", `{"hypervisor_clock":{"source":"KVM virtual PTP","read_duration_nanoseconds":-1}}`, false},
		{"failed read", `{"hypervisor_clock_error":"read failed","hypervisor_clock":{"source":"hyperv"}}`, false},
		{"NTP despite failed read", `{"ntp_synchronized":true,"hypervisor_clock_error":"read failed"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for condition, want := range map[string]bool{"clock == synchronized": tc.pass, "clock == unsynchronized": !tc.pass} {
				matched, err := a.CheckCondition(condition, []byte(tc.data))
				require.NoError(t, err)
				require.Equal(t, want, matched, condition)
			}
			results, err := a.Analyze(func(string) ([]byte, error) { return []byte(tc.data), nil }, nil)
			require.NoError(t, err)
			require.Len(t, results, 1)
			require.Equal(t, tc.pass, results[0].IsPass)
			require.Equal(t, !tc.pass, results[0].IsFail)
		})
	}
	for state, active := range map[string]bool{"active": true, "inactive": false} {
		data := []byte(fmt.Sprintf(`{"ntp_active":%t,"hypervisor_clock":{"source":"hyperv"}}`, active))
		matched, err := a.CheckCondition("ntp == synchronized+"+state, data)
		require.NoError(t, err)
		require.False(t, matched, "hypervisor agreement must not change NTP status")
	}
	for _, condition := range []string{"clock != synchronized", "clock == unknown"} {
		_, err := a.CheckCondition(condition, []byte(`{}`))
		require.Error(t, err)
	}
}
