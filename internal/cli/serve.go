package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var signalNotifyContext = signal.NotifyContext

func newServeCmd() *cobra.Command {
	var port int

	cmd := &cobra.Command{
		Use:   "serve <dir>",
		Short: "Serve static output locally",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signalNotifyContext(cmd.Context(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			return serveDirectory(ctx, args[0], port, cmd.OutOrStdout())
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "Port to listen on (use 0 for random available port)")

	return cmd
}

func serveDirectory(ctx context.Context, dir string, port int, out io.Writer) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat serve directory %s: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("serve path is not a directory: %s", dir)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	defer listener.Close()

	server := &http.Server{
		Handler: loggingMiddleware(http.FileServer(http.Dir(dir)), out),
	}

	fmt.Fprintf(out, "Serving %s at http://%s\n", dir, listener.Addr().String())

	errCh := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("serve failed: %w", err)
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("graceful shutdown failed: %w", err)
		}
		<-errCh
		return nil
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func loggingMiddleware(next http.Handler, out io.Writer) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, req)
		fmt.Fprintf(out, "%s %s %d\n", req.Method, req.URL.Path, recorder.status)
	})
}
