package collect

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/apparentlymart/go-cidr/cidr"
	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/debug"
)

type SubnetAvailableResult struct {
	CIDRRangeAlloc string `json:"CIDRRangeAlloc"`
	DesiredCIDR    string `json:"desiredCIDR"`
	// If true, at least 1 of the DesiredCIDR size is available within CIDRRangeAlloc
	ADesiredIsAvailable bool `json:"aDesiredIsAvailable"`
}

type CollectHostSubnetAvailable struct {
	hostCollector *troubleshootv1beta2.SubnetAvailable
	BundlePath    string
}

func (c *CollectHostSubnetAvailable) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Subnet Available")
}

func (c *CollectHostSubnetAvailable) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostSubnetAvailable) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	procNetRoute, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return nil, errors.Wrap(err, "failed to read contents of /proc/net/route")
	}

	routes, err := parseProcNetRoute(string(procNetRoute))
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse /proc/net/route")
	}
	debug.Printf("Routes: %+v\n", routes)

	// c.hostCollector.CIDRRangeAlloc
	// c.hostCollector.DesiredCIDR
	// TODO: parse into types we can use

	result := SubnetAvailableResult{}
	result.CIDRRangeAlloc = c.hostCollector.CIDRRangeAlloc
	result.DesiredCIDR = c.hostCollector.DesiredCIDR
	// TODO: populate result.DesiredIsAvailable true/false

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "subnetAvailable"
	}
	name := filepath.Join("host-collectors/subnetAvailable", collectorName+".json")

	output := NewResult()
	output.SaveResult(c.BundlePath, name, bytes.NewBuffer(b))

	return map[string][]byte{
		name: b,
	}, nil
}

type systemRoutes []systemRoute

type systemRoute struct {
	Iface   string
	DestNet net.IPNet
	Gateway net.IP
	Metric  uint32
}

// Parses the output of /proc/net/route into something useful
// This only deals with IPv4 - another file /proc/net/ipv6_route deals with IPv6 (not implemented here)
func parseProcNetRoute(input string) (systemRoutes, error) {
	routes := systemRoutes{}
	for _, line := range strings.Split(input, "\n") {
		if line[0:5] == "Iface" {
			continue
		}

		splitLine := strings.Split(strings.TrimSpace(line), "\t")
		if len(splitLine) != 11 {
			return []systemRoute{}, errors.Errorf("invalid /proc/net/route line '%s', expected 11 columns got %d", line, len(splitLine))
		}

		dest, err := hex.DecodeString(strings.TrimSpace(splitLine[1]))
		if err != nil {
			return []systemRoute{}, errors.Wrapf(err, "cannot parse dest column (index 1) for /proc/net/route line '%s'", line)
		}
		destStr := fmt.Sprintf("%d.%d.%d.%d", dest[3], dest[2], dest[1], dest[0])

		gw, err := hex.DecodeString(strings.TrimSpace(splitLine[2]))
		if err != nil {
			return []systemRoute{}, errors.Wrapf(err, "cannot parse gateway column (index 2) for /proc/net/route line '%s'", line)
		}
		gwStr := fmt.Sprintf("%d.%d.%d.%d", gw[3], gw[2], gw[1], gw[0])

		mask, err := hex.DecodeString(strings.TrimSpace(splitLine[7]))
		if err != nil {
			return []systemRoute{}, errors.Wrapf(err, "cannot parse mask column (index 7) for /proc/net/route line '%s'", line)
		}
		maskStr := fmt.Sprintf("%d.%d.%d.%d", mask[3], mask[2], mask[1], mask[0])
		maskBytes := []byte{}
		for _, v := range strings.Split(maskStr, ".") {
			maskByte, err := strconv.Atoi(v)
			if err != nil {
				return []systemRoute{}, errors.Wrapf(err, "cannot convert mask octet '%s' to byte", v)
			}
			maskBytes = append(maskBytes, byte(maskByte))
		}

		metric, err := strconv.Atoi(strings.TrimSpace(splitLine[6]))
		if err != nil {
			return []systemRoute{}, errors.Wrapf(err, "cannot parse metric column (index 6) for /proc/net/route line '%s'", line)
		}

		routes = append(routes, systemRoute{
			Iface: strings.TrimSpace(splitLine[0]),
			DestNet: net.IPNet{
				IP:   net.ParseIP(destStr),
				Mask: net.IPv4Mask(maskBytes[0], maskBytes[1], maskBytes[2], maskBytes[3]),
			},
			Gateway: net.ParseIP(gwStr),
			Metric:  uint32(metric),
		})
	}

	return routes, nil
}

// Credit: https://github.com/replicatedhq/kURL/blob/main/kurl_util/cmd/subnet/main.go findAvailableSubnet
// TODOLATER: consolidate some of this logic into a unified library? will need a bit of refactoring if so
//
// isASubnetAvailableInCIDR will check if a subnet of cidrRange size is available within subnetRange (IPv4 only)
func isASubnetAvailableInCIDR(cidrRange int, subnetRange *net.IPNet, routes *systemRoutes, debug bool) (bool, error) {
	forceV4 := len(subnetRange.IP) == net.IPv4len

	startIP, _ := cidr.AddressRange(subnetRange)

	_, subnet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", startIP, cidrRange))
	if err != nil {
		return false, errors.Wrap(err, "parse cidr")
	}
	if debug {
		fmt.Fprintf(os.Stderr, "Looking for a /%d within %s\n", cidrRange, subnetRange)
		fmt.Fprintf(os.Stderr, "First subnet to test %s\n", subnet)
	}

	for {
		firstIP, lastIP := cidr.AddressRange(subnet)
		if !subnetRange.Contains(firstIP) || !subnetRange.Contains(lastIP) {
			return false, errors.New("no available subnet found")
		}

		route := findFirstOverlappingRoute(subnet, routes)
		if route == nil {
			if debug {
				fmt.Fprintf(os.Stderr, "No overlapping routes found\n")
			}
			return true, nil
		}
		if forceV4 {
			// NOTE: this may break with v6 addresses
			if ip4 := route.DestNet.IP.To4(); ip4 != nil {
				route.DestNet.IP = ip4
			}
		}
		if debug {
			fmt.Fprintf(os.Stderr, "Route %s overlaps with subnet %s\n", &route.DestNet, subnet)
		}

		subnet, _ = cidr.NextSubnet(&route.DestNet, cidrRange)
		if debug {
			fmt.Fprintf(os.Stderr, "Next subnet %s\n", subnet)
		}
	}
}

// findFirstOverlappingRoute will return the first overlapping route with the subnet specified
func findFirstOverlappingRoute(subnet *net.IPNet, routes *systemRoutes) *systemRoute {
	defaultRoute := net.IPNet{
		IP:   net.IPv4(0, 0, 0, 0),
		Mask: net.CIDRMask(0, 32),
	}

	for _, route := range *routes {
		// Exclude default routes (0.0.0.0/0)
		if route.DestNet.IP.Equal(defaultRoute.IP) && route.DestNet.Mask.String() == defaultRoute.Mask.String() {
			continue
		}

		if netOverlaps(&route.DestNet, subnet) {
			return &route
		}
	}
	return nil
}

func netOverlaps(n1, n2 *net.IPNet) bool {
	return n1.Contains(n2.IP) || n2.Contains(n1.IP)
}
