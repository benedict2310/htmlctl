package transport

import (
	"context"
	"net/http"
)

// Transport executes HTTP requests to htmlservd over a chosen network transport.
type Transport interface {
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
	Close() error
}
