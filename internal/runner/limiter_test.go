package runner

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestStartLimiterUsesOneAggregateCadence(t *testing.T) {
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, err := NewStartLimiter(4, clock)
	if err != nil {
		t.Fatalf("NewStartLimiter() error = %v", err)
	}

	for range 4 {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatalf("Wait() error = %v", err)
		}
	}
	waits := clock.waitsCopy()
	for index, wait := range waits {
		want := time.Unix(100, 0).Add(time.Duration(index) * 250 * time.Millisecond)
		if !wait.Equal(want) {
			t.Fatalf("wait[%d] = %v, want %v", index, wait, want)
		}
	}
	if stats := limiter.Stats(); stats != (LimiterStats{Offered: 4, Achieved: 4}) {
		t.Fatalf("Stats() = %#v", stats)
	}
}

func TestStartLimiterIsSharedByConcurrentWorkers(t *testing.T) {
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, _ := NewStartLimiter(10, clock)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			if err := limiter.Wait(context.Background()); err != nil {
				t.Errorf("Wait() error = %v", err)
			}
		}()
	}
	workers.Wait()
	waits := clock.waitsCopy()
	if len(waits) != 8 {
		t.Fatalf("wait count = %d, want 8", len(waits))
	}
	seen := make(map[time.Time]bool)
	for _, wait := range waits {
		if seen[wait] {
			t.Fatalf("multiple workers received the same rate slot %v", wait)
		}
		seen[wait] = true
	}
}

func TestStartLimiterCancellationWhileWaiting(t *testing.T) {
	clock := &blockingClock{now: time.Unix(100, 0), entered: make(chan struct{}, 1)}
	limiter, _ := NewStartLimiter(1, clock)
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("first Wait() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- limiter.Wait(ctx) }()
	<-clock.entered
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Wait() error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait() did not return promptly after cancellation")
	}
	if stats := limiter.Stats(); stats != (LimiterStats{Offered: 2, Achieved: 1}) {
		t.Fatalf("Stats() = %#v", stats)
	}
}

func TestStartLimiterRejectsInvalidRate(t *testing.T) {
	if _, err := NewStartLimiter(0, nil); err == nil {
		t.Fatal("NewStartLimiter() error = nil")
	}
}

type recordingClock struct {
	mu    sync.Mutex
	now   time.Time
	waits []time.Time
}

func newRecordingClock(now time.Time) *recordingClock { return &recordingClock{now: now} }

func (c *recordingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *recordingClock) WaitUntil(ctx context.Context, deadline time.Time) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	c.mu.Lock()
	c.waits = append(c.waits, deadline)
	if deadline.After(c.now) {
		c.now = deadline
	}
	c.mu.Unlock()
	return nil
}

func (c *recordingClock) waitsCopy() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]time.Time(nil), c.waits...)
}

type blockingClock struct {
	mu      sync.Mutex
	now     time.Time
	waits   int
	entered chan struct{}
}

func (c *blockingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *blockingClock) WaitUntil(ctx context.Context, deadline time.Time) error {
	c.mu.Lock()
	c.waits++
	call := c.waits
	c.mu.Unlock()
	if call == 1 {
		return nil
	}
	c.entered <- struct{}{}
	<-ctx.Done()
	return ctx.Err()
}
