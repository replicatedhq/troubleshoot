package tsbundle

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/bundle"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/supportbundle"
)

// Bundle is the Troubleshoot implementation of the bundle.Bundler interface
type Bundle struct {
	data         *collect.BundleData
	progressChan chan any
}

type TroubleshootBundleOptions struct {
	ProgressChan chan any // a channel to write progress information to
	Namespace    string   // namespace to limit scope the collectors need to run in
}

// NewTroubleshootBundle returns a new instance of the Troubleshoot bundle
func NewTroubleshootBundle(opt TroubleshootBundleOptions) bundle.Bundler {
	return &Bundle{
		progressChan: opt.ProgressChan,
	}
}

func (b *Bundle) reportProgress(msg any) {
	if b.progressChan != nil {
		// Non-blocking write to channel.
		// In case there is no listener drop the message.
		select {
		case b.progressChan <- msg:
		default:
		}
	}
}

func (b *Bundle) Collect(ctx context.Context, opt bundle.CollectOptions) error {
	if b.data != nil {
		return fmt.Errorf("we cannot run collectors if a bundle already exists")
	}

	results, err := b.doCollect(ctx, opt)
	if err != nil {
		return err
	}

	b.data = collect.NewBundleData(collect.BundleDataOptions{
		Data:      results,
		BundleDir: opt.BundleDir,
	})
	return nil
}

func (b *Bundle) Analyze(ctx context.Context, opt bundle.AnalyzeOptions) (bundle.AnalyzeOutput, error) {
	if b.data == nil {
		return bundle.AnalyzeOutput{}, errors.New("no bundle data to analyze")
	}

	sbSpec := supportbundle.ConcatSpecs(opt.Specs.SupportBundlesV1Beta2...)

	// Run Analyzers
	analyzeResults, err := supportbundle.AnalyzeSupportBundle(ctx, &sbSpec.Spec, b.data.BundleDir())
	if err != nil {
		return bundle.AnalyzeOutput{}, err
	}

	return bundle.AnalyzeOutput{
		Results: analyzeResults,
	}, nil
}

func (b *Bundle) BundleData() *collect.BundleData {
	return b.data
}

func (b *Bundle) Redact(ctx context.Context, opt bundle.RedactOptions) error {
	return nil
}

func (b *Bundle) Archive(ctx context.Context, opt bundle.ArchiveOptions) error {
	err := b.data.Data().ArchiveSupportBundle(b.data.BundleDir(), opt.ArchivePath)
	if err != nil {
		return errors.Wrap(err, "failed to create bundle archive")
	}
	return nil
}

func (b *Bundle) Load(ctx context.Context, opt bundle.LoadBundleOptions) error {
	if b.data != nil {
		return fmt.Errorf("we cannot run collectors if a bundle already exists")
	}
	// TODO: Load bundle from disk or url
	return nil
}

func (b *Bundle) Serve(ctx context.Context, opt bundle.ServeOptions) error {
	return nil
}
