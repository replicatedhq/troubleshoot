package traces

import (
	"context"

	"github.com/replicatedhq/troubleshoot/pkg/logger"
	"github.com/replicatedhq/troubleshoot/pkg/version"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"
)

// ConfigureTracing configures the OpenTelemetry trace provider for CLI
// commands. Projects using troubleshoot as a library would need to register
// troubleshoot's exporter like so.
//
//	var tp *trace.TracerProvider	// client application's trace provider
//	tp.RegisterSpanProcessor(
//		trace.NewSimpleSpanProcessor(
//			traces.GetExporterInstance(),	// Troubleshoot's exporter
//		),
//	)
//
// The client application is responsible for constructing the trace provider
// and registering the exporter. Multiple exporters can be registered.
func ConfigureTracing(processName string) (func(), error) {
	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			resource.Default().SchemaURL(),
			semconv.ProcessCommandKey.String(processName),
			semconv.ProcessRuntimeVersionKey.String(version.Version()),
			attribute.String("environment", "cli"),
		),
	)

	if err != nil {
		return nil, err
	}

	// Trace provider for support bundle cli. Each application is required
	// to have its own trace provider.
	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithSyncer(
			GetExporterInstance(),
		),
		trace.WithResource(r),
	)

	otel.SetTracerProvider(tp)

	return func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			logger.Printf("Failed to shutdown trace provider: %v", err)
		}
	}, nil
}
