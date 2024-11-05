package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestHostRemoteCollector(t *testing.T) {
	feature := features.New("Host OS Remote Collector Test").
		Assess("run support bundle command successfully", func(ctx context.Context, t *testing.T, c *envconf.Config) context.Context {
			var out bytes.Buffer
			supportbundleName := "host-os-remote-collector"
			cmd := exec.CommandContext(ctx, sbBinary(), "spec/remoteHostCollectors.yaml", fmt.Sprintf("-o=%s", supportbundleName))
			cmd.Stdout = &out
			err := cmd.Run()
			if err != nil {
				fmt.Println(out.String())
				t.Fatalf("Failed to run the binary: %v", err)
			}

			defer func() {
				err := os.Remove(fmt.Sprintf("%s.tar.gz", supportbundleName))
				if err != nil {
					t.Fatalf("Error removing file: %v", err)
				}
			}()

			// At this point, we only care that the binary ran successfully, no need to check folder contents.
			t.Logf("Binary executed successfully: %s", out.String())

			return ctx
		}).Feature()

	testenv.Test(t, feature)
}
