package transport

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockTransport struct {
	lastReq *http.Request
	resp    *http.Response
	err     error
	closed  bool
}

func (m *mockTransport) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	m.lastReq = req.Clone(ctx)
	if m.err != nil {
		return nil, m.err
	}
	if m.resp != nil {
		return m.resp, nil
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader("ok")),
	}, nil
}

func (m *mockTransport) Close() error {
	m.closed = true
	return nil
}

func TestTransportInterfaceWithMock(t *testing.T) {
	var tr Transport = &mockTransport{}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.test/healthz", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	resp, err := tr.Do(t.Context(), req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}
