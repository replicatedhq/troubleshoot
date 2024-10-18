package supportbundle

import (
	"context"
	"testing"

	v1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace/noop"
)

func Test_filterHostCollectors(t *testing.T) {
	otel.SetTracerProvider(noop.NewTracerProvider())

	testCases := []struct {
		name           string
		collectSpecs   []*v1beta2.HostCollect
		bundlePath     string
		opts           SupportBundleCreateOpts
		expectedResult []FilteredCollector
		expectedError  error
	}{
		{
			name:         "nil host collectors spec",
			collectSpecs: []*v1beta2.HostCollect{},
			bundlePath:   "/tmp",
			opts: SupportBundleCreateOpts{
				ProgressChan: make(chan interface{}, 10),
			},
			expectedResult: []FilteredCollector{},
			expectedError:  nil,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filtered, err := filterHostCollectors(context.TODO(), tc.collectSpecs, tc.bundlePath, tc.opts)
			if err != tc.expectedError {
				t.Fatalf("expected error %v, got %v", tc.expectedError, err)
			}
			if len(filtered) != len(tc.expectedResult) {
				t.Fatalf("expected %d filtered collectors, got %d", len(tc.expectedResult), len(filtered))
			}
		})
	}
}
