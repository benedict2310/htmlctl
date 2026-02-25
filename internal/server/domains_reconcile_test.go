package server

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

type captureCaddyReloader struct {
	reasons []string
	err     error
}

func (c *captureCaddyReloader) Reload(ctx context.Context, reason string) error {
	c.reasons = append(c.reasons, reason)
	return c.err
}

func TestReconcileDomainConfigWithoutReloader(t *testing.T) {
	s := &Server{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if err := s.reconcileDomainConfig(context.Background(), "noop"); err != nil {
		t.Fatalf("reconcileDomainConfig() error = %v", err)
	}
}

func TestReconcileDomainConfigSuccess(t *testing.T) {
	reloader := &captureCaddyReloader{}
	s := &Server{
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		caddyReloader: reloader,
	}
	if err := s.reconcileDomainConfig(context.Background(), "remove rollback failure sample.example.com"); err != nil {
		t.Fatalf("reconcileDomainConfig() error = %v", err)
	}
	if len(reloader.reasons) != 1 {
		t.Fatalf("expected one reload call, got %d", len(reloader.reasons))
	}
	if reloader.reasons[0] != "domain.reconcile remove rollback failure sample.example.com" {
		t.Fatalf("unexpected reconcile reason %q", reloader.reasons[0])
	}
}

func TestReconcileDomainConfigError(t *testing.T) {
	reloader := &captureCaddyReloader{err: context.DeadlineExceeded}
	s := &Server{
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		caddyReloader: reloader,
	}
	err := s.reconcileDomainConfig(context.Background(), "add rollback failure sample.example.com")
	if err == nil {
		t.Fatalf("expected reconcileDomainConfig error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected wrapped deadline exceeded, got %v", err)
	}
}
