package types

type NotFoundError struct {
	Name string
}

func (e *NotFoundError) Error() string {
	return e.Name + ": not found"
}

type ExitError interface {
	Error() string
	ExitStatus() int
}

type ExitCodeError struct {
	Msg  string
	Code int
}

func (e *ExitCodeError) Error() string {
	return e.Msg
}

func (e *ExitCodeError) ExitStatus() int {
	return e.Code
}

func NewExitCodeError(exitCode int, theErr error) *ExitCodeError {
	useErr := ""
	if theErr != nil {
		useErr = theErr.Error()
	}
	return &ExitCodeError{Msg: useErr, Code: exitCode}
}

// Bundler interface implements all functionality related to managing troubleshoot bundles
// Should be able to walk the directory, or should we?
// *  Finding files by path
// *  Finding files by glob
// *  Reading files
// *  Searching files by regex
type Bundler interface {
    // Collect runs collections defined in TroubleshootKinds passed through CollectOptions
    Collect(CollectOptions) error

    // We need to expose the bundle data collected in some form of structure as well
    // We have https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/collect/result.go#L15 at the moment

    // Analyze runs analysis defined in TroubleshootKinds passed through AnalyzeOptions
    Analyze(AnalyzeOptions) (AnalysisResults, error)

	// AnalysisResults contains the analysis results that the bundle may have had
	AnalysisResults() AnalysisResults

    // Redact runs redaction defined in TroubleshootKinds passed through RedactOptions
    Redact(RedactOptions) error

    // Archive produces an archive from a bundle on disk with options passed in ArchiveOptions
    Archive(ArchiveOptions) error

    // Load loads a bundle from a directory or archive served from disk or a remote location like a URL
    Load(LoadBundleOptions) error
    // it's worth noting that while this may appear inefficient now
    // it allows us to extend what we include in the Bundle struct in future without having to continuously extend the load function.

    // Serve starts an sbctl-like server with options defined in ServeOptions
    Serve(ServeOptions) error
}

type AnalysisResults struct {

}

type LoadBundleOptions struct {
    Path string // Path to archive or directory of bundle files
}

type CollectOptions struct {
    Specs *TroubleshootKinds // list of specs to extract collectors and redactors from
    ProgressChan chan // a channel to write progress information to
}

type RedactOptions struct {
    Specs *TroubleshootKinds // list of specs to extract redactors from
    ProgressChan chan // a channel to write progress information to
}
// Note: this is almost identical to `CollectOptions` for now but remains separate to enable easier addition of redact specific options at a later date

type AnalyzeOptions struct {
    ProgressChan chan // a channel to write progress information to
}

type ServeOptions struct {
    Address string // address to listen on including port (0.0.0.0:8080)
    ConfigPath string // optional path to store generated kubeconfig
}

type AnalysisResults struct {
    Bundle Bundle // include bundle metadata
    // and whatever resulsts for the analysis
}

// These interfaces already exist in some form. We would need to review them

// https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/collect/host_collector.go#L7
type HostCollector interface {
    Title() string
    IsExcluded() (bool, error)
    ObjectTyper

    // Belongs in the Bundler interface
    // Collect(progressChan chan<- interface{}) (map[string][]byte, error)
}

// https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/collect/collector.go#L18
type Collector interface {
    Title() string
    IsExcluded() (bool, error)
    ObjectTyper

    // This belong elsewhere and, if need be, should be composed (golang's interface composition) into this interface. Retaining them for review
    // GetRBACErrors() []error
    // HasRBACErrors() bool
    // CheckRBAC(ctx context.Context, c Collector, collector *troubleshootv1beta2.Collect, clientConfig *rest.Config, namespace string) error

    // Belongs in the Bundler interface
    // Collect(progressChan chan<- interface{}) (CollectorResult, error)
}

// https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/analyze/analyzer.go#LL173C1-L177C2
type Analyzer interface {
    Title() string
    IsExcluded() (bool, error)
    ObjectTyper

    // Belongs in the Bundler interface
    // Analyze(getFile getCollectedFileContents, findFiles getChildCollectedFileContents) ([]*AnalyzeResult, error)
}

// https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/analyze/host_analyzer.go#L5
type HostAnalyzer interface {
    Title() string
    IsExcluded() (bool, error)
    ObjectTyper

    // Belongs in the Bundler interface
    Analyze(getFile func(string) ([]byte, error)) ([]*AnalyzeResult, error)
}

// https://github.com/replicatedhq/troubleshoot/blob/620fa75eb593247a07c4dc39ea96fc6a059be111/pkg/redact/redact.go#LL31C1-L33C2
type Redactor interface {
    Redact(input io.Reader, path string) io.Reader
    ObjectTyper
}

// ObjectTyper interface exposes an internal object and its type for a consumer of an interface to get the concrete type implementing that interface
// This interface is meant for collectors, analysers and redactors so as to get back the object created from a spec. It should not be used with other
// interfaces that have internal implementation that's bound to change such as Bundler.
// TODO: Is this the best name for this? I'm just following go's recommendation - https://go.dev/doc/effective_go#interface-names
type ObjectTyper interface {
    // TODO: Is there a simpler way?
    // TODO: Maybe we should limit this interface usage to collectors/analysers/redactor. Call it KindCaster? ObjectKinder?? SpecTyper??
    Object() interface{}  // typeless object that was created e.g troubleshootv1beta2.Ceph collector, or redact.MultiLineRedactor redactor
    Type() string // type information that can be used to cast the object back to its original concrete implementation. e.g troubleshootv1beta2.Ceph
                  // NOTE: The concrete type exposed here needs to be a public type
}