package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/benedict2310/htmlctl/internal/server"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("htmlservd", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "Path to config file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := server.LoadConfig(*configPath)
	if err != nil {
		return err
	}

	logger, err := server.NewLogger(cfg.LogLevel)
	if err != nil {
		return err
	}

	srv, err := server.New(cfg, logger, version)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	return srv.Run(ctx)
}
