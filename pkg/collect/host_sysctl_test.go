package collect

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"github.com/stretchr/testify/require"
)

func setKernelVirtualFilesPath(path string) {
	sysctlVirtualFiles = path
}

func TestCollectHostSysctl_Error(t *testing.T) {
	req := require.New(t)
	tmpDir := t.TempDir()

	setKernelVirtualFilesPath(fmt.Sprintf("%s/does/not/exist", tmpDir))

	c := &CollectHostSysctl{
		BundlePath: tmpDir,
	}

	_, err := c.Collect(nil)
	req.ErrorContains(err, "failed to initialize sysctl client")
}

func TestCollectHostSysctl(t *testing.T) {
	req := require.New(t)
	expectedOut := map[string]string{
		"net.ipv4.conf.all.arp_ignore":          "0",
		"net.ipv4.conf.all.arp_filter":          "1",
		"net.ipv4.conf.all.arp_evict_nocarrier": "1",
	}

	tmpDir := t.TempDir()
	virtualFilesPath := fmt.Sprintf("%s/proc/sys/", tmpDir)
	ipv4All := fmt.Sprintf("%s/net/ipv4/conf/all", virtualFilesPath)

	setKernelVirtualFilesPath(virtualFilesPath)
	err := os.MkdirAll(ipv4All, 0777)
	req.NoError(err)

	err = os.WriteFile(fmt.Sprintf("%s/arp_ignore", ipv4All), []byte("0"), 0777)
	req.NoError(err)
	err = os.WriteFile(fmt.Sprintf("%s/arp_filter", ipv4All), []byte("1"), 0777)
	req.NoError(err)
	err = os.WriteFile(fmt.Sprintf("%s/arp_evict_nocarrier", ipv4All), []byte("1"), 0777)
	req.NoError(err)

	c := &CollectHostSysctl{
		BundlePath: tmpDir,
	}

	out, err := c.Collect(nil)
	req.NoError(err)
	res := CollectorResult(out)
	reader, err := res.GetReader(tmpDir, HostSysctlPath)
	req.NoError(err)

	parameters := map[string]string{}
	err = json.NewDecoder(reader).Decode(&parameters)
	req.NoError(err)

	req.Equal(parameters, expectedOut)
}

func TestCollectHostSysctl_Title(t *testing.T) {
	req := require.New(t)

	// Default title is set
	c := &CollectHostSysctl{
		hostCollector: &troubleshootv1beta2.HostSysctl{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{},
		},
	}
	req.Equal("Sysctl", c.Title())

	// Configured title is set
	c = &CollectHostSysctl{
		hostCollector: &troubleshootv1beta2.HostSysctl{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				CollectorName: "foobar",
			},
		},
	}
	req.Equal("foobar", c.Title())
}

func TestCollectHostSysctl_IsExcluded(t *testing.T) {
	req := require.New(t)

	// Exclude is true
	c := &CollectHostSysctl{
		hostCollector: &troubleshootv1beta2.HostSysctl{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				Exclude: multitype.FromBool(true),
			},
		},
	}
	isExcluded, err := c.IsExcluded()
	req.NoError(err)
	req.Equal(true, isExcluded)

	// Exclude is false
	c = &CollectHostSysctl{
		hostCollector: &troubleshootv1beta2.HostSysctl{
			HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
				Exclude: multitype.FromBool(false),
			},
		},
	}
	isExcluded, err = c.IsExcluded()
	req.NoError(err)
	req.Equal(false, isExcluded)
}
