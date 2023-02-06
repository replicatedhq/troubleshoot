package traces

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

var (
	_        trace.SpanExporter = (*Exporter)(nil)
	once     sync.Once
	exporter *Exporter
	printer  = message.NewPrinter(language.English)
)

// FUTURE WORK: This exporter should only be used by troubleshoot CLIs
// until the following issue is addressed:
// 1. The cache of spans grows infinitely at the moment. This is OK for short lived
//    invocations such as CLI applications running one-shot commands. For long running
//    applications, this will be a problem.
// 2. At the moment, the summary of an execution is the only case this exporter
// 	  is being used for.

// GetExporterInstance creates a singleton exporter instance
func GetExporterInstance() *Exporter {
	once.Do(func() {
		exporter = &Exporter{
			allSpans: make([]trace.ReadOnlySpan, 1024),
		}
	})
	return exporter
}

// Exporter is an implementation of trace.SpanExporter that writes to a destination.
type Exporter struct {
	spansMu  sync.Mutex
	allSpans []trace.ReadOnlySpan

	stoppedMu sync.RWMutex
	stopped   bool
}

// ExportSpans writes spans to an in-memory cache
// This function can/will be called on every span.End() at worst.
func (e *Exporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	// This is a no-op if the context is canceled.
	if ctx.Err() != nil {
		return ctx.Err()
	}

	e.stoppedMu.RLock()
	stopped := e.stopped
	e.stoppedMu.RUnlock()
	if stopped {
		return nil
	}

	if len(spans) == 0 {
		return nil
	}

	e.spansMu.Lock()
	defer e.spansMu.Unlock()

	// Cache received span updates allSpans
	e.allSpans = append(e.allSpans, spans...)

	return nil
}

func isType(stub *tracetest.SpanStub, t string) bool {
	if stub == nil {
		return false
	}

	for _, attr := range stub.Attributes {
		if string(attr.Key) == "type" && strings.Contains(attr.Value.AsString(), t) {
			return true
		}
	}
	return false
}

// maxInt returns the larger of x or y.
func maxInt(x, y int) int {
	if x < y {
		return y
	}
	return x
}

// GetSummary returns the runtime summary of the execution
// so far. Call this function after your "root" span has ended
// and the program operations needing tracing have completed.
func (e *Exporter) GetSummary() string {
	e.spansMu.Lock()
	stubs := tracetest.SpanStubsFromReadOnlySpans(e.allSpans)
	e.spansMu.Unlock()

	// No spans to log
	if len(stubs) == 0 {
		return ""
	}

	// TODO: We may want to collect more information about the
	// execution of the program. For example, we can collect
	// the number of times a collector was executed, whether
	// it was successful or not, if it was skipped, etc.
	collectors := make(map[string]time.Duration)
	redactors := make(map[string]time.Duration)
	analysers := make(map[string]time.Duration)

	totalDuration := time.Duration(0)

	for i := range stubs {
		stub := &stubs[i]

		// Summary of span stubs
		duration := stub.EndTime.Sub(stub.StartTime)
		switch {
		case stub.Name == constants.TROUBLESHOOT_ROOT_SPAN_NAME:
			totalDuration = duration
		case isType(stub, "Collect"):
			collectors[stub.Name] = duration
		case isType(stub, "Redactors"):
			redactors[stub.Name] = duration
		case isType(stub, "Analyze"):
			analysers[stub.Name] = duration
		default:
			continue
		}
	}

	sb := strings.Builder{}

	collectorsSummary(collectors, &sb)
	redactorsSummary(redactors, &sb)
	analysersSummary(analysers, &sb)
	sb.WriteString(printer.Sprintf("\nDuration: %dms\n", totalDuration/time.Millisecond))

	return sb.String()
}

// summary of collector runtimes
func collectorsSummary(summary map[string]time.Duration, sb *strings.Builder) {
	padding, keys := sortedKeysAndPadding(summary)

	sb.WriteString("========= Collectors summary ==========\n")
	if len(summary) == 0 {
		sb.WriteString("No collectors executed\n")
		return
	}

	for _, name := range keys {
		sb.WriteString(printer.Sprintf("%-*s : %dms\n", padding, name, summary[name]/time.Millisecond))
	}
}

// summary of redactor runtime
func redactorsSummary(summary map[string]time.Duration, sb *strings.Builder) {
	padding, keys := sortedKeysAndPadding(summary)

	sb.WriteString("\n========= Redactors summary ==========\n")
	if len(summary) == 0 {
		sb.WriteString("No redactors executed\n")
		return
	}

	for _, name := range keys {
		sb.WriteString(printer.Sprintf("%-*s : %dms\n", padding, name, summary[name]/time.Millisecond))
	}
}

// summary of analyser runtime
func analysersSummary(summary map[string]time.Duration, sb *strings.Builder) {
	padding, keys := sortedKeysAndPadding(summary)

	sb.WriteString("\n========= Analysers summary ==========\n")
	if len(summary) == 0 {
		sb.WriteString("No analysers executed\n")
		return
	}

	for _, name := range keys {
		sb.WriteString(printer.Sprintf("%-*s : %dms\n", padding, name, summary[name]/time.Millisecond))
	}
}

func sortedKeysAndPadding(summary map[string]time.Duration) (int, []string) {
	keys := make([]string, 0, len(summary))
	padding := 0
	for k := range summary {
		padding = maxInt(padding, len(k))
		keys = append(keys, k)
	}
	sort.SliceStable(keys, func(l, r int) bool {
		return summary[keys[l]] > summary[keys[r]]
	})
	return padding, keys
}

// Shutdown is called to stop the exporter, it preforms no action.
func (e *Exporter) Shutdown(ctx context.Context) error {
	e.stoppedMu.Lock()
	e.stopped = true
	e.stoppedMu.Unlock()

	e.Reset()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

func (e *Exporter) Reset() {
	e.spansMu.Lock()
	e.allSpans = e.allSpans[:0] // clear the slice
	e.spansMu.Unlock()
}

// MarshalLog is the marshaling function used by the logging system to represent this exporter.
func (e *Exporter) MarshalLog() interface{} {
	return struct {
		Type string
	}{
		Type: "troubleshoot",
	}
}
