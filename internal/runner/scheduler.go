// Package runner schedules and executes bounded HTTP experiments.
package runner

import (
	"context"
	"errors"
	"math"
	"time"
)

// Arrival is one immutable intended request start.
type Arrival struct {
	Sequence     int
	PlannedStart time.Time
}

// StartedArrival pairs the intended start with the time execution could begin.
type StartedArrival struct {
	Arrival
	ActualStart time.Time
}

// SchedulingDelay is intentionally separate from HTTP service duration.
func (a StartedArrival) SchedulingDelay() time.Duration {
	if a.ActualStart.Before(a.PlannedStart) {
		return 0
	}
	return a.ActualStart.Sub(a.PlannedStart)
}

// Schedule is a bounded sequence generated from one monotonic time origin.
type Schedule struct {
	arrivals []Arrival
	interval time.Duration
}

// NewSchedule creates evenly spaced planned starts. The first arrival is at the
// origin; subsequent arrivals retain their original cadence even under delay.
func NewSchedule(origin time.Time, count int, rate float64) (Schedule, error) {
	if origin.IsZero() {
		return Schedule{}, errors.New("create arrival schedule: origin is required")
	}
	if count < 0 {
		return Schedule{}, errors.New("create arrival schedule: count must not be negative")
	}
	if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) {
		return Schedule{}, errors.New("create arrival schedule: rate must be finite and greater than zero")
	}
	interval := time.Duration(float64(time.Second) / rate)
	if interval <= 0 {
		return Schedule{}, errors.New("create arrival schedule: rate exceeds clock resolution")
	}
	arrivals := make([]Arrival, count)
	for sequence := range count {
		arrivals[sequence] = Arrival{
			Sequence:     sequence,
			PlannedStart: origin.Add(time.Duration(sequence) * interval),
		}
	}
	return Schedule{arrivals: arrivals, interval: interval}, nil
}

// Arrivals returns a defensive copy of the bounded schedule.
func (s Schedule) Arrivals() []Arrival { return append([]Arrival(nil), s.arrivals...) }

// Clock makes waiting deterministic in scheduler tests.
type Clock interface {
	Now() time.Time
	WaitUntil(context.Context, time.Time) error
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

func (realClock) WaitUntil(ctx context.Context, deadline time.Time) error {
	delay := time.Until(deadline)
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// DispatchStats describe offered work. A missed arrival is one dispatched at
// least one schedule interval late; delayed work remains visible and is never
// silently dropped.
type DispatchStats struct {
	Offered int
	Sent    int
	Missed  int
}

// Dispatch sends every arrival unless the context is cancelled. It owns and
// closes the output channel.
func Dispatch(ctx context.Context, schedule Schedule, output chan<- Arrival, clock Clock) (DispatchStats, error) {
	defer close(output)
	if clock == nil {
		clock = realClock{}
	}
	stats := DispatchStats{}
	for _, arrival := range schedule.arrivals {
		if err := clock.WaitUntil(ctx, arrival.PlannedStart); err != nil {
			return stats, err
		}
		stats.Offered++
		select {
		case output <- arrival:
			stats.Sent++
			if clock.Now().Sub(arrival.PlannedStart) >= schedule.interval {
				stats.Missed++
			}
		case <-ctx.Done():
			return stats, ctx.Err()
		}
	}
	return stats, nil
}

// Start stamps the actual worker start while preserving the planned arrival.
func Start(arrival Arrival, actual time.Time) StartedArrival {
	return StartedArrival{Arrival: arrival, ActualStart: actual}
}
