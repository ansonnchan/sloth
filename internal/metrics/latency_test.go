package metrics

import (
	"math"
	"testing"
	"time"
)

func TestCalculateLatencyGoldenCase(t *testing.T) {
	values := []time.Duration{40, 10, 50, 20, 30}
	got, err := CalculateLatency(values)
	if err != nil {
		t.Fatalf("CalculateLatency() error = %v", err)
	}
	want := LatencyStatistics{
		Available: true, Count: 5, Min: 10, Max: 50, Mean: 30,
		Median: 30, P90: 46, P95: 48, P99: 50,
		PopulationStdDev: 14,
	}
	if got != want {
		t.Fatalf("CalculateLatency() = %#v, want %#v", got, want)
	}
	if values[0] != 40 {
		t.Fatal("CalculateLatency() sorted the caller's input")
	}
}

func TestCalculateLatencyEmptyAndSingleton(t *testing.T) {
	empty, err := CalculateLatency(nil)
	if err != nil || empty.Available || empty.Count != 0 {
		t.Fatalf("empty statistics = (%#v, %v)", empty, err)
	}

	one, err := CalculateLatency([]time.Duration{17 * time.Millisecond})
	if err != nil || !one.Available || one.Mean != 17*time.Millisecond || one.P99 != 17*time.Millisecond || one.PopulationStdDev != 0 {
		t.Fatalf("singleton statistics = (%#v, %v)", one, err)
	}
}

func TestCalculateLatencyDuplicatesAndOrdering(t *testing.T) {
	got, err := CalculateLatency([]time.Duration{5, 1, 5, 1})
	if err != nil {
		t.Fatalf("CalculateLatency() error = %v", err)
	}
	if got.Min != 1 || got.Max != 5 || got.Median != 3 || got.Mean != 3 || got.PopulationStdDev != 2 {
		t.Fatalf("duplicate statistics = %#v", got)
	}
}

func TestCalculateLatencyRejectsNegativeDuration(t *testing.T) {
	if _, err := CalculateLatency([]time.Duration{time.Millisecond, -1}); err == nil {
		t.Fatal("CalculateLatency() error = nil")
	}
}

func TestCalculateLatencyHandlesExtremeDurationsWithoutOverflow(t *testing.T) {
	got, err := CalculateLatency([]time.Duration{0, time.Duration(math.MaxInt64)})
	if err != nil {
		t.Fatalf("CalculateLatency() error = %v", err)
	}
	if got.Min != 0 || got.Max != time.Duration(math.MaxInt64) || got.Mean <= 0 || got.P99 <= 0 {
		t.Fatalf("extreme statistics = %#v", got)
	}
}

func TestPercentileUsesNMinusOneLinearInterpolation(t *testing.T) {
	values := []time.Duration{0, 10, 20, 30}
	got, err := CalculateLatency(values)
	if err != nil {
		t.Fatalf("CalculateLatency() error = %v", err)
	}
	if got.Median != 15 || got.P90 != 27 || got.P95 != 29 || got.P99 != 30 {
		t.Fatalf("interpolated percentiles = %#v", got)
	}
}
