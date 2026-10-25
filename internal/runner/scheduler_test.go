package runner

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestNewSchedulePreservesPlannedCadence(t *testing.T) {
	origin := time.Unix(100, 0)
	schedule, err := NewSchedule(origin, 4, 2)
	if err != nil {
		t.Fatalf("NewSchedule() error = %v", err)
	}
	arrivals := schedule.Arrivals()
	for index, arrival := range arrivals {
		want := origin.Add(time.Duration(index) * 500 * time.Millisecond)
		if !arrival.PlannedStart.Equal(want) || arrival.Sequence != index {
			t.Fatalf("arrival[%d] = %#v, want %v", index, arrival, want)
		}
	}
	arrivals[0].Sequence = 99
	if schedule.Arrivals()[0].Sequence != 0 {
		t.Fatal("schedule changed through returned arrivals")
	}
}

func TestDispatchCountsLateArrivalsWithoutDropping(t *testing.T) {
	origin := time.Unix(100, 0)
	schedule, _ := NewSchedule(origin, 3, 2)
	clock := &fakeClock{now: origin, dispatchLag: time.Second}
	output := make(chan Arrival, 3)

	stats, err := Dispatch(context.Background(), schedule, output, clock)
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if stats.Offered != 3 || stats.Sent != 3 || stats.Missed != 3 {
		t.Fatalf("Dispatch() stats = %#v", stats)
	}
	var count int
	for range output {
		count++
	}
	if count != 3 {
		t.Fatalf("dispatched %d arrivals, want 3", count)
	}
}

func TestDispatchCancellationClosesOutput(t *testing.T) {
	origin := time.Unix(100, 0)
	schedule, _ := NewSchedule(origin, 2, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	output := make(chan Arrival)

	stats, err := Dispatch(ctx, schedule, output, &fakeClock{now: origin})
	if !errors.Is(err, context.Canceled) || stats.Sent != 0 {
		t.Fatalf("Dispatch() = (%#v, %v), want cancellation before send", stats, err)
	}
	if _, open := <-output; open {
		t.Fatal("output remained open after cancellation")
	}
}

func TestStartedArrivalSeparatesSchedulingDelay(t *testing.T) {
	planned := time.Unix(100, 0)
	started := Start(Arrival{Sequence: 2, PlannedStart: planned}, planned.Add(35*time.Millisecond))
	if started.SchedulingDelay() != 35*time.Millisecond {
		t.Fatalf("SchedulingDelay() = %v", started.SchedulingDelay())
	}
}

func TestNewScheduleRejectsInvalidInputs(t *testing.T) {
	for _, test := range []struct {
		name   string
		origin time.Time
		count  int
		rate   float64
	}{
		{name: "origin", count: 1, rate: 1},
		{name: "count", origin: time.Now(), count: -1, rate: 1},
		{name: "rate", origin: time.Now(), count: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewSchedule(test.origin, test.count, test.rate); err == nil {
				t.Fatal("NewSchedule() error = nil")
			}
		})
	}
}

type fakeClock struct {
	now         time.Time
	dispatchLag time.Duration
}

func (c *fakeClock) Now() time.Time { return c.now.Add(c.dispatchLag) }

func (c *fakeClock) WaitUntil(ctx context.Context, deadline time.Time) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		c.now = deadline
		return nil
	}
}
