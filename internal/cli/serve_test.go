package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func TestServeCommandServesAndShutsDownGracefully(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>ok</h1>"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := NewRootCmd("test")
	out := &lockedBuffer{}
	errOut := &lockedBuffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"serve", dir, "--port", "0"})

	done := make(chan error, 1)
	go func() {
		done <- cmd.Execute()
	}()

	baseURL, err := waitForBaseURL(out, 3*time.Second)
	if err != nil {
		cancel()
		t.Fatalf("wait for server startup: %v\nstdout: %s\nstderr: %s", err, out.String(), errOut.String())
	}

	client := &http.Client{Timeout: 2 * time.Second}

	resp, err := client.Get(baseURL + "/")
	if err != nil {
		cancel()
		t.Fatalf("GET / failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		cancel()
		t.Fatalf("expected 200 for /, got %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "<h1>ok</h1>") {
		cancel()
		t.Fatalf("unexpected body for /: %s", string(body))
	}

	resp404, err := client.Get(baseURL + "/missing")
	if err != nil {
		cancel()
		t.Fatalf("GET /missing failed: %v", err)
	}
	_ = resp404.Body.Close()
	if resp404.StatusCode != http.StatusNotFound {
		cancel()
		t.Fatalf("expected 404 for /missing, got %d", resp404.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve command exited with error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("serve command did not exit after cancellation")
	}

	logs := out.String()
	if !strings.Contains(logs, "GET / 200") {
		t.Fatalf("expected request log for GET / 200, got:\n%s", logs)
	}
	if !strings.Contains(logs, "GET /missing 404") {
		t.Fatalf("expected request log for GET /missing 404, got:\n%s", logs)
	}
}

func TestServeCommandMissingDirectoryFails(t *testing.T) {
	cmd := NewRootCmd("test")
	out := &lockedBuffer{}
	errOut := &lockedBuffer{}
	cmd.SetOut(out)
	cmd.SetErr(errOut)
	cmd.SetArgs([]string{"serve", filepath.Join(t.TempDir(), "missing-dir")})

	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected error for missing directory")
	}
	if !strings.Contains(err.Error(), "stat serve directory") {
		t.Fatalf("expected stat error, got: %v", err)
	}
}

func waitForBaseURL(out *lockedBuffer, timeout time.Duration) (string, error) {
	re := regexp.MustCompile(`http://127\.0\.0\.1:[0-9]+`)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if match := re.FindString(out.String()); match != "" {
			return match, nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return "", context.DeadlineExceeded
}
