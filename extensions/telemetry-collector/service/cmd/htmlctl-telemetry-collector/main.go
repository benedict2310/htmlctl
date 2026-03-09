package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/benedict2310/htmlctl/extensions/telemetry-collector/service/internal/config"
	"github.com/benedict2310/htmlctl/extensions/telemetry-collector/service/internal/server"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: htmlctl-telemetry-collector <serve>")
	}
	if args[0] != "serve" {
		return fmt.Errorf("unknown command %q (expected serve)", args[0])
	}
	return runServe()
}

func runServe() error {
	cfg, err := config.LoadServeFromEnv()
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.HTTPAddr, err)
	}
	defer ln.Close()

	httpServer := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: server.New(server.Options{
			Logger:        log.Default(),
			PublicBaseURL: cfg.PublicBaseURL,
			AllowedEvents: cfg.AllowedEvents,
			MaxBodyBytes:  cfg.MaxBodyBytes,
			MaxEvents:     cfg.MaxEvents,
			Forwarder: server.HTTPForwarder{
				BaseURL: cfg.HTMLSERVDBaseURL,
				Token:   cfg.HTMLSERVDToken,
			},
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	shutdownCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- httpServer.Serve(ln)
	}()

	log.Printf("htmlctl-telemetry-collector (%s) listening on %s -> %s", cfg.Environment, cfg.HTTPAddr, cfg.HTMLSERVDBaseURL)

	select {
	case <-shutdownCtx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("graceful shutdown: %w", err)
		}
		return nil
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve: %w", err)
	}
}
