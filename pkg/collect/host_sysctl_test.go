package collect

import (
	"encoding/json"
	"os/exec"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/multitype"
	"github.com/stretchr/testify/require"
)

type execStub struct {
	cmd  *exec.Cmd
	name string
	args []string
}

func (s *execStub) testExecCommand(name string, args ...string) *exec.Cmd {
	s.name = name
	s.args = args
	return s.cmd
}

func setExecStub(c *exec.Cmd) {
	e := &execStub{
		cmd: c,
	}
	execCommand = e.testExecCommand
}

func TestCollectHostSysctl_Error(t *testing.T) {
	req := require.New(t)
	setExecStub(exec.Command("sh", "-c", "exit 1"))

	tmpDir := t.TempDir()
	c := &CollectHostSysctl{
		BundlePath: tmpDir,
	}

	_, err := c.Collect(nil)
	req.ErrorContains(err, "failed to run sysctl exit-code=1")
}

func TestCollectHostSysctl_(t *testing.T) {
	tests := []struct {
		name     string
		cmdOut   string
		expected map[string]string
	}{
		{
			name: "linux",
			cmdOut: `
				net.ipv4.conf.all.arp_evict_nocarrier = 1
				net.ipv4.conf.all.arp_filter = 0
				net.ipv4.conf.all.arp_ignore = 0
			`,
			expected: map[string]string{
				"net.ipv4.conf.all.arp_evict_nocarrier": "1",
				"net.ipv4.conf.all.arp_filter":          "0",
				"net.ipv4.conf.all.arp_ignore":          "0",
			},
		},
		{
			name: "darwin",
			cmdOut: `
				kern.prng.pool_31.max_sample_count: 16420665
				kern.crypto.sha1: SHA1_VNG_ARM
				kern.crypto.sha512: SHA512_VNG_ARM_HW
				kern.crypto.aes.ecb.encrypt: AES_ECB_ARM
				kern.monotonicclock: 4726514
				kern.monotonicclock_usecs: 4726514658233 13321990885027
			`,
			expected: map[string]string{
				"kern.prng.pool_31.max_sample_count": "16420665",
				"kern.crypto.sha1":                   "SHA1_VNG_ARM",
				"kern.crypto.sha512":                 "SHA512_VNG_ARM_HW",
				"kern.crypto.aes.ecb.encrypt":        "AES_ECB_ARM",
				"kern.monotonicclock":                "4726514",
				"kern.monotonicclock_usecs":          "4726514658233 13321990885027",
			},
		},
		{
			name: "skip non valid entries and keep empty values",
			cmdOut: `
				net.ipv4.conf.all.arp_ignore = 
				kern.prng.pool_31.max_sample_count:
				not-valid
				net.ipv4.conf.all.arp_filter = 0
			`,
			expected: map[string]string{
				"net.ipv4.conf.all.arp_ignore":       "",
				"kern.prng.pool_31.max_sample_count": "",
				"net.ipv4.conf.all.arp_filter":       "0",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := require.New(t)

			setExecStub(exec.Command("echo", "-n", test.cmdOut)) // #nosec G204

			tmpDir := t.TempDir()
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

			req.Equal(test.expected, parameters)
		})
	}
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
