package collect

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestObserveHypervisorClock(t *testing.T) {
	start := time.Unix(1700000000, 0)
	for _, tc := range []struct {
		name            string
		wall, monotonic time.Duration
		wantError       bool
	}{
		{"stable", 2 * time.Millisecond, 2 * time.Millisecond, false},
		{"sampling noise", 2 * time.Millisecond, 2*time.Millisecond + time.Microsecond, false},
		{"slow read", time.Second, time.Second, true},
		{"backward duration", -time.Millisecond, time.Millisecond, true},
		// Both durations are valid individually, but a clock step invalidates the sample.
		{"backward step", time.Millisecond, 90 * time.Millisecond, true},
		{"forward step", 90 * time.Millisecond, time.Millisecond, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := observeHypervisorClockSample(kvmPTPClockName, start, start.Add(955*time.Millisecond), start.Add(tc.wall), tc.monotonic)
			if tc.wantError {
				require.Error(t, err)
				require.Nil(t, ref)
				return
			}
			require.NoError(t, err)
			require.Equal(t, int64(tc.wall/2-955*time.Millisecond), ref.OffsetNanoseconds)
			require.Equal(t, int64(tc.wall), ref.ReadDurationNanoseconds)
			require.True(t, (TimeInfo{HypervisorClock: ref}).ClockSynchronized())
		})
	}
	// Dates centuries apart must not overflow into an apparently small offset.
	before := time.Date(2600, 1, 1, 0, 0, 0, 0, time.UTC)
	ref, err := observeHypervisorClock(kvmPTPClockName, before, start, before.Add(time.Millisecond))
	require.NoError(t, err)
	require.False(t, (TimeInfo{HypervisorClock: ref}).ClockSynchronized())
}
