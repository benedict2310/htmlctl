package audit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const defaultQueueSize = 512

type AsyncLogger struct {
	sink    *SQLiteLogger
	onError func(error)

	queue   chan Entry
	closed  bool
	mu      sync.RWMutex
	once    sync.Once
	wg      sync.WaitGroup
	pending sync.WaitGroup
}

func NewAsyncLogger(sink *SQLiteLogger, queueSize int, onError func(error)) *AsyncLogger {
	if queueSize <= 0 {
		queueSize = defaultQueueSize
	}
	l := &AsyncLogger{
		sink:    sink,
		onError: onError,
		queue:   make(chan Entry, queueSize),
	}
	l.wg.Add(1)
	go l.run()
	return l
}

func (l *AsyncLogger) run() {
	defer l.wg.Done()
	for entry := range l.queue {
		ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
		err := l.sink.Log(ctx, entry)
		cancel()
		if err != nil && l.onError != nil {
			l.onError(err)
		}
		l.pending.Done()
	}
}

func (l *AsyncLogger) Log(_ context.Context, entry Entry) error {
	l.mu.RLock()
	if l.closed {
		l.mu.RUnlock()
		return fmt.Errorf("audit logger is closed")
	}
	l.pending.Add(1)
	select {
	case l.queue <- entry:
		l.mu.RUnlock()
		return nil
	default:
		l.pending.Done()
		l.mu.RUnlock()
		return fmt.Errorf("audit log queue is full")
	}
}

func (l *AsyncLogger) Query(ctx context.Context, filter Filter) (QueryResult, error) {
	return l.sink.Query(ctx, filter)
}

func (l *AsyncLogger) Close(ctx context.Context) error {
	l.once.Do(func() {
		l.mu.Lock()
		l.closed = true
		close(l.queue)
		l.mu.Unlock()
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		l.wg.Wait()
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *AsyncLogger) WaitIdle(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		defer close(done)
		l.pending.Wait()
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
