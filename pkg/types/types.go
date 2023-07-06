package types

import (
	"io"
)

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
// type HostAnalyzer interface {
//     Title() string
//     IsExcluded() (bool, error)
//     ObjectTyper

//     // Belongs in the Bundler interface
//     Analyze(getFile func(string) ([]byte, error)) ([]*AnalyzeResult, error)
// }

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
	Object() interface{} // typeless object that was created e.g troubleshootv1beta2.Ceph collector, or redact.MultiLineRedactor redactor
	Type() string        // type information that can be used to cast the object back to its original concrete implementation. e.g troubleshootv1beta2.Ceph
	// NOTE: The concrete type exposed here needs to be a public type
}
