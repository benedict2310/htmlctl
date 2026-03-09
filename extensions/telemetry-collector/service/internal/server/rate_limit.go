package server

import (
	"sync"
	"time"
)

type Clock func() time.Time

type RateLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	now    Clock
	state  map[string]rateState
	hits   uint64

	maxEntries  int
	sweepEveryN uint64
}

type rateState struct {
	count   int
	resetAt time.Time
}

func NewRateLimiter(limit int, window time.Duration, now Clock) *RateLimiter {
	if limit <= 0 {
		limit = 1
	}
	if window <= 0 {
		window = time.Minute
	}
	if now == nil {
		now = time.Now
	}
	return &RateLimiter{
		limit:       limit,
		window:      window,
		now:         now,
		state:       make(map[string]rateState),
		maxEntries:  10000,
		sweepEveryN: 256,
	}
}

func (l *RateLimiter) Allow(key string) bool {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.hits++
	if l.sweepEveryN > 0 && l.hits%l.sweepEveryN == 0 {
		l.pruneExpired(now)
	}

	s, ok := l.state[key]
	if !ok || now.After(s.resetAt) {
		if !ok && l.maxEntries > 0 && len(l.state) >= l.maxEntries {
			l.pruneExpired(now)
			if len(l.state) >= l.maxEntries {
				return false
			}
		}
		l.state[key] = rateState{count: 1, resetAt: now.Add(l.window)}
		return true
	}
	if s.count >= l.limit {
		return false
	}
	s.count++
	l.state[key] = s
	return true
}

func (l *RateLimiter) pruneExpired(now time.Time) {
	for key, state := range l.state {
		if now.After(state.resetAt) {
			delete(l.state, key)
		}
	}
}
