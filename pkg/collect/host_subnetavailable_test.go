package collect

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
func TestCollectHostSubnetAvailable_Collect(t *testing.T) {
	type fields struct {
		hostCollector *troubleshootv1beta2.SubnetAvailable
	}
	tests := []struct {
		name   string
		fields fields
		want   map[string][]byte
	}{
		{
			name: "TODO",
			want: map[string][]byte{
				"host-collectors/subnetAvailable/subnetAvailable.json": []byte(`{"status":"connected","message":""}`),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "bundle")
			require.NoError(t, err)

			c := &CollectHostSubnetAvailable{
				hostCollector: &troubleshootv1beta2.SubnetAvailable{
					// TODO: implement
				},
				BundlePath: tmpDir,
			}
		})
	}
}
*/

func TestParseProcNetRoute(t *testing.T) {
	input := `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth1	00000000	016BA8C0	0003	0	0	200	00000000	0	0	0
eth0	00000000	0205A8C0	0003	0	0	202	00000000	0	0	0
docker0	000011AC	00000000	0001	0	0	0	0000FFFF	0	0	0
eth0	0005A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
eth1	006BA8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0`

	/*
	   Destination     Gateway         Genmask         Flags Metric Ref    Use Iface
	   0.0.0.0         192.168.107.1   0.0.0.0         UG    200    0        0 eth1
	   0.0.0.0         192.168.5.2     0.0.0.0         UG    202    0        0 eth0
	   172.17.0.0      0.0.0.0         255.255.0.0     U     0      0        0 docker0
	   192.168.5.0     0.0.0.0         255.255.255.0   U     0      0        0 eth0
	   192.168.107.0   0.0.0.0         255.255.255.0   U     0      0        0 eth1
	*/

	expected := systemRoutes{
		systemRoute{
			Iface: "eth1",
			DestNet: net.IPNet{
				IP:   net.IPv4(0, 0, 0, 0),
				Mask: net.CIDRMask(0, 32),
			},
			Gateway: net.IPv4(192, 168, 107, 1),
			Metric:  uint32(200),
		},
		systemRoute{
			Iface: "eth0",
			DestNet: net.IPNet{
				IP:   net.IPv4(0, 0, 0, 0),
				Mask: net.CIDRMask(0, 32),
			},
			Gateway: net.IPv4(192, 168, 5, 2),
			Metric:  uint32(202),
		},
		systemRoute{
			Iface: "docker0",
			DestNet: net.IPNet{
				IP:   net.IPv4(172, 17, 0, 0),
				Mask: net.CIDRMask(16, 32),
			},
			Gateway: net.IPv4(0, 0, 0, 0),
			Metric:  uint32(0),
		},
		systemRoute{
			Iface: "eth0",
			DestNet: net.IPNet{
				IP:   net.IPv4(192, 168, 5, 0),
				Mask: net.CIDRMask(24, 32),
			},
			Gateway: net.IPv4(0, 0, 0, 0),
			Metric:  uint32(0),
		},
		systemRoute{
			Iface: "eth1",
			DestNet: net.IPNet{
				IP:   net.IPv4(192, 168, 107, 0),
				Mask: net.CIDRMask(24, 32),
			},
			Gateway: net.IPv4(0, 0, 0, 0),
			Metric:  uint32(0),
		},
	}

	result, err := parseProcNetRoute(input)

	require.NoError(t, err)
	assert.Equal(t, expected, result)
}
