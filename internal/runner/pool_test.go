package runner

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/experiment"
)

func TestPoolBoundsWorkersAndOrdersResults(t *testing.T) {
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, _ := NewStartLimiter(1000, clock)
	budget, _ := NewRequestBudget(12)
	var current atomic.Int32
	var peak atomic.Int32
	executor := ExecutorFunc(func(_ context.Context, arrival StartedArrival) (experiment.Sample, error) {
		active := current.Add(1)
		for {
			old := peak.Load()
			if active <= old || peak.CompareAndSwap(old, active) {
				break
			}
		}
		time.Sleep(time.Millisecond)
		current.Add(-1)
		return experiment.Sample{Sequence: arrival.Sequence}, nil
	})
	pool, _ := NewPool(3, executor, clock)

	report, err := pool.Run(context.Background(), testArrivals(12), limiter, budget)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if peak.Load() > 3 || report.Started != 12 || len(report.Samples) != 12 {
		t.Fatalf("peak=%d report=%#v", peak.Load(), report)
	}
	for sequence, sample := range report.Samples {
		if sample.Sequence != sequence || sample.PlannedStart.IsZero() || sample.ActualStart.IsZero() {
			t.Fatalf("sample[%d] = %#v", sequence, sample)
		}
	}
}

func TestPoolStopsAtSharedBudget(t *testing.T) {
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, _ := NewStartLimiter(1000, clock)
	budget, _ := NewRequestBudget(4)
	pool, _ := NewPool(2, sampleExecutor(), clock)

	report, err := pool.Run(context.Background(), testArrivals(10), limiter, budget)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Started != 4 || budget.Used() != 4 || report.StopReason != experiment.StopBudgetExhausted {
		t.Fatalf("budget report = %#v, used=%d", report, budget.Used())
	}
}

func TestPoolCancellationDrainsStartedResults(t *testing.T) {
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, _ := NewStartLimiter(1000, clock)
	budget, _ := NewRequestBudget(10)
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	pool, _ := NewPool(2, ExecutorFunc(func(_ context.Context, arrival StartedArrival) (experiment.Sample, error) {
		once.Do(cancel)
		return experiment.Sample{Sequence: arrival.Sequence}, nil
	}), clock)

	report, err := pool.Run(ctx, testArrivals(10), limiter, budget)
	if !errors.Is(err, context.Canceled) || report.StopReason != experiment.StopCancelled {
		t.Fatalf("Run() = (%#v, %v), want cancelled partial result", report, err)
	}
	if report.Started == 0 || report.Started > 2 {
		t.Fatalf("started %d requests after cancellation boundary", report.Started)
	}
}

func TestPoolExecutionFailureStopsProducer(t *testing.T) {
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, _ := NewStartLimiter(1000, clock)
	budget, _ := NewRequestBudget(10)
	wantErr := errors.New("executor failed")
	pool, _ := NewPool(1, ExecutorFunc(func(_ context.Context, arrival StartedArrival) (experiment.Sample, error) {
		return experiment.Sample{Sequence: arrival.Sequence}, wantErr
	}), clock)

	report, err := pool.Run(context.Background(), testArrivals(10), limiter, budget)
	if !errors.Is(err, wantErr) || report.Started != 1 || report.StopReason != experiment.StopExecutionFailure {
		t.Fatalf("Run() = (%#v, %v)", report, err)
	}
}

func TestPoolValidatesDependencies(t *testing.T) {
	if _, err := NewPool(0, sampleExecutor(), nil); err == nil {
		t.Fatal("NewPool(0) error = nil")
	}
	if _, err := NewPool(1, nil, nil); err == nil {
		t.Fatal("NewPool(nil executor) error = nil")
	}
	if _, err := NewRequestBudget(-1); err == nil {
		t.Fatal("NewRequestBudget(-1) error = nil")
	}
}

func sampleExecutor() Executor {
	return ExecutorFunc(func(_ context.Context, arrival StartedArrival) (experiment.Sample, error) {
		return experiment.Sample{Sequence: arrival.Sequence}, nil
	})
}

func testArrivals(count int) []Arrival {
	arrivals := make([]Arrival, count)
	start := time.Unix(100, 0)
	for index := range count {
		arrivals[index] = Arrival{Sequence: index, PlannedStart: start.Add(time.Duration(index) * time.Millisecond)}
	}
	return arrivals
}
