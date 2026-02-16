package config

import (
	"strings"
	"testing"
)

func TestResolveContextWithExplicitOverride(t *testing.T) {
	cfg := Config{
		CurrentContext: "staging",
		Contexts: []Context{
			{
				Name:        "staging",
				Server:      "ssh://root@staging.example.com",
				Website:     "futurelab",
				Environment: "staging",
			},
			{
				Name:        "prod",
				Server:      "ssh://root@prod.example.com",
				Website:     "futurelab",
				Environment: "prod",
				Port:        8420,
			},
		},
	}

	ctx, err := ResolveContext(cfg, "prod")
	if err != nil {
		t.Fatalf("ResolveContext() error = %v", err)
	}
	if ctx.Name != "prod" || ctx.Environment != "prod" {
		t.Fatalf("expected prod context, got %#v", ctx)
	}
	if ctx.RemotePort != 8420 {
		t.Fatalf("expected resolved remote port 8420, got %d", ctx.RemotePort)
	}
}

func TestResolveContextUsesCurrentContextByDefault(t *testing.T) {
	cfg := Config{
		CurrentContext: "staging",
		Contexts: []Context{
			{
				Name:        "staging",
				Server:      "ssh://root@staging.example.com",
				Website:     "futurelab",
				Environment: "staging",
			},
		},
	}

	ctx, err := ResolveContext(cfg, "")
	if err != nil {
		t.Fatalf("ResolveContext() error = %v", err)
	}
	if ctx.Name != "staging" {
		t.Fatalf("expected staging context, got %#v", ctx)
	}
}

func TestResolveContextMissingContextListsAvailable(t *testing.T) {
	cfg := Config{
		CurrentContext: "staging",
		Contexts: []Context{
			{
				Name:        "staging",
				Server:      "ssh://root@staging.example.com",
				Website:     "futurelab",
				Environment: "staging",
			},
			{
				Name:        "prod",
				Server:      "ssh://root@prod.example.com",
				Website:     "futurelab",
				Environment: "prod",
			},
		},
	}

	_, err := ResolveContext(cfg, "does-not-exist")
	if err == nil {
		t.Fatalf("expected missing context error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "does-not-exist") {
		t.Fatalf("expected missing context name in error, got %v", err)
	}
	if !strings.Contains(msg, "prod") || !strings.Contains(msg, "staging") {
		t.Fatalf("expected available contexts in error, got %v", err)
	}
}

func TestResolveContextNoCurrentContextSelected(t *testing.T) {
	cfg := Config{
		CurrentContext: "",
		Contexts: []Context{
			{
				Name:        "staging",
				Server:      "ssh://root@staging.example.com",
				Website:     "futurelab",
				Environment: "staging",
			},
		},
	}

	_, err := ResolveContext(cfg, "")
	if err == nil {
		t.Fatalf("expected no-current-context error")
	}
	if !strings.Contains(err.Error(), "set current-context") {
		t.Fatalf("expected current-context guidance, got %v", err)
	}
}

func TestResolveContextMissingFromEmptyContexts(t *testing.T) {
	cfg := Config{
		CurrentContext: "staging",
		Contexts:       nil,
	}

	_, err := ResolveContext(cfg, "prod")
	if err == nil {
		t.Fatalf("expected missing context error")
	}
	if !strings.Contains(err.Error(), "config has no contexts") {
		t.Fatalf("expected no-contexts detail, got %v", err)
	}
}
