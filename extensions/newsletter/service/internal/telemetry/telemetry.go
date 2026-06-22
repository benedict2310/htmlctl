package telemetry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
)

// Config captures the OpenTelemetry environment values this service cares about.
// The exporter itself reads the standard OTEL_* variables, including headers.
type Config struct {
	Enabled     bool
	Endpoint    string
	Protocol    string
	ServiceName string
	Environment string
}

// LoadConfig reads standard OpenTelemetry environment variables. Telemetry is
// opt-in: without OTEL_EXPORTER_OTLP_ENDPOINT, the service starts without an SDK.
func LoadConfig(defaultServiceName, environment string) Config {
	serviceName := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	if serviceName == "" {
		serviceName = strings.TrimSpace(defaultServiceName)
	}

	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	protocol := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"))
	return Config{
		Enabled:     endpoint != "",
		Endpoint:    endpoint,
		Protocol:    protocol,
		ServiceName: serviceName,
		Environment: strings.TrimSpace(environment),
	}
}

type Options struct {
	ServiceName string
	Environment string
	Logger      *log.Logger
}

type Shutdown func(context.Context) error

// Init installs a global OpenTelemetry tracer provider using the standard OTEL_*
// environment variables. It deliberately never reads or logs ingest tokens.
func Init(ctx context.Context, options Options) (Shutdown, Config, error) {
	logger := options.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	cfg := LoadConfig(options.ServiceName, options.Environment)
	if !cfg.Enabled {
		logger.Print("opentelemetry disabled: OTEL_EXPORTER_OTLP_ENDPOINT is not set")
		return func(context.Context) error { return nil }, cfg, nil
	}
	if cfg.ServiceName == "" {
		return nil, cfg, errors.New("OTEL_SERVICE_NAME is required when telemetry is enabled")
	}
	if cfg.Protocol != "" && cfg.Protocol != "http/protobuf" {
		return nil, cfg, fmt.Errorf("OTEL_EXPORTER_OTLP_PROTOCOL must be http/protobuf, got %q", cfg.Protocol)
	}

	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, cfg, fmt.Errorf("create OTLP trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			attribute.String("service.namespace", "htmlctl"),
			semconv.DeploymentEnvironmentName(cfg.Environment),
		),
	)
	if err != nil {
		return nil, cfg, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}

	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(2*time.Second)),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	logger.Printf("opentelemetry enabled for service %q", cfg.ServiceName)
	return provider.Shutdown, cfg, nil
}

// WrapHandler adds HTTP server spans around the supplied handler.
func WrapHandler(handler http.Handler, serviceName string) http.Handler {
	operation := strings.TrimSpace(serviceName)
	if operation == "" {
		operation = "http.server"
	}
	return otelhttp.NewHandler(handler, operation)
}
