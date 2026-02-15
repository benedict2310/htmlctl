package release

import (
	"crypto/rand"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

var (
	entropyMu sync.Mutex
	entropy   = ulid.Monotonic(rand.Reader, 0)
)

func NewReleaseID(now time.Time) (string, error) {
	entropyMu.Lock()
	defer entropyMu.Unlock()
	id, err := ulid.New(ulid.Timestamp(now.UTC()), entropy)
	if err != nil {
		if err == io.EOF {
			return "", fmt.Errorf("generate release id: insufficient entropy")
		}
		return "", fmt.Errorf("generate release id: %w", err)
	}
	return id.String(), nil
}
