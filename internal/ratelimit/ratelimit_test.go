package ratelimit

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func newTestLimiter(max int, window time.Duration) (*Limiter, *time.Time) {
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	l := New(max, window)
	l.now = func() time.Time { return clock }
	l.lastSweep = clock

	return l, &clock
}

func TestTakeBlocksAfterMax(t *testing.T) {
	l, _ := newTestLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if _, ok := l.Take("a"); !ok {
			t.Fatalf("take %d blocked, want allowed", i+1)
		}
	}

	wait, ok := l.Take("a")
	if ok {
		t.Fatal("fourth take allowed, want blocked")
	}
	if wait != time.Minute {
		t.Fatalf("retry after = %v, want 1m", wait)
	}
}

func TestConcurrentTakeIsAtomic(t *testing.T) {
	l := New(5, time.Minute)

	var wg sync.WaitGroup
	var mu sync.Mutex
	allowed := 0
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok := l.Take("a"); ok {
				mu.Lock()
				allowed++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if allowed != 5 {
		t.Fatalf("allowed %d of 50 concurrent takes, want 5", allowed)
	}
}

func TestReleaseGivesTheSlotBack(t *testing.T) {
	l, _ := newTestLimiter(2, time.Minute)

	l.Take("a")
	l.Take("a")
	if _, ok := l.Take("a"); ok {
		t.Fatal("third take allowed, want blocked")
	}

	l.Release("a")
	if _, ok := l.Take("a"); !ok {
		t.Fatal("take after release blocked, want allowed")
	}
}

func TestKeysAreIndependent(t *testing.T) {
	l, _ := newTestLimiter(1, time.Minute)

	l.Take("a")
	if _, ok := l.Take("b"); !ok {
		t.Fatal("key b blocked by key a")
	}
}

func TestBlockExpiresWithWindow(t *testing.T) {
	l, clock := newTestLimiter(1, time.Minute)

	l.Take("a")
	*clock = clock.Add(59 * time.Second)
	if wait, ok := l.Take("a"); ok || wait != time.Second {
		t.Fatalf("ok=%v wait=%v, want blocked for 1s", ok, wait)
	}

	*clock = clock.Add(time.Second)
	if _, ok := l.Take("a"); !ok {
		t.Fatal("still blocked after the window ended")
	}
}

func TestSweepRemovesExpiredEntries(t *testing.T) {
	l, clock := newTestLimiter(1, time.Minute)

	l.Take("a")
	l.Take("b")
	*clock = clock.Add(2 * time.Minute)
	l.Take("c")

	if len(l.entries) != 1 {
		t.Fatalf("entries = %d, want only the live key c", len(l.entries))
	}
}

func TestEntriesAreCapped(t *testing.T) {
	l := New(1, time.Hour)

	for i := 0; i < 3*maxEntries; i++ {
		l.Take(fmt.Sprintf("key-%d", i))
	}

	if len(l.entries) > maxEntries {
		t.Fatalf("entries = %d, want at most %d", len(l.entries), maxEntries)
	}
}
