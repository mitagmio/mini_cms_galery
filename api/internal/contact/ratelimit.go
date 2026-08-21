package contact

import (
	"sync"
	"time"
)

type Limiter struct {
	mu     sync.Mutex
	hits   map[string][]time.Time
	window time.Duration
	max    int
}

func NewLimiter(window time.Duration, max int) *Limiter {
	if window <= 0 {
		window = 10 * time.Minute
	}
	if max <= 0 {
		max = 5
	}
	return &Limiter{hits: map[string][]time.Time{}, window: window, max: max}
}

func (l *Limiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	if key == "" {
		key = "unknown"
	}
	now := time.Now()
	cutoff := now.Add(-l.window)
	l.mu.Lock()
	defer l.mu.Unlock()
	q := l.hits[key]
	n := 0
	for _, t := range q {
		if t.After(cutoff) {
			q[n] = t
			n++
		}
	}
	q = q[:n]
	if len(q) >= l.max {
		l.hits[key] = q
		return false
	}
	l.hits[key] = append(q, now)
	return true
}
