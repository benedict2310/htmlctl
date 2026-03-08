package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/benedict2310/htmlctl/extensions/newsletter/service/internal/campaign"
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
		return errors.New("usage: htmlctl-newsletter <serve|migrate|import-legacy|campaign>")
	}

	switch args[0] {
	case "serve":
		return runServe()
	case "migrate":
		return runMigrate()
	case "import-legacy":
		return runImportLegacy(args[1:])
	case "campaign":
		return runCampaign(args[1:])
	default:
		return fmt.Errorf("unknown command %q (expected serve, migrate, import-legacy, or campaign)", args[0])
	}
}

func runServe() error {
	cfg, err := config.LoadServeFromEnv()
	if err != nil {
		return err
	}

	db, err := openDB(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := migrate.Apply(context.Background(), db); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	store := server.NewPostgresStore(db)
	hasMailerKey := hasRealResendKey(cfg.ResendAPIKey)
	if err := ensureMailerConfig(cfg.Environment, hasMailerKey); err != nil {
		return err
	}

	var mailer server.Mailer
	switch {
	case hasMailerKey:
		mailer = server.NewResendMailer(cfg.ResendAPIKey, cfg.ResendFrom)
	case cfg.Environment == "staging":
		mailer = server.LoggingMailer{}
	default:
		mailer = server.NoopMailer{}
	}

	ln, err := net.Listen("tcp", cfg.HTTPAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", cfg.HTTPAddr, err)
	}
	defer ln.Close()

	httpServer := &http.Server{
		Addr: cfg.HTTPAddr,
		Handler: server.New(server.Options{
			Store:         store,
			Mailer:        mailer,
			Logger:        log.Default(),
			Environment:   cfg.Environment,
			PublicBaseURL: cfg.PublicBaseURL,
			LinkSecret:    cfg.LinkSecret,
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

	log.Printf("htmlctl-newsletter (%s) listening on %s", cfg.Environment, cfg.HTTPAddr)

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

func runMigrate() error {
	cfg, err := config.LoadMigrateFromEnv()
	if err != nil {
		return err
	}

	db, err := openDB(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

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

func runImportLegacy(args []string) error {
	fs := flag.NewFlagSet("import-legacy", flag.ContinueOnError)
	sourceDBURL := fs.String("source-database-url", "", "legacy source database URL")
	confirm := fs.Bool("confirm", false, "apply import instead of dry-run")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*sourceDBURL) == "" {
		return errors.New("import-legacy requires --source-database-url")
	}

	cfg, err := config.LoadMigrateFromEnv()
	if err != nil {
		return err
	}
	targetDB, err := openDB(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer targetDB.Close()
	if _, err := migrate.Apply(context.Background(), targetDB); err != nil {
		return fmt.Errorf("apply target migrations: %w", err)
	}

	sourceDB, err := openDB(strings.TrimSpace(*sourceDBURL))
	if err != nil {
		return fmt.Errorf("open source database: %w", err)
	}
	defer sourceDB.Close()

	summary, err := server.NewPostgresStore(targetDB).ImportLegacySubscribers(context.Background(), sourceDB, !*confirm)
	if err != nil {
		return err
	}
	log.Printf("legacy import summary (%s): source_total=%d inserted=%d updated=%d skipped=%d dry_run=%t statuses=%v", cfg.Environment, summary.SourceTotal, summary.Inserted, summary.Updated, summary.Skipped, !*confirm, summary.StatusCounts)
	return nil
}

func runCampaign(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: htmlctl-newsletter campaign <upsert|preview|send>")
	}
	switch args[0] {
	case "upsert":
		return runCampaignUpsert(args[1:])
	case "preview":
		return runCampaignPreview(args[1:])
	case "send":
		return runCampaignSend(args[1:])
	default:
		return fmt.Errorf("unknown campaign command %q (expected upsert, preview, or send)", args[0])
	}
}

func runCampaignUpsert(args []string) error {
	fs := flag.NewFlagSet("campaign upsert", flag.ContinueOnError)
	slug := fs.String("slug", "", "campaign slug")
	subject := fs.String("subject", "", "email subject")
	htmlFile := fs.String("html-file", "", "path to HTML body file")
	textFile := fs.String("text-file", "", "path to text body file")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadMigrateFromEnv()
	if err != nil {
		return err
	}
	db, err := openDB(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := migrate.Apply(context.Background(), db); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	stored, err := campaign.UpsertFromFiles(context.Background(), server.NewPostgresStore(db), *slug, *subject, *htmlFile, *textFile, time.Now())
	if err != nil {
		return err
	}
	log.Printf("campaign %q stored (%s, status=%s)", stored.Slug, cfg.Environment, stored.Status)
	return nil
}

func runCampaignPreview(args []string) error {
	fs := flag.NewFlagSet("campaign preview", flag.ContinueOnError)
	slug := fs.String("slug", "", "campaign slug")
	to := fs.String("to", "", "preview recipient")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.LoadServeFromEnv()
	if err != nil {
		return err
	}
	if !hasRealResendKey(cfg.ResendAPIKey) {
		return errors.New("NEWSLETTER_RESEND_API_KEY must be configured for campaign preview")
	}
	if strings.TrimSpace(*slug) == "" {
		return errors.New("campaign preview requires --slug")
	}

	db, err := openDB(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := migrate.Apply(context.Background(), db); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	store := server.NewPostgresStore(db)
	mailer := server.NewResendMailer(cfg.ResendAPIKey, cfg.ResendFrom)
	if err := campaign.Preview(context.Background(), store, mailer, *slug, *to); err != nil {
		return err
	}
	log.Printf("sent campaign preview %q (%s) to %s", *slug, cfg.Environment, strings.TrimSpace(*to))
	return nil
}

func runCampaignSend(args []string) error {
	fs := flag.NewFlagSet("campaign send", flag.ContinueOnError)
	slug := fs.String("slug", "", "campaign slug")
	mode := fs.String("mode", "all", "send mode: all or seed")
	seedEmails := fs.String("seed-emails", "", "comma-separated seed list used only in seed mode")
	interval := fs.Duration("interval", 30*time.Second, "delay between provider calls")
	staleAfter := fs.Duration("stale-after", 10*time.Minute, "retry in-flight send claims older than this")
	confirm := fs.Bool("confirm", false, "required to send to recipients")
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*slug) == "" {
		return errors.New("campaign send requires --slug")
	}

	cfg, err := config.LoadServeFromEnv()
	if err != nil {
		return err
	}
	if !hasRealResendKey(cfg.ResendAPIKey) {
		return errors.New("NEWSLETTER_RESEND_API_KEY must be configured for campaign send")
	}

	db, err := openDB(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := migrate.Apply(context.Background(), db); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	store := server.NewPostgresStore(db)
	mailer := server.NewResendMailer(cfg.ResendAPIKey, cfg.ResendFrom)
	summary, err := campaign.Send(context.Background(), store, mailer, campaign.SendOptions{
		Slug:          *slug,
		Mode:          *mode,
		SeedEmails:    []string{*seedEmails},
		Interval:      *interval,
		StaleAfter:    *staleAfter,
		Confirm:       *confirm,
		PublicBaseURL: cfg.PublicBaseURL,
		LinkSecret:    cfg.LinkSecret,
	})
	if err != nil {
		return err
	}
	log.Printf("campaign send summary %q (%s): eligible=%d attempted=%d succeeded=%d failed=%d skipped=%d mode=%s interval=%s", *slug, cfg.Environment, summary.Eligible, summary.Attempted, summary.Succeeded, summary.Failed, summary.Skipped, *mode, interval.String())
	return nil
}

func openDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("pgx", strings.TrimSpace(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}

func hasRealResendKey(key string) bool {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return false
	}
	return !strings.HasPrefix(strings.ToUpper(trimmed), "REPLACE_WITH_")
}

func ensureMailerConfig(environment string, hasMailerKey bool) error {
	if environment == "prod" && !hasMailerKey {
		return errors.New("NEWSLETTER_RESEND_API_KEY must be configured in prod")
	}
	return nil
}
