package traces

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestExporter_LogSummaryAndReset(t *testing.T) {
	logger.SetQuiet(true)

	tests := []struct {
		name  string
		spans tracetest.SpanStubs
		want  string
	}{
		{
			name:  "with no spans",
			spans: tracetest.SpanStubs{},
			want:  "",
		},
		{
			name: "with root span only",
			spans: tracetest.SpanStubs{
				tracetest.SpanStub{
					Name:      constants.TROUBLESHOOT_ROOT_SPAN_NAME,
					StartTime: time.Now(),
					EndTime:   time.Now().Add(time.Second),
				},
			},
			want: "Duration: 1,000ms",
		},
		{
			name: "with collectors",
			spans: tracetest.SpanStubs{
				tracetest.SpanStub{
					Name: "all-logs", StartTime: time.Now(), EndTime: time.Now().Add(time.Minute),
					Attributes: []attribute.KeyValue{
						attribute.String("type", "*collect.CollectLogs"),
					},
				},
				tracetest.SpanStub{
					Name: "host-os", StartTime: time.Now(), EndTime: time.Now().Add(time.Second),
					Attributes: []attribute.KeyValue{
						attribute.String("type", "*collect.CollectHostOS"),
					},
				},
			},
			want: `
========= Collectors summary ==========
all-logs : 60,000ms
host-os  : 1,000ms`,
		},
		{
			name: "with analyzers",
			spans: tracetest.SpanStubs{
				tracetest.SpanStub{
					Name: "cluster-version", StartTime: time.Now(), EndTime: time.Now().Add(time.Second),
					Attributes: []attribute.KeyValue{
						attribute.String("type", "*analyzer.AnalyzeClusterVersion"),
					},
				},
				tracetest.SpanStub{
					Name: "host-cpu", StartTime: time.Now(), EndTime: time.Now().Add(time.Minute),
					Attributes: []attribute.KeyValue{
						attribute.String("type", "*analyzer.AnalyzeHostCPU"),
					},
				},
			},
			want: `
========= Analysers summary ==========
host-cpu        : 60,000ms
cluster-version : 1,000ms
`,
		},
		{
			name: "with redactors",
			spans: tracetest.SpanStubs{
				tracetest.SpanStub{
					Name: "cluster redactor", StartTime: time.Now(), EndTime: time.Now().Add(time.Second),
					Attributes: []attribute.KeyValue{
						attribute.String("type", "Redactors"),
					},
				},
			},
			want: `
========= Redactors summary ==========
cluster redactor : 1,000ms`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Exporter{}

			ctx := context.Background()
			err := e.ExportSpans(ctx, tt.spans.Snapshots())
			require.NoError(t, err)

			assert.Contains(t, e.GetSummary(), strings.TrimSpace(tt.want))
		})
	}
}

func TestExporter_Shutdown(t *testing.T) {
	e := &Exporter{}

	ctx := context.Background()
	spans := tracetest.SpanStubs{}
	for i := 0; i < 5; i++ {
		spans = append(spans, tracetest.SpanStub{Name: fmt.Sprintf("span-%d", i)})
	}

	err := e.ExportSpans(ctx, spans.Snapshots())
	require.NoError(t, err)

	assert.Len(t, e.allSpans, 5)

	require.NoError(t, e.Shutdown(ctx))
	assert.Len(t, e.allSpans, 0)

	err = e.ExportSpans(ctx, spans.Snapshots())
	require.NoError(t, err)

	assert.Len(t, e.allSpans, 0)
}
