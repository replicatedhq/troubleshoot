package bundle

import (
	"context"

	"github.com/replicatedhq/troubleshoot/pkg/loader"
)

// Bundler interface implements all functionality related to managing troubleshoot bundles
// TODO: Should be able to walk the directory, or should we?
// *  Finding files by path
// *  Finding files by glob
// *  Reading files
// *  Searching files by regex
type Bundler interface {
	// Collect runs collections defined in TroubleshootKinds passed through CollectOptions
	Collect(context.Context, CollectOptions) error

	// We need to expose the bundle data collected in some form of structure as well
	// We have https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/collect/result.go#L15 at the moment

	// Analyze runs analysis defined in TroubleshootKinds passed through AnalyzeOptions
	Analyze(context.Context, AnalyzeOptions) (AnalysisResults, error)

	// AnalysisResults contains the analysis results that the bundle may have had
	// TODO: I'm not yet about this
	AnalysisResults() AnalysisResults

	// Redact runs redaction defined in TroubleshootKinds passed through RedactOptions
	Redact(context.Context, RedactOptions) error

	// Archive produces an archive from a bundle on disk with options passed in ArchiveOptions
	Archive(context.Context, ArchiveOptions) error

	// Load loads a bundle from a directory or archive served from disk or a remote location like a URL
	Load(context.Context, LoadBundleOptions) error
	// it's worth noting that while this may appear inefficient now
	// it allows us to extend what we include in the Bundle struct in future without having to continuously extend the load function.

	// Serve starts an sbctl-like server with options defined in ServeOptions
	Serve(context.Context, ServeOptions) error
}

type LoadBundleOptions struct {
	Path string // Path to archive or directory of bundle files
}

type CollectOptions struct {
	Specs        *loader.TroubleshootKinds // list of specs to extract collectors and redactors from
	BundleDir    string                    // directory to write bundle files to
	ProgressChan chan any                  // a channel to write progress information to
}

type ArchiveOptions struct {
	ArchivePath  string   // path to archive file to write
	ProgressChan chan any // a channel to write progress information to
}

type RedactOptions struct {
	Specs        *loader.TroubleshootKinds // list of specs to extract redactors from
	ProgressChan chan any                  // a channel to write progress information to
}

// Note: this is almost identical to `CollectOptions` for now but remains separate to enable easier addition of redact specific options at a later date

type AnalyzeOptions struct {
	Specs        *loader.TroubleshootKinds // list of specs to extract analyzers from
	PathInBundle string                    // path to store results in the bundle
	ProgressChan chan any                  // a channel to write progress information to
}

type ServeOptions struct {
	Address    string // address to listen on including port (0.0.0.0:8080)
	ConfigPath string // optional path to store generated kubeconfig
}

type AnalysisResults struct {
	Bundle Bundler // include bundle metadata
	// and whatever resulsts for the analysis
}
