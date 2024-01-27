package collect

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessEnv(t *testing.T) {
	pathEnv := fmt.Sprintf("%s=%s", "PATH", os.Getenv("PATH"))
	pwdEnv := fmt.Sprintf("%s=%s", "PWD", os.Getenv("PWD"))
	homeEnv := fmt.Sprintf("%s=%s", "HOME", os.Getenv("HOME"))
	os.Setenv("KUBECONFIG", "/some/kubeconfig")
	kubeconfigEnv := fmt.Sprintf("%s=%s", "KUBECONFIG", os.Getenv("KUBECONFIG"))
	tests := []struct {
		name      string
		collector *CollectHostRun
		parentEnv []string
		want      []string
	}{
		{
			name: "setting Env field",
			collector: &CollectHostRun{
				hostCollector: &troubleshootv1beta2.HostRun{
					Env: []string{
						"USER=dummy",
						"AWS_REGION=us-east-1",
					},
				},
				BundlePath: "",
			},
			want: append([]string{
				"USER=dummy",
				"AWS_REGION=us-east-1",
			}, os.Environ()...),
		},
		{
			name: "guaranteed env",
			collector: &CollectHostRun{
				hostCollector: &troubleshootv1beta2.HostRun{
					IgnoreParentEnvs: true,
				},
				BundlePath: "",
			},
			want: []string{pathEnv, pwdEnv, kubeconfigEnv},
		},
		{
			name: "ignoring parent env",
			collector: &CollectHostRun{
				hostCollector: &troubleshootv1beta2.HostRun{
					Env: []string{
						"USER=dummy",
						"AWS_REGION=us-east-1",
					},
					IgnoreParentEnvs: true,
					// inheritEnv will be ignored if IgnoreParentEnvs is true
					InheritEnvs: []string{
						"HOME",
					},
				},
				BundlePath: "",
			},
			want: append([]string{
				"USER=dummy",
				"AWS_REGION=us-east-1",
			}, pathEnv, pwdEnv, kubeconfigEnv),
		},
		{
			name: "inheriting a subset of parent env",
			collector: &CollectHostRun{
				hostCollector: &troubleshootv1beta2.HostRun{
					Env: []string{
						"AWS_REGION=us-east-1",
					},
					InheritEnvs: []string{
						"HOME",
					},
				},
				BundlePath: "",
			},
			want: append([]string{
				"AWS_REGION=us-east-1",
			}, pathEnv, pwdEnv, homeEnv, kubeconfigEnv),
		},
	}

	for _, test := range tests {
		cmd := exec.Command(test.collector.hostCollector.Command, test.collector.hostCollector.Args...)
		err := test.collector.processEnvVars(cmd)
		require.NoError(t, err)
		require.ElementsMatch(t, test.want, cmd.Env)
	}
}

func TestCollectHostRun_Collect(t *testing.T) {
	testDir, mkdirErr := os.MkdirTemp("", "host-run-test-*")
	defer os.RemoveAll(testDir)
	require.NoError(t, mkdirErr)
	tests := []struct {
		name      string
		collector *CollectHostRun
		parentEnv []string
		want      map[string][]byte
		wantError bool
	}{
		{
			name: "saving cmd run output",
			collector: &CollectHostRun{
				hostCollector: &troubleshootv1beta2.HostRun{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: "my-cmd-with-output",
					},
					Command: "sh",
					Args:    []string{"-c", "echo ${TS_INPUT_DIR}/dummy.conf > $TS_OUTPUT_DIR/input-file.txt"},
					Input: map[string]string{
						"dummy.conf": "[hello]\nhello = 1",
					},
					OutputDir: "magic-output",
				},
				BundlePath: path.Join(testDir, "bundle-1"),
			},
			want: CollectorResult{
				"host-collectors/run-host/my-cmd-with-output-info.json":                   nil,
				"host-collectors/run-host/my-cmd-with-output.txt":                         nil,
				"host-collectors/run-host/my-cmd-with-output/magic-output/input-file.txt": nil,
			},
		},
		{
			name: "invalid input filename",
			collector: &CollectHostRun{
				hostCollector: &troubleshootv1beta2.HostRun{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: "test",
					},
					Input: map[string]string{
						"/home/ec2-user/invalid-path.conf": "[hello]\nhello = 1",
						"valid-path.conf":                  "[env]\ndummy = dummy",
					},
				},
			},
			wantError: true,
		},
	}

	for _, test := range tests {
		got, err := test.collector.Collect(nil)
		if test.wantError {
			assert.Error(t, err)
		} else {
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		}
	}
}

func TestCollectHostRunCollectWithTimeout(t *testing.T) {
	tests := []struct {
		name      string
		collector *CollectHostRun
		wantError bool
	}{
		{
			name: "no timeout",
			collector: &CollectHostRun{
				hostCollector: &troubleshootv1beta2.HostRun{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: "no timeout",
					},
					Command: "echo",
					Args:    []string{"1"},
				},
			},
			wantError: false,
		},
		{
			name: "negative timeout",
			collector: &CollectHostRun{
				hostCollector: &troubleshootv1beta2.HostRun{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: "negative timeout",
					},
					Command: "echo",
					Args:    []string{"1"},
					Timeout: "-10s",
				},
			},
			wantError: false,
		},
		{
			name: "endless command with timeout",
			collector: &CollectHostRun{
				hostCollector: &troubleshootv1beta2.HostRun{
					HostCollectorMeta: troubleshootv1beta2.HostCollectorMeta{
						CollectorName: "endless command",
					},
					Command: "ping",
					Args:    []string{"google.com"},
					Timeout: "200ms",
				},
			},
			wantError: false,
		},
	}

	for _, test := range tests {
		got, err := test.collector.Collect(nil)
		if test.wantError {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.NotNil(t, got)
		}
	}
}
