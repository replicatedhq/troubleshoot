package cli

import (
	"fmt"
	"os"
	"path"
	"time"

	"github.com/replicatedhq/troubleshoot/internal/bundleimpl"
	"github.com/replicatedhq/troubleshoot/internal/util"
	"github.com/replicatedhq/troubleshoot/pkg/bundle"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/spf13/cobra"
)

func CollectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect bundle from a cluster or host",
		Long:  "Collect bundle from a cluster or host",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			// TODO: This logic must be functionally equivalent to CollectSupportBundleFromSpec

			// 1. Load troubleshoot specs from args
			// TODO: "RawSpecsFromArgs" missing the logic to load specs from the cluster
			rawSpecs, err := util.RawSpecsFromArgs(args)
			if err != nil {
				return err
			}
			kinds, err := loader.LoadSpecs(ctx, loader.LoadOptions{
				RawSpecs: rawSpecs,
			})
			if err != nil {
				return err
			}

			// 2. Collect the support bundle
			bundleDir, err := os.MkdirTemp("", "troubleshoot")
			if err != nil {
				return err
			}
			defer os.RemoveAll(bundleDir)

			bdl := bundleimpl.NewTroubleshootBundle()
			err = bdl.Collect(ctx, bundle.CollectOptions{
				Specs:     kinds,
				BundleDir: bundleDir,
			})
			if err != nil {
				return err
			}

			// 3. Analyze the support bundle
			// TODO: Add results to the support bundle
			_, err = bdl.Analyze(ctx, bundle.AnalyzeOptions{
				Specs:        kinds,
				PathInBundle: "analysis.json",
			})
			if err != nil {
				return err
			}

			// 4. Redact the support bundle
			err = bdl.Redact(ctx, bundle.RedactOptions{
				Specs: kinds,
			})
			if err != nil {
				return err
			}

			// 5. Archive the support bundle
			supportBundlePath := path.Join(util.HomeDir(), fmt.Sprintf("support-bundle-%s.tgz", time.Now().Format("2006-01-02T15_04_05")))
			err = bdl.Archive(ctx, bundle.ArchiveOptions{
				ArchivePath: supportBundlePath,
			})
			if err != nil {
				return err
			}

			// 6. Print outro i.e. "Support bundle saved to <filename>"
			// Print to screen output of bdl.Analyze i.e "analysisResults"
			fmt.Printf("Support bundle saved to %s\n", supportBundlePath)

			return nil
		},
	}
	return cmd
}
