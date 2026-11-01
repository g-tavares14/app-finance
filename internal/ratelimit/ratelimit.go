package ratelimit

import (
	"sync"
	"time"
)

type entry struct {
	failures  int
	windowEnd time.Time
}

type Limiter struct {
	mu        sync.Mutex
	max       int
	window    time.Duration
	entries   map[string]entry
	lastSweep time.Time
	now       func() time.Time
}

func New(max int, window time.Duration) *Limiter {
	return &Limiter{
		max:       max,
		window:    window,
		entries:   make(map[string]entry),
		lastSweep: time.Now(),
		now:       time.Now,
	}
}

func (l *Limiter) Blocked(key string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	e, ok := l.entries[key]
	if !ok || !now.Before(e.windowEnd) || e.failures < l.max {
		return 0, false
	}

	return e.windowEnd.Sub(now), true
}

func (l *Limiter) Fail(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	e, ok := l.entries[key]
	if !ok || !now.Before(e.windowEnd) {
		e = entry{windowEnd: now.Add(l.window)}
	}
	e.failures++
	l.entries[key] = e
}

func (l *Limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}

	for key, e := range l.entries {
		if !now.Before(e.windowEnd) {
			delete(l.entries, key)
		}
	}
	l.lastSweep = now
}
