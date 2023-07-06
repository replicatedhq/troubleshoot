package bundle

import (
	"context"
	"io"

	analyze "github.com/replicatedhq/troubleshoot/pkg/analyze"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
	"github.com/replicatedhq/troubleshoot/pkg/loader"
)

// Bundler interface defines the API for managing troubleshoot bundles
type Bundler interface {
	// Collect runs collectors defined in TroubleshootKinds
	Collect(context.Context, CollectOptions) (CollectOutput error)

	// Analyze runs analysers defined in TroubleshootKinds
	// TODO: Consider making a new interface type for this function.
	// It does not modify the bundle. BundleAnalyzer?
	Analyze(context.Context, AnalyzeOptions) (AnalyzeOutput, error)

	// Bundle returns collected or loaded bundle data
	BundleData() *collect.BundleData

	// Reset resets the bundle data clearing all data collected or loaded
	// The bundle instance can be reused after reset
	Reset()

	// Redact runs redactors defined in TroubleshootKinds
	// It modifies the bundle data in place
	// TODO: Do we want to report what was redacted in the output, or is progress/log messaging enough?
	Redact(context.Context, RedactOptions) error

	// Archive produces an archive from a bundle and writes it to the provided output (stream or file)
	Archive(context.Context, ArchiveOptions) error

	// Load loads a bundle from source (stream, archive, directory or url)
	Load(context.Context, LoadBundleOptions) error
}

// APIServer interface defines the API for implementing an API server of a read-only kubernetes cluster
// from a bundle containing cluster resources collected from a live cluster
type APIServer interface {
	// Serve starts kubernetes API server to serve a read-only kubernetes cluster
	// TODO: Should we return a channel to stream progress? Logs?
	// TODO: Should this be a blocking call? Or have a ListenAndServe method?
	Serve(context.Context, ServeOptions) (ServeOutput, error)
}

type LoadBundleOptions struct {
	Path      string    // Path to archive or directory of bundle files
	BundleDir string    // directory to drop bundle files to. TODO: We may not need this
	Stream    io.Reader // stream to read archive file from. Takes precedence over Path
}

type CollectOptions struct {
	Specs     *loader.TroubleshootKinds // list of specs to extract collectors and redactors from
	BundleDir string                    // directory to write bundle files to. The bundle directory will
	// not be deleted when the bundle is reset.
	Namespace string // namespace to limit scope the in-cluster collectors need to run in
}

type CollectOutput struct {
	// Nothing for now. Left here for future use
}

type ArchiveOptions struct {
	ArchivePath string    // path to write archive file to
	Stream      io.Writer // stream to write archive file to. Takes precedence over ArchivePath
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

// ServeOptions defines options for serving a read-only kubernetes cluster
type ServeOptions struct {
	BundleData collect.BundleData // bundle data containing cluster resources to serve
	Address    string             // address to listen on including port (0.0.0.0:8080)
	ConfigPath string             // optional path to store generated kubeconfig
	// TODO: How do we stop the server? Signals? Context (in API call)? Explicit channel?
	StopCh chan struct{} // channel to stop the server ???
}

type ServeOutput struct {
	Kubeconfig string // generated kubeconfig to access the read-only cluster
}
