package telemetry

import (
	"context"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"go.opentelemetry.io/otel/trace"
)

var (
	// Tracer is the global OpenTelemetry tracer
	Tracer trace.Tracer
	// TracerProvider is exposed for use with otelhttp and other instrumentation
	TracerProvider *sdktrace.TracerProvider
)

// parseEndpoint extracts the host from the OTEL endpoint URL
func parseEndpoint(endpoint string) string {
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	endpoint = strings.TrimSuffix(endpoint, "/v1/traces")
	endpoint = strings.TrimSuffix(endpoint, "/v1/logs")
	return endpoint
}

// InitTracer initializes the OpenTelemetry tracer and returns a shutdown function
func InitTracer(otelEndpoint, serviceName, serviceVersion string) (func(context.Context) error, error) {
	ctx := context.Background()

	endpoint := parseEndpoint(otelEndpoint)

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	// Set up W3C TraceContext propagator for distributed tracing
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Store and set the provider globally
	TracerProvider = tp
	otel.SetTracerProvider(tp)
	Tracer = otel.Tracer(serviceName)

	return tp.Shutdown, nil
}
