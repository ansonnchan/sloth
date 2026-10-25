package runner

import (
	"context"
	"errors"
	"math"
	"sync"
	"time"
)

// StartLimiter owns one aggregate request-start cadence for an entire suite.
// Sharing one instance across warm-ups, arms, and workers prevents concurrency
// from multiplying the configured traffic rate.
type StartLimiter struct {
	mu       sync.Mutex
	clock    Clock
	interval time.Duration
	next     time.Time
	offered  int
	achieved int
}

// NewStartLimiter creates a no-burst limiter. The first acquisition is
// immediate and every later acquisition is separated by the configured rate.
func NewStartLimiter(rate float64, clock Clock) (*StartLimiter, error) {
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return nil, errors.New("create start limiter: rate must be finite and greater than zero")
	}
	interval := time.Duration(float64(time.Second) / rate)
	if interval <= 0 {
		return nil, errors.New("create start limiter: rate exceeds clock resolution")
	}
	if clock == nil {
		clock = realClock{}
	}
	return &StartLimiter{clock: clock, interval: interval}, nil
}

// Wait reserves and waits for the next suite-wide start slot. A cancelled wait
// remains offered but is not counted as achieved.
func (l *StartLimiter) Wait(ctx context.Context) error {
	l.mu.Lock()
	now := l.clock.Now()
	slot := now
	if l.next.After(slot) {
		slot = l.next
	}
	l.next = slot.Add(l.interval)
	l.offered++
	l.mu.Unlock()

	if err := l.clock.WaitUntil(ctx, slot); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	l.mu.Lock()
	l.achieved++
	l.mu.Unlock()
	return nil
}

// LimiterStats reports aggregate suite starts, including cancelled offers.
type LimiterStats struct {
	Offered  int
	Achieved int
}

func (l *StartLimiter) Stats() LimiterStats {
	l.mu.Lock()
	defer l.mu.Unlock()
	return LimiterStats{Offered: l.offered, Achieved: l.achieved}
}
