// Is this naming convention correct? i.e impl is for implementation
package bundleimpl

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/bundle"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

// Bundle is the Troubleshoot implementation of the bundle.Bundler interface
type Bundle struct {
	data         *collect.BundleData
	progressChan chan any
}

type TroubleshootBundleOptions struct {
	// TODO: Consider just a callback function. Channels can block if not read from
	ProgressChan chan any // a channel to write progress information to
	Namespace    string   // namespace to limit scope the collectors need to run in
}

// NewTroubleshootBundle returns a new instance of the Troubleshoot bundle
func NewTroubleshootBundle(opt TroubleshootBundleOptions) bundle.Bundler {
	return &Bundle{
		progressChan: opt.ProgressChan,
	}
}

func (b *Bundle) Collect(ctx context.Context, opt bundle.CollectOptions) error {
	// TODO: Error if b.data is not nil. We do not want to overwrite existing data
	if b.data != nil {
		return fmt.Errorf("bundle already has data")
	}

	b.data = collect.NewBundleData(opt.BundleDir)

	results, err := b.doCollect(ctx, opt)
	if err != nil {
		return err
	}
	b.data.Data().AddResult(results)
	return nil
}

func (b *Bundle) Analyze(ctx context.Context, opt bundle.AnalyzeOptions) (bundle.AnalyzeOutput, error) {
	return bundle.AnalyzeOutput{}, nil
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
	// TODO: Error if b.data is not nil. We do not want to overwrite existing data
	return nil
}

func (b *Bundle) Serve(ctx context.Context, opt bundle.ServeOptions) error {
	return nil
}
