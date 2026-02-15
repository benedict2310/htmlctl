package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"
)

const shutdownTimeout = 10 * time.Second

type Server struct {
	cfg        Config
	logger     *slog.Logger
	version    string
	dataPaths  DataPaths
	listener   net.Listener
	httpServer *http.Server
	errCh      chan error
}

func New(cfg Config, logger *slog.Logger, version string) (*Server, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	if version == "" {
		version = "dev"
	}

	mux := http.NewServeMux()
	registerHealthRoutes(mux, version)

	srv := &Server{
		cfg:     cfg,
		logger:  logger,
		version: version,
		errCh:   make(chan error, 1),
		httpServer: &http.Server{
			Addr:         cfg.ListenAddr(),
			Handler:      mux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
	return srv, nil
}

func (s *Server) Start() error {
	paths, err := InitDataDir(s.cfg.DataDir)
	if err != nil {
		return err
	}
	s.dataPaths = paths

	ln, err := net.Listen("tcp", s.cfg.ListenAddr())
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.cfg.ListenAddr(), err)
	}
	s.listener = ln

	if !isLoopbackHost(s.cfg.BindAddr) {
		s.logger.Warn("binding to non-loopback address", "bind", s.cfg.BindAddr)
	}

	s.logger.Info("htmlservd starting",
		"listen_addr", ln.Addr().String(),
		"data_dir", s.cfg.DataDir,
		"version", s.version,
	)

	go func() {
		err := s.httpServer.Serve(ln)
		if err != nil && err != http.ErrServerClosed {
			s.errCh <- err
		}
		close(s.errCh)
	}()

	return nil
}

func (s *Server) Run(ctx context.Context) error {
	if err := s.Start(); err != nil {
		return err
	}

	select {
	case err := <-s.errCh:
		if err != nil {
			return fmt.Errorf("http server failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.listener == nil {
		return nil
	}

	s.logger.Info("htmlservd shutting down")
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	if err, ok := <-s.errCh; ok && err != nil {
		return fmt.Errorf("http server failed: %w", err)
	}

	s.listener = nil
	return nil
}

func (s *Server) Addr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func isLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

func parseLogLevel(level string) (slog.Level, error) {
	switch level {
	case "debug":
		return slog.LevelDebug, nil
	case "info", "":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("invalid log level %q (expected debug|info|warn|error)", level)
	}
}

func NewLogger(level string) (*slog.Logger, error) {
	parsed, err := parseLogLevel(level)
	if err != nil {
		return nil, err
	}
	h := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: parsed})
	return slog.New(h), nil
}
