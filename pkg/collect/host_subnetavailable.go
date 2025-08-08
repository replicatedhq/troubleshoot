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
	"k8s.io/klog/v2"
)

type SubnetStatus string

const (
	SubnetStatusAvailable     = "a-subnet-is-available"
	SubnetStatusNoneAvailable = "no-subnet-available"
)

type SubnetAvailableResult struct {
	CIDRRangeAlloc string `json:"CIDRRangeAlloc"`
	DesiredCIDR    int    `json:"desiredCIDR"`
	// If subnet-available, at least 1 of the DesiredCIDR size is available within CIDRRangeAlloc
	Status SubnetStatus `json:"status"`
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

func (c *CollectHostSubnetAvailable) SkipRedaction() bool {
	return c.hostCollector.SkipRedaction
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
	klog.V(3).Infof("Routes: %+v\n", routes)

	// IPv4 only right now...
	if c.hostCollector.DesiredCIDR < 1 || c.hostCollector.DesiredCIDR > 32 {
		return nil, errors.Wrap(err, fmt.Sprintf("CIDR range size %d invalid, must be between 1 and 32", c.hostCollector.DesiredCIDR))
	}

	splitCIDRRangeAlloc := strings.Split(c.hostCollector.CIDRRangeAlloc, "/")
	if len(splitCIDRRangeAlloc) != 2 {
		return nil, errors.Wrap(err, fmt.Sprintf("CIDRRangeAlloc value %s invalid, expected format x.x.x.x/##", c.hostCollector.CIDRRangeAlloc))
	}
	maskInt, err := strconv.Atoi(splitCIDRRangeAlloc[1])
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("CIDRRangeAlloc mask %s invalid, expected integer", splitCIDRRangeAlloc[1]))
	}
	if maskInt < 0 || maskInt > 32 {
		return nil, errors.Wrap(err, fmt.Sprintf("CIDRRangeAlloc mask %d invalid, must be between 0 and 32", maskInt))
	}
	cidrRangeAllocIPNet := net.IPNet{
		IP:   net.ParseIP(splitCIDRRangeAlloc[0]),
		Mask: net.CIDRMask(maskInt, 32),
	}

	result := SubnetAvailableResult{}
	result.CIDRRangeAlloc = c.hostCollector.CIDRRangeAlloc
	result.DesiredCIDR = c.hostCollector.DesiredCIDR
	available, err := isASubnetAvailableInCIDR(c.hostCollector.DesiredCIDR, &cidrRangeAllocIPNet, &routes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine if desired CIDR is available within subnet")
	}
	if available == true {
		result.Status = SubnetStatusAvailable
	} else {
		result.Status = SubnetStatusNoneAvailable
	}

	b, err := json.Marshal(result)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal result")
	}

	collectorName := c.hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "result"
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
		if len(line) < 4 {
			// Likely a blank line?
			continue
		}
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
// isASubnetAvailableInCIDR will check if a subnet of cidrRange size is available within subnetRange (IPv4 only), checking against system routes for conflicts
func isASubnetAvailableInCIDR(cidrRange int, subnetRange *net.IPNet, routes *systemRoutes) (bool, error) {
	// Sanity check that the CIDR range size is IPv4 valid (/0 to /32)
	if cidrRange < 1 || cidrRange > 32 {
		return false, errors.New(fmt.Sprintf("CIDR range size %d invalid, must be between 1 and 32", cidrRange))
	}

	// Check that cidrRange is equal to or smaller than the subnet size of subnetRange
	// Always exit false if not...
	subnetRangeSize, subnetRangeSizeBits := subnetRange.Mask.Size()
	if subnetRangeSizeBits != 32 {
		return false, errors.New(fmt.Sprintf("subnetRange size is not IPv4 compatible? expected 32 got %d", subnetRangeSizeBits))
	}
	// NOTE: reversed operator as we're talking about CIDR blocks (smaller integer = more IPs)
	if subnetRangeSize > cidrRange {
		return false, errors.New(fmt.Sprintf("subnetRange size (%d) must be larger than or equal to cidrRange size (%d), can't check if a range larger than itself is available", subnetRangeSize, cidrRange))
	}

	// Find the start IP of subnetRange, this will become the first subnet to be tested (with cidrRange as the size)
	startIP, _ := cidr.AddressRange(subnetRange)

	_, subnet, err := net.ParseCIDR(fmt.Sprintf("%s/%d", startIP, cidrRange))
	if err != nil {
		return false, errors.Wrap(err, "parse cidr")
	}

	for {
		// This is the extents of the subnet to be tested
		firstIP, lastIP := cidr.AddressRange(subnet)
		klog.V(2).Infof("Checking subnet: %s firstIP: %s lastIP: %s\n", subnet.String(), firstIP, lastIP)

		// Make sure the smaller subnet is (still) within the large subnetRange. If subnetRange has been exhausted one of these will be false
		if !subnetRange.Contains(firstIP) || !subnetRange.Contains(lastIP) {
			return false, nil
		}

		// Check if any system routes overlap with the smaller subnet being tested
		route := findFirstOverlappingRoute(subnet, routes)
		if route == nil {
			// No system routes match, this (smaller) subnet is available
			klog.V(1).Infof("Subnet %s is available", subnet.String())
			return true, nil
		}

		// Try the next subnet in the range
		subnet, _ = cidr.NextSubnet(subnet, cidrRange)
	}
}

// findFirstOverlappingRoute will return the first overlapping route with the subnet specified
func findFirstOverlappingRoute(subnet *net.IPNet, routes *systemRoutes) *systemRoute {
	// NOTE: IPv4 specific
	defaultRoute := net.IPNet{
		IP:   net.IPv4(0, 0, 0, 0),
		Mask: net.CIDRMask(0, 32),
	}

	for _, route := range *routes {
		// Exclude default routes (0.0.0.0/0)
		if route.DestNet.IP.Equal(defaultRoute.IP) && route.DestNet.Mask.String() == defaultRoute.Mask.String() {
			continue
		}

		klog.V(2).Infof("Checking if route %s overlaps with subnet %s - ", &route.DestNet, subnet)
		// TODOLATER: can we use cidr.VerifyNoOverlap to replace this? tests fail right now trying to do so...
		//if cidr.VerifyNoOverlap([]*net.IPNet{subnet}, &route.DestNet) != nil {
		if netOverlaps(&route.DestNet, subnet) {
			klog.V(2).Infof("Overlaps\n")
			return &route
		} else {
			klog.V(2).Infof("No overlap\n")
		}
	}
	klog.V(2).Infof("Subnet %s has no overlap with any system routes\n", subnet)
	return nil
}

func netOverlaps(n1, n2 *net.IPNet) bool {
	n1FirstIP, n1LastIP := cidr.AddressRange(n1)
	n2FirstIP, n2LastIP := cidr.AddressRange(n2)

	// Check if the first net contains either the first or last IP of the second net
	if n1.Contains(n2FirstIP) || n1.Contains(n2LastIP) {
		return true
	}
	// Now do the reverse: check if the second net contains either the first or last IP of the first net
	if n2.Contains(n1FirstIP) || n2.Contains(n1LastIP) {
		return true
	}

	return false
}

func (c *CollectHostSubnetAvailable) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
