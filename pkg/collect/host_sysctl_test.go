package collect

import (
	"io"
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

func TestCollectHostSysctl(t *testing.T) {
	req := require.New(t)
	cmdOut := `
		net.ipv4.conf.all.arp_evict_nocarrier = 1
		net.ipv4.conf.all.arp_filter = 0
		net.ipv4.conf.all.arp_ignore = 0
	`
	setExecStub(exec.Command("echo", "-n", cmdOut))

	tmpDir := t.TempDir()
	c := &CollectHostSysctl{
		BundlePath: tmpDir,
	}

	out, err := c.Collect(nil)
	req.NoError(err)
	res := CollectorResult(out)
	reader, err := res.GetReader(tmpDir, HostSysctlPath)
	req.NoError(err)
	actualOut, err := io.ReadAll(reader)
	req.NoError(err)
	req.Equal(string(actualOut), cmdOut)
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
