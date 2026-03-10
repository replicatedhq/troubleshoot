package supportbundle

import (
	"testing"
)

func TestGetCollectTimeout(t *testing.T) {
	t.Run("value not set returns default", func(t *testing.T) {
		opts := SupportBundleCreateOpts{}
		actualTimeout := getCollectTimeout(opts)
		if actualTimeout != defaultTimeout {
			t.Errorf("getCollectTimeout() = %d, want default %d", actualTimeout, defaultTimeout)
		}
	})

	t.Run("value set returns configured timeout", func(t *testing.T) {
		opts := SupportBundleCreateOpts{RemoteHostCollectTimeoutSeconds: 90}
		actualTimeout := getCollectTimeout(opts)
		expectedTimeout := 90
		if actualTimeout != expectedTimeout {
			t.Errorf("getCollectTimeout() = %d, want %d", actualTimeout, expectedTimeout)
		}
	})
}
