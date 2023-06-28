package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	"github.com/replicatedhq/troubleshoot/internal/tsbundle"
	"github.com/replicatedhq/troubleshoot/internal/util"
	"github.com/replicatedhq/troubleshoot/pkg/bundle"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel"
	"k8s.io/klog/v2"
)

func CollectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect bundle from a cluster or host",
		Long:  "Collect bundle from a cluster or host",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := doRun(cmd.Context(), args)
			if err != nil {
				klog.Errorf("Failure collecting support bundle: %v", err)
				return err
			}

			return nil
		},
	}

	// Initialize klog flags
	// TODO: Make these flags global i.e RootCmd.PersistentFlags()
	logger.InitKlogFlags(cmd)

	return cmd
}

func doRun(ctx context.Context, args []string) error {
	// TODO: This logic must be functionally equivalent to CollectSupportBundleFromSpec

	// Boilerplate to collect progress information
	var wg sync.WaitGroup
	progressChan := make(chan interface{})
	defer func() {
		close(progressChan)
		wg.Wait()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for msg := range progressChan {
			// TODO: Expect error or string types
			klog.Infof("Collecting bundle: %v", msg)
		}
	}()
	ctxWrap, root := otel.Tracer(constants.LIB_TRACER_NAME).Start(
		ctx, constants.TROUBLESHOOT_ROOT_SPAN_NAME,
	)
	defer root.End()

	// 1. Load troubleshoot specs from args
	// TODO: "RawSpecsFromArgs" missing the logic to load specs from the cluster
	rawSpecs, err := util.RawSpecsFromArgs(args)
	if err != nil {
		return err
	}
	kinds, err := loader.LoadSpecs(ctxWrap, loader.LoadOptions{
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

	bdl := tsbundle.NewTroubleshootBundle(tsbundle.TroubleshootBundleOptions{
		ProgressChan: progressChan,
	})
	klog.Infof("Collect support bundle")
	err = bdl.Collect(ctxWrap, bundle.CollectOptions{
		Specs:     kinds,
		BundleDir: bundleDir,
	})
	if err != nil {
		return err
	}

	// 3. Analyze the support bundle
	// TODO: Add results to the support bundle
	klog.Infof("Analyse support bundle")
	out, err := bdl.Analyze(ctxWrap, bundle.AnalyzeOptions{
		Specs: kinds,
	})
	if err != nil {
		return err
	}
	// Save the analysis results to the bundle. Do it here so as not to redact
	// TODO: Perhaps the result should already be marshalled to JSON
	// i.e out.ResultsJSON propert or a function like out.ResultsJSON()
	analysis, err := out.ResultsJSON()
	if err != nil {
		return err
	}
	err = bdl.BundleData().Data().SaveResult(bundleDir, supportbundle.AnalysisFilename, bytes.NewBuffer(analysis))
	if err != nil {
		return err
	}

	// 4. Redact the support bundle
	klog.Infof("Redact support bundle")
	err = bdl.Redact(ctxWrap, bundle.RedactOptions{
		Specs: kinds,
	})
	if err != nil {
		return err
	}

	// 5. Archive the support bundle
	klog.Infof("Archive support bundle")
	supportBundlePath := path.Join(util.HomeDir(), fmt.Sprintf("support-bundle-%s.tgz", time.Now().Format("2006-01-02T15_04_05")))
	err = bdl.Archive(ctxWrap, bundle.ArchiveOptions{
		ArchivePath: supportBundlePath,
	})
	if err != nil {
		return err
	}

	// 6. Save the bundle to a file
	klog.Infof("Save version info to support bundle")
	reader, err := supportbundle.GetVersionFile()
	if err != nil {
		return err
	}
	err = bdl.BundleData().Data().SaveResult(bundleDir, constants.VERSION_FILENAME, reader)
	if err != nil {
		return err
	}

	// 7. Print outro i.e. "Support bundle saved to <filename>"
	// Print to screen output of bdl.Analyze i.e "analysisResults"
	fmt.Printf("Support bundle saved to %s\n", supportBundlePath)

	return nil
}
