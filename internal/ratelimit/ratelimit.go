package ratelimit

import (
	"sync"
	"time"
)

const maxEntries = 10_000

type entry struct {
	used      int
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

func (l *Limiter) Take(key string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	e, ok := l.entries[key]
	if ok && now.Before(e.windowEnd) && e.used >= l.max {
		return e.windowEnd.Sub(now), false
	}

	if !ok || !now.Before(e.windowEnd) {
		l.makeRoom(now)
		e = entry{windowEnd: now.Add(l.window)}
	}
	e.used++
	l.entries[key] = e

	return 0, true
}

func (l *Limiter) Release(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	if !ok {
		return
	}

	e.used--
	if e.used <= 0 {
		delete(l.entries, key)
		return
	}
	l.entries[key] = e
}

func (l *Limiter) makeRoom(now time.Time) {
	if now.Sub(l.lastSweep) >= l.window {
		for key, e := range l.entries {
			if !now.Before(e.windowEnd) {
				delete(l.entries, key)
			}
		}
		l.lastSweep = now
	}

	if len(l.entries) >= maxEntries {
		for key := range l.entries {
			delete(l.entries, key)
			break
		}
	}
}
