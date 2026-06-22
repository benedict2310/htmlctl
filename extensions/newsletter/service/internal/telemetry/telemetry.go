package telemetry

import (
	"context"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/felixge/httpsnoop"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	apilog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
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
	Logger      *stdlog.Logger
}

type Shutdown func(context.Context) error

// Init installs a global OpenTelemetry tracer provider using the standard OTEL_*
// environment variables. It deliberately never reads or logs ingest tokens.
func Init(ctx context.Context, options Options) (Shutdown, Config, error) {
	logger := options.Logger
	if logger == nil {
		logger = stdlog.New(io.Discard, "", 0)
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

	traceExporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, cfg, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	logExporter, err := otlploghttp.New(ctx)
	if err != nil {
		return nil, cfg, fmt.Errorf("create OTLP log exporter: %w", err)
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, cfg, err
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(2*time.Second)),
		sdktrace.WithResource(res),
	)
	logProvider := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter)),
		sdklog.WithResource(res),
	)
	otel.SetTracerProvider(traceProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	stdlog.SetOutput(NewLogWriter(os.Stderr, logProvider.Logger(cfg.ServiceName), time.Now))

	logger.Printf("opentelemetry enabled for service %q", cfg.ServiceName)
	return func(ctx context.Context) error {
		return errors.Join(traceProvider.Shutdown(ctx), logProvider.Shutdown(ctx))
	}, cfg, nil
}

func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
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
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}
	return res, nil
}

// WrapHandler adds HTTP server spans around the supplied handler.
func WrapHandler(handler http.Handler, serviceName string) http.Handler {
	operation := strings.TrimSpace(serviceName)
	if operation == "" {
		operation = "http.server"
	}
	return otelhttp.NewHandler(statusAnnotatingHandler{handler: handler}, operation)
}

type statusAnnotatingHandler struct {
	handler http.Handler
}

func (h statusAnnotatingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics := httpsnoop.CaptureMetrics(h.handler, w, r)
	status := metrics.Code
	if status == 0 {
		status = http.StatusOK
	}

	span := trace.SpanFromContext(r.Context())
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(
		attribute.Int("http.status_code", status),
		attribute.Int("http.response.status_code", status),
	)
	if status >= http.StatusInternalServerError {
		span.SetStatus(codes.Error, http.StatusText(status))
	}
}

// NewLogWriter mirrors standard-library log output to destination while also
// emitting timestamped OpenTelemetry log records.
func NewLogWriter(destination io.Writer, logger apilog.Logger, now func() time.Time) io.Writer {
	if destination == nil {
		destination = io.Discard
	}
	if now == nil {
		now = time.Now
	}
	return &logWriter{destination: destination, logger: logger, now: now}
}

type logWriter struct {
	mu          sync.Mutex
	destination io.Writer
	logger      apilog.Logger
	now         func() time.Time
}

func (w *logWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n, err := w.destination.Write(p)
	if err != nil {
		return n, err
	}
	message := strings.TrimRight(string(p), "\r\n")
	if message == "" || w.logger == nil {
		return len(p), nil
	}

	ts := w.now()
	var record apilog.Record
	record.SetTimestamp(ts)
	record.SetObservedTimestamp(ts)
	record.SetSeverity(apilog.SeverityInfo)
	record.SetSeverityText("INFO")
	record.SetBody(apilog.StringValue(message))
	w.logger.Emit(context.Background(), record)
	return len(p), nil
}
