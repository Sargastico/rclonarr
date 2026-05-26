package observability

import (
	"context"
	"fmt"

	"github.com/Sargastico/rclonarr/internal/core/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

// SetupTracing configures OTLP trace export and returns a shutdown function.
func SetupTracing(ctx context.Context) (func(context.Context) error, error) {
	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(config.App.OtelExporterEndpoint),
	}
	if config.App.Development {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}

	exporter, err := otlptracegrpc.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(config.AppTraceName),
			semconv.ServiceVersion(config.App.ReleaseVersion),
			semconv.DeploymentEnvironment(config.App.Environment),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	return tp.Shutdown, nil
}

// Tracer returns the global tracer for this service.
func Tracer() trace.Tracer {
	return otel.Tracer(config.AppTraceName)
}
