// Is this naming convention correct? i.e impl is for implementation
package bundleimpl

import (
	"context"

	"github.com/replicatedhq/troubleshoot/pkg/bundle"
)

// Bundle is the Troubleshoot implementation of the bundle.Bundler interface
type Bundle struct{}

// NewTroubleshootBundle returns a new instance of the Troubleshoot bundle
func NewTroubleshootBundle() bundle.Bundler {
	return &Bundle{}
}

func (b *Bundle) Collect(ctx context.Context, opt bundle.CollectOptions) error {
	return nil
}

func (b *Bundle) Analyze(ctx context.Context, opt bundle.AnalyzeOptions) (bundle.AnalysisResults, error) {
	return bundle.AnalysisResults{}, nil
}

func (b *Bundle) AnalysisResults() bundle.AnalysisResults {
	return bundle.AnalysisResults{}
}

func (b *Bundle) Redact(ctx context.Context, opt bundle.RedactOptions) error {
	return nil
}

func (b *Bundle) Archive(ctx context.Context, opt bundle.ArchiveOptions) error {
	return nil
}

func (b *Bundle) Load(ctx context.Context, opt bundle.LoadBundleOptions) error {
	return nil
}

func (b *Bundle) Serve(ctx context.Context, opt bundle.ServeOptions) error {
	return nil
}
