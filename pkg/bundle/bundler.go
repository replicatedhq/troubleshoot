package bundle

import (
	"context"

	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
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
	Collect(context.Context, CollectOptions) (CollectOutput error)

	// Analyze runs analysis defined in TroubleshootKinds passed through AnalyzeOptions
	Analyze(context.Context, AnalyzeOptions) (AnalyzeOutput, error)

	// Bundle returns collected or loaded bundle data
	// What should this return? CollectorResult? BundleData?
	// Use this to add data to the bundle that is not collected by collectors
	BundleData() *collect.BundleData

	// AnalysisResults contains the analysis results that the bundle may have had
	// TODO: I'm not yet about this. Maybe it should be in BundleData returned by Bundle()?
	// AnalysisResults() AnalysisResults

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
	Path      string // Path to archive or directory of bundle files
	BundleDir string // directory to drop bundle files to. TODO: We may not need this
}

type CollectOptions struct {
	Specs     *loader.TroubleshootKinds // list of specs to extract collectors and redactors from
	BundleDir string                    // directory to write bundle files to
	Namespace string                    // namespace to limit scope the in-cluster collectors need to run in
}

type CollectOutput struct {
	// Nothing for now. Left here for future use
}

type ArchiveOptions struct {
	ArchivePath string // path to write archive file to
}

type AnalyzeOutput struct {
	// TODO: Does this need to be a slice of pointers? It's a slice and slices are already pointers
	Results []*analyze.AnalyzeResult // analysis results
}

type RedactOptions struct {
	Specs *loader.TroubleshootKinds // list of specs to extract redactors from
}

// Note: this is almost identical to `CollectOptions` for now but remains separate to enable easier addition of redact specific options at a later date
type AnalyzeOptions struct {
	Specs *loader.TroubleshootKinds // list of specs to extract analyzers from
}

type ServeOptions struct {
	Address    string // address to listen on including port (0.0.0.0:8080)
	ConfigPath string // optional path to store generated kubeconfig
}
