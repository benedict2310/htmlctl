package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/benedict2310/htmlctl/extensions/newsletter/service/internal/config"
	"github.com/benedict2310/htmlctl/extensions/newsletter/service/internal/migrate"
	"github.com/benedict2310/htmlctl/extensions/newsletter/service/internal/server"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: htmlctl-newsletter <serve|migrate>")
	}

	switch args[0] {
	case "serve":
		return runServe()
	case "migrate":
		return runMigrate()
	default:
		return fmt.Errorf("unknown command %q (expected serve or migrate)", args[0])
	}
}

func runServe() error {
	cfg, err := config.LoadServeFromEnv()
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	if _, err := migrate.Apply(context.Background(), db); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.HTTPAddr, err)
	}
	defer ln.Close()

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.New(),
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

	log.Printf("htmlctl-newsletter (%s) listening on %s", cfg.Environment, cfg.HTTPAddr)

	select {
	case <-shutdownCtx.Done():
		ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
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

func runMigrate() error {
	cfg, err := config.LoadMigrateFromEnv()
	if err != nil {
		return err
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	versions, err := migrate.Apply(context.Background(), db)
	if err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	if len(versions) == 0 {
		log.Printf("no migrations pending (%s)", cfg.Environment)
		return nil
	}

	log.Printf("applied %d migration(s) (%s): %v", len(versions), cfg.Environment, versions)
	return nil
}
