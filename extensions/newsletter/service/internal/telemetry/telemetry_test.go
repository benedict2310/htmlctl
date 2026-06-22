package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
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
