package runner

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/experiment"
)

func BenchmarkWorkerPoolRenderingFree(b *testing.B) {
	for _, count := range []int{1_000, 10_000} {
		b.Run(fmt.Sprintf("requests-%d", count), func(b *testing.B) {
			arrivals := testArrivals(count)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				clock := newRecordingClock(time.Unix(100, 0))
				limiter, _ := NewStartLimiter(100, clock)
				budget, _ := NewRequestBudget(count)
				pool, _ := NewPool(50, ExecutorFunc(func(_ context.Context, arrival StartedArrival) (experiment.Sample, error) {
					return experiment.Sample{Sequence: arrival.Sequence}, nil
				}), clock)
				if report, err := pool.Run(context.Background(), arrivals, limiter, budget); err != nil || report.Started != count {
					b.Fatalf("Run() = (%d starts, %v)", report.Started, err)
				}
			}
		})
	}
}

// This deterministic test exercises the complete normal suite request ceiling
// without sleeping for its 100-second real-time cadence.
func TestSchedulerAccuracyAtNormalSuiteCeiling(t *testing.T) {
	const totalRequests = 10_000
	const rate = 100
	start := time.Unix(100, 0)
	clock := newRecordingClock(start)
	limiter, err := NewStartLimiter(rate, clock)
	if err != nil {
		t.Fatal(err)
	}
	for range totalRequests {
		if err := limiter.Wait(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	waits := clock.waitsCopy()
	if len(waits) != totalRequests {
		t.Fatalf("wait count = %d, want %d", len(waits), totalRequests)
	}
	wantLast := start.Add(time.Duration(totalRequests-1) * 10 * time.Millisecond)
	if !waits[0].Equal(start) || !waits[len(waits)-1].Equal(wantLast) {
		t.Fatalf("schedule bounds = %v..%v, want %v..%v", waits[0], waits[len(waits)-1], start, wantLast)
	}
	if stats := limiter.Stats(); stats != (LimiterStats{Offered: totalRequests, Achieved: totalRequests}) {
		t.Fatalf("Stats() = %#v", stats)
	}
}

func TestPoolBoundsResultsAtNormalSuiteCeiling(t *testing.T) {
	const totalRequests = 10_000
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, _ := NewStartLimiter(100, clock)
	budget, _ := NewRequestBudget(totalRequests)
	pool, _ := NewPool(50, sampleExecutor(), clock)

	report, err := pool.Run(context.Background(), testArrivals(totalRequests), limiter, budget)
	if err != nil {
		t.Fatal(err)
	}
	if report.Started != totalRequests || len(report.Samples) != totalRequests || cap(report.Samples) > 2*totalRequests {
		t.Fatalf("result storage = starts:%d len:%d cap:%d", report.Started, len(report.Samples), cap(report.Samples))
	}
}
