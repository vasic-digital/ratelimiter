// Package ladder implements a per-key fixed-step backoff state machine.
//
// On each failure for a given key, the ladder advances by one step,
// clamping at the last step. CheckBlocked reports whether the key is
// currently blocked AND the remaining retry-after duration.
//
// State is held in-memory via a sync.Map keyed by an opaque string
// (typically a client IP). Memory grows with the number of distinct
// failing keys; callers SHOULD periodically Prune entries whose
// blockedUntil + retentionTTL has passed.
//
// Use case: HTTP auth middleware that progressively delays retries
// from a misbehaving source. The Lava project (vasic-digital/Lava)
// consumes this primitive in lava-api-go/internal/auth/backoff.go to
// translate its operator-mandated ladder (2s, 5s, 10s, 30s, 1m, 1h)
// into per-IP HTTP 429 + Retry-After responses.
package ladder

import (
	"sync"
	"time"
)

// Ladder is a per-key fixed-step backoff state machine.
type Ladder struct {
	steps []time.Duration
	state sync.Map // key string → *entry
}

type entry struct {
	mu           sync.Mutex
	failures     int
	blockedUntil time.Time
}

// New constructs a Ladder with the given step durations.
// Panics if steps is empty.
func New(steps []time.Duration) *Ladder {
	if len(steps) == 0 {
		panic("ladder.New: steps must be non-empty")
	}
	cp := make([]time.Duration, len(steps))
	copy(cp, steps)
	return &Ladder{steps: cp}
}

// RecordFailure advances the failure counter for key and returns the
// duration to block for. Failures past the last step clamp to it.
func (l *Ladder) RecordFailure(key string, now time.Time) time.Duration {
	e := l.lookup(key)
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.failures < len(l.steps) {
		e.failures++
	}
	duration := l.steps[e.failures-1]
	e.blockedUntil = now.Add(duration)
	return duration
}

// Reset clears the counter for key.
func (l *Ladder) Reset(key string) {
	l.state.Delete(key)
}

// CheckBlocked reports whether key is currently blocked and how long
// until retry is allowed.
func (l *Ladder) CheckBlocked(key string, now time.Time) (bool, time.Duration) {
	raw, ok := l.state.Load(key)
	if !ok {
		return false, 0
	}
	e := raw.(*entry)
	e.mu.Lock()
	defer e.mu.Unlock()
	if now.Before(e.blockedUntil) {
		return true, e.blockedUntil.Sub(now)
	}
	return false, 0
}

// Prune removes entries whose blockedUntil + retention has passed.
// Returns the number of entries removed.
func (l *Ladder) Prune(now time.Time, retention time.Duration) int {
	pruned := 0
	cutoff := now.Add(-retention)
	l.state.Range(func(k, v any) bool {
		e := v.(*entry)
		e.mu.Lock()
		expired := e.blockedUntil.Before(cutoff)
		e.mu.Unlock()
		if expired {
			l.state.Delete(k)
			pruned++
		}
		return true
	})
	return pruned
}

func (l *Ladder) lookup(key string) *entry {
	raw, _ := l.state.LoadOrStore(key, &entry{})
	return raw.(*entry)
}
