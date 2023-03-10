package collect

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestIsASubnetAvailableInCIDR(t *testing.T) {
	// Single routing table used for all tests
	sysRoutes := systemRoutes{
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
			Iface: "docker1",
			DestNet: net.IPNet{
				IP:   net.IPv4(172, 16, 0, 0),
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

	tests := []struct {
		name        string
		cidrRange   int
		subnetRange net.IPNet
		expected    bool
	}{
		{
			name:      "unavailable 1",
			cidrRange: 24,
			subnetRange: net.IPNet{
				IP:   net.IPv4(172, 17, 0, 0),
				Mask: net.CIDRMask(20, 32),
			},
			expected: false,
		},
		{
			name:      "unavailable 2",
			cidrRange: 27,
			subnetRange: net.IPNet{
				IP:   net.IPv4(192, 168, 5, 0),
				Mask: net.CIDRMask(24, 32),
			},
			expected: false,
		},
		{
			name:      "available 1",
			cidrRange: 23,
			subnetRange: net.IPNet{
				IP:   net.IPv4(172, 20, 0, 0),
				Mask: net.CIDRMask(16, 32),
			},
			expected: true,
		},
		{
			name:      "available 2",
			cidrRange: 24,
			subnetRange: net.IPNet{
				IP:   net.IPv4(10, 0, 0, 0),
				Mask: net.CIDRMask(8, 32),
			},
			expected: true,
		},
		{
			name:      "available 3",
			cidrRange: 24,
			subnetRange: net.IPNet{
				IP:   net.IPv4(172, 16, 0, 0),
				Mask: net.CIDRMask(12, 32),
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := require.New(t)
			actual, err := isASubnetAvailableInCIDR(tt.cidrRange, &tt.subnetRange, &sysRoutes) // debug bool is useful for fixing bugs here, but off by default for noise
			req.NoError(err)

			assert.Equal(t, tt.expected, actual)
		})
	}
}
