package collect

import (
	"encoding/json"
	"net"
	"os"
	"strconv"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectHostUDPPortStatus_Collect(t *testing.T) {
	getPort := func() (int, *net.UDPConn, error) {
		listenAddress := net.UDPAddr{
			IP: net.ParseIP("0.0.0.0"),
		}
		conn, err := net.ListenUDP("udp", &listenAddress)
		if err != nil {
			return 0, nil, err
		}

		_, p, err := net.SplitHostPort(conn.LocalAddr().String())
		if err != nil {
			return 0, nil, err
		}
		port, err := strconv.Atoi(p)
		return port, conn, err
	}

	tests := []struct {
		name           string
		getPort        func(t *testing.T) (port int, closeFn func() error)
		wantStatus     string
		wantMsgContain string
		wantErr        bool
	}{
		{
			name: "connected",
			getPort: func(t *testing.T) (int, func() error) {
				port, conn, err := getPort()
				require.NoError(t, err)
				conn.Close()
				return port, nil
			},
			wantStatus:     "connected",
			wantMsgContain: "",
			wantErr:        false,
		},
		{
			name: "address-in-use",
			getPort: func(t *testing.T) (int, func() error) {
				port, conn, err := getPort()
				require.NoError(t, err)
				return port, conn.Close
			},
			wantStatus:     "address-in-use",
			wantMsgContain: "address already in use",
			wantErr:        true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port, closeFn := tt.getPort(t)
			if closeFn != nil {
				defer closeFn()
			}

			tmpDir, err := os.MkdirTemp("", "bundle")
			require.NoError(t, err)

			c := &CollectHostUDPPortStatus{
				hostCollector: &troubleshootv1beta2.UDPPortStatus{
					Port: port,
				},
				BundlePath: tmpDir,
			}

			progresChan := make(chan interface{})
			defer close(progresChan)
			go func() {
				for range progresChan {
				}
			}()
			got, err := c.Collect(progresChan)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			require.Len(t, got, 1)
			var result NetworkStatusResult
			err = json.Unmarshal(got["host-collectors/udpPortStatus/udpPortStatus.json"], &result)
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, string(result.Status))
			if tt.wantMsgContain != "" {
				assert.Contains(t, result.Message, tt.wantMsgContain)
			} else {
				assert.Empty(t, result.Message)
			}
		})
	}
}
