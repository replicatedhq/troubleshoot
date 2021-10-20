package collect

import (
	"context"
	"reflect"
	"testing"
	"time"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/apiserver/pkg/storage/names"
)

type testRunner struct {
	delay time.Duration
}

func (r *testRunner) run(ctx context.Context, collector *troubleshootv1beta2.HostCollect, namespace string, name string, nodeName string, results chan<- map[string][]byte) error {
	output := map[string][]byte{
		nodeName: []byte("logdata"),
	}

	delay := r.delay
	if delay == 0 {
		delay = time.Millisecond
	}

	ticker := time.NewTicker(delay)

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ticker.C:
	}

	results <- output
	return nil
}

func TestRemoteCollector_RunRemote(t *testing.T) {
	tests := []struct {
		name       string
		nodes      []string
		delay      time.Duration
		timeout    time.Duration
		want       map[string][]byte
		wantErr    bool
		wantErrStr string
	}{
		{
			name:    "no timeout",
			nodes:   []string{"1", "2", "3", "4", "5"},
			timeout: 2 * time.Second,
			want: map[string][]byte{
				"1": []byte("logdata"),
				"2": []byte("logdata"),
				"3": []byte("logdata"),
				"4": []byte("logdata"),
				"5": []byte("logdata"),
			},
		},
		{
			name:       "timeout",
			nodes:      []string{"1", "2", "3", "4", "5"},
			delay:      200 * time.Millisecond,
			timeout:    100 * time.Millisecond,
			want:       nil,
			wantErr:    true,
			wantErrStr: "failed remote collection: context deadline exceeded",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tt.timeout)
			defer cancel()

			c := &RemoteCollector{
				Namespace: "test-ns",
				Timeout:   tt.timeout,
			}

			hc := &troubleshootv1beta2.HostCollect{
				CPU: &troubleshootv1beta2.CPU{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{},
				},
			}
			got, err := c.RunRemote(ctx, &testRunner{delay: tt.delay}, tt.nodes, hc, names.SimpleNameGenerator, "test")
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoteCollector.RunRemote() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.wantErrStr != "" && err.Error() != tt.wantErrStr {
				t.Errorf("RemoteCollector.RunRemote() error msg = %s, wantErrStr %s", err.Error(), tt.wantErrStr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("RemoteCollector.RunRemote() = %v, want %v", got, tt.want)
			}
		})
	}
}
