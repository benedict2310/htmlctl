package telemetry

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	apilog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestLoadConfigDisablesTelemetryWithoutEndpoint(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	t.Setenv("OTEL_SERVICE_NAME", "")

	cfg := LoadConfig("htmlctl-newsletter", "staging")
	if cfg.Enabled {
		t.Fatal("expected telemetry to be disabled without OTLP endpoint")
	}
	if cfg.ServiceName != "htmlctl-newsletter" {
		t.Fatalf("ServiceName = %q, want default service name", cfg.ServiceName)
	}
	if cfg.Environment != "staging" {
		t.Fatalf("Environment = %q, want staging", cfg.Environment)
	}
}

func TestLoadConfigUsesStandardOTelEnvironment(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "https://example.invalid/api/v2/otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_SERVICE_NAME", "custom-service")

	cfg := LoadConfig("htmlctl-newsletter", "prod")
	if !cfg.Enabled {
		t.Fatal("expected telemetry to be enabled when OTLP endpoint is set")
	}
	if cfg.Endpoint != "https://example.invalid/api/v2/otlp" {
		t.Fatalf("Endpoint = %q", cfg.Endpoint)
	}
	if cfg.Protocol != "http/protobuf" {
		t.Fatalf("Protocol = %q", cfg.Protocol)
	}
	if cfg.ServiceName != "custom-service" {
		t.Fatalf("ServiceName = %q, want env override", cfg.ServiceName)
	}
	if cfg.Environment != "prod" {
		t.Fatalf("Environment = %q, want prod", cfg.Environment)
	}
}

func TestInitRejectsNonHTTPProtobufProtocol(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "https://example.invalid/api/v2/otlp")
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")

	_, _, err := Init(context.Background(), Options{ServiceName: "htmlctl-newsletter", Environment: "staging"})
	if err == nil {
		t.Fatal("expected invalid protocol error")
	}
}

func TestWrapHandlerPreservesHandlerBehavior(t *testing.T) {
	handler := WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}), "htmlctl-newsletter")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
}

func TestWrapHandlerRecordsTimestampsAndHTTPStatusAttributes(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	previousProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previousProvider)
		_ = provider.Shutdown(context.Background())
	})

	handler := WrapHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}), "htmlctl-newsletter")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/newsletter/verify", nil)
	handler.ServeHTTP(rec, req)

	spans := spanRecorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(spans))
	}
	span := spans[0]
	if span.StartTime().IsZero() {
		t.Fatal("span start time is zero")
	}
	if span.EndTime().IsZero() {
		t.Fatal("span end time is zero")
	}
	if !span.EndTime().After(span.StartTime()) && !span.EndTime().Equal(span.StartTime()) {
		t.Fatalf("span end time %s is before start time %s", span.EndTime(), span.StartTime())
	}
	if got, ok := intAttribute(span.Attributes(), "http.status_code"); !ok || got != http.StatusTeapot {
		t.Fatalf("http.status_code attribute = %d, %t; want %d, true", got, ok, http.StatusTeapot)
	}
	if got, ok := intAttribute(span.Attributes(), "http.response.status_code"); !ok || got != http.StatusTeapot {
		t.Fatalf("http.response.status_code attribute = %d, %t; want %d, true", got, ok, http.StatusTeapot)
	}
}

func TestOTelLogWriterEmitsTimestampedRecords(t *testing.T) {
	exporter := &recordingLogExporter{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(exporter)))
	defer func() { _ = provider.Shutdown(context.Background()) }()

	var stderr bytes.Buffer
	now := time.Date(2026, 6, 22, 12, 30, 0, 0, time.UTC)
	writer := NewLogWriter(&stderr, provider.Logger("htmlctl-newsletter"), func() time.Time { return now })

	if _, err := writer.Write([]byte("hello from newsletter\n")); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if stderr.String() != "hello from newsletter\n" {
		t.Fatalf("stderr got %q", stderr.String())
	}
	if len(exporter.records) != 1 {
		t.Fatalf("exported records = %d, want 1", len(exporter.records))
	}
	record := exporter.records[0]
	if record.Timestamp() != now {
		t.Fatalf("record timestamp = %s, want %s", record.Timestamp(), now)
	}
	if record.ObservedTimestamp() != now {
		t.Fatalf("record observed timestamp = %s, want %s", record.ObservedTimestamp(), now)
	}
	if got := record.Body().AsString(); got != "hello from newsletter" {
		t.Fatalf("record body = %q", got)
	}
}

func intAttribute(attrs []attribute.KeyValue, key string) (int, bool) {
	for _, attr := range attrs {
		if string(attr.Key) == key {
			return int(attr.Value.AsInt64()), true
		}
	}
	return 0, false
}

type recordingLogExporter struct {
	records []sdklog.Record
}

func (e *recordingLogExporter) Export(_ context.Context, records []sdklog.Record) error {
	for _, record := range records {
		e.records = append(e.records, record.Clone())
	}
	return nil
}

func (e *recordingLogExporter) Shutdown(context.Context) error {
	return nil
}

func (e *recordingLogExporter) ForceFlush(context.Context) error {
	return nil
}

var _ sdklog.Exporter = (*recordingLogExporter)(nil)
var _ apilog.Logger
