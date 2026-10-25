package runner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

func TestPoolCancellationBeforeStartExecutesNothing(t *testing.T) {
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, _ := NewStartLimiter(1000, clock)
	budget, _ := NewRequestBudget(10)
	var calls atomic.Int32
	pool, _ := NewPool(4, ExecutorFunc(func(context.Context, StartedArrival) (experiment.Sample, error) {
		calls.Add(1)
		return experiment.Sample{}, nil
	}), clock)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	report, err := pool.Run(ctx, testArrivals(10), limiter, budget)
	if !errors.Is(err, context.Canceled) || calls.Load() != 0 || report.Started != 0 {
		t.Fatalf("Run() = (%#v, %v), calls=%d", report, err, calls.Load())
	}
}

func TestPoolCancellationDuringLimiterWaitExecutesNothing(t *testing.T) {
	clock := &blockingClock{now: time.Unix(100, 0), entered: make(chan struct{}, 1)}
	limiter, _ := NewStartLimiter(1, clock)
	if err := limiter.Wait(context.Background()); err != nil {
		t.Fatalf("prime limiter: %v", err)
	}
	budget, _ := NewRequestBudget(1)
	var calls atomic.Int32
	pool, _ := NewPool(1, ExecutorFunc(func(context.Context, StartedArrival) (experiment.Sample, error) {
		calls.Add(1)
		return experiment.Sample{}, nil
	}), clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var report PoolReport
	var runErr error
	go func() {
		report, runErr = pool.Run(ctx, testArrivals(1), limiter, budget)
		close(done)
	}()
	<-clock.entered
	cancel()
	waitForTestCompletion(t, done)
	if !errors.Is(runErr, context.Canceled) || calls.Load() != 0 || report.Started != 0 {
		t.Fatalf("Run() = (%#v, %v), calls=%d", report, runErr, calls.Load())
	}
}

func TestPoolCancellationInFlightDrainsCancelledSample(t *testing.T) {
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, _ := NewStartLimiter(1000, clock)
	budget, _ := NewRequestBudget(4)
	entered := make(chan struct{}, 1)
	pool, _ := NewPool(1, ExecutorFunc(func(ctx context.Context, arrival StartedArrival) (experiment.Sample, error) {
		entered <- struct{}{}
		<-ctx.Done()
		return experiment.Sample{
			Sequence: arrival.Sequence,
			Outcome:  httpmeasure.Outcome{Category: httpmeasure.OutcomeCancelled},
		}, nil
	}), clock)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	var report PoolReport
	var runErr error
	go func() {
		report, runErr = pool.Run(ctx, testArrivals(4), limiter, budget)
		close(done)
	}()
	<-entered
	cancel()
	waitForTestCompletion(t, done)
	if !errors.Is(runErr, context.Canceled) || report.Started != 1 || len(report.Samples) != 1 || report.Samples[0].Outcome.Category != httpmeasure.OutcomeCancelled {
		t.Fatalf("Run() = (%#v, %v)", report, runErr)
	}
}

func TestPoolCancellationDuringCollectionDoesNotDeadlock(t *testing.T) {
	for iteration := range 100 {
		clock := newRecordingClock(time.Unix(int64(100+iteration), 0))
		limiter, _ := NewStartLimiter(1_000_000, clock)
		budget, _ := NewRequestBudget(32)
		ctx, cancel := context.WithCancel(context.Background())
		var calls atomic.Int32
		pool, _ := NewPool(8, ExecutorFunc(func(_ context.Context, arrival StartedArrival) (experiment.Sample, error) {
			if calls.Add(1) == 8 {
				cancel()
			}
			return experiment.Sample{Sequence: arrival.Sequence}, nil
		}), clock)
		done := make(chan struct{})
		go func() {
			_, _ = pool.Run(ctx, testArrivals(32), limiter, budget)
			close(done)
		}()
		waitForTestCompletion(t, done)
	}
}

func TestPoolBoundsSlowFailingExecutors(t *testing.T) {
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, _ := NewStartLimiter(1_000_000, clock)
	budget, _ := NewRequestBudget(10)
	var calls atomic.Int32
	var active atomic.Int32
	var peak atomic.Int32
	wantErr := errors.New("synthetic executor failure")
	pool, _ := NewPool(4, ExecutorFunc(func(_ context.Context, arrival StartedArrival) (experiment.Sample, error) {
		current := active.Add(1)
		updatePeak(&peak, current)
		defer active.Add(-1)
		time.Sleep(time.Millisecond)
		if calls.Add(1) == 7 {
			return experiment.Sample{Sequence: arrival.Sequence}, wantErr
		}
		return experiment.Sample{Sequence: arrival.Sequence}, nil
	}), clock)

	report, err := pool.Run(context.Background(), testArrivals(100), limiter, budget)
	if !errors.Is(err, wantErr) || report.StopReason != experiment.StopExecutionFailure {
		t.Fatalf("Run() = (%#v, %v)", report, err)
	}
	if peak.Load() > 4 || budget.Used() > 10 {
		t.Fatalf("bounds exceeded: peak=%d budget=%d", peak.Load(), budget.Used())
	}
}

func waitForTestCompletion(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runner lifecycle did not terminate")
	}
}
