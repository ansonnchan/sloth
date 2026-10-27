package compare

import (
	"math"
	"testing"

	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/report"
)

func TestCalculateSafeTypedDeltas(t *testing.T) {
	before := deltaExperiment(100, 200, 0, 10, metrics.OutcomeCounts{Attempted: 100, Successful: 88, HTTPError: 10, Timeout: 2})
	after := deltaExperiment(80, 250, 300, 12, metrics.OutcomeCounts{Attempted: 100, Successful: 92, HTTPError: 5, Timeout: 3})
	compatibility := Compatibility{
		Mode: ModeGate, DescriptiveEligible: true, GateEligible: true,
		Pairs: []ExperimentPair{{ID: "baseline", Before: before, After: after}},
	}
	result, err := Calculate(compatibility)
	if err != nil {
		t.Fatal(err)
	}
	metricsByName := indexMetrics(result.Experiments[0].Metrics)
	assertDelta(t, metricsByName["latency.p50_ms"], -20, -20, LowerIsBetter)
	assertDelta(t, metricsByName["latency.p95_ms"], 50, 25, LowerIsBetter)
	assertDelta(t, metricsByName["throughput.completed_per_second"], 2, 20, HigherIsBetter)
	assertDelta(t, metricsByName["outcomes.errors"], -5, -50, LowerIsBetter)
	assertDelta(t, metricsByName["outcomes.timeouts"], 1, 50, LowerIsBetter)
	assertDelta(t, metricsByName["phases.first_byte_wait.p50_ms"], -10, -20, LowerIsBetter)
	if metric := metricsByName["latency.p99_ms"]; metric.Before != nil || metric.Absolute != nil || metric.PercentChange != nil {
		t.Fatalf("missing p99 became numeric: %+v", metric)
	}
}

func TestPercentageUndefinedForZeroBaselineWithoutLosingAbsoluteChange(t *testing.T) {
	metric := newDelta("throughput", "requests/s", HigherIsBetter, floatPointer(0), floatPointer(5))
	if metric.Absolute == nil || *metric.Absolute != 5 || metric.PercentChange != nil {
		t.Fatalf("delta = %+v", metric)
	}
}

func TestCalculatePreservesRawPrecision(t *testing.T) {
	metric := newDelta("latency", "ms", LowerIsBetter, floatPointer(1), floatPointer(1.005))
	if metric.Absolute == nil || math.Abs(*metric.Absolute-0.005) > 1e-12 || metric.PercentChange == nil || math.Abs(*metric.PercentChange-.5) > 1e-12 {
		t.Fatalf("delta = %+v", metric)
	}
}

func TestCalculateRejectsIneligibleEvidenceBeforeDeltas(t *testing.T) {
	for _, compatibility := range []Compatibility{
		{Mode: ModeDescriptive},
		{Mode: ModeGate, DescriptiveEligible: true, GateEligible: false, Pairs: []ExperimentPair{{ID: "baseline"}}},
	} {
		if _, err := Calculate(compatibility); err == nil {
			t.Fatalf("Calculate(%+v) succeeded", compatibility)
		}
	}
}

func deltaExperiment(p50, p95, p99, throughput float64, outcomes metrics.OutcomeCounts) report.Experiment {
	phaseP50 := 50.0
	phaseP95 := 75.0
	if p50 == 80 {
		phaseP50 = 40
		phaseP95 = 60
	}
	statistics := report.Statistics{Count: outcomes.Attempted, P50Millis: floatPointer(p50), P95Millis: floatPointer(p95)}
	if p99 != 0 {
		statistics.P99Millis = floatPointer(p99)
	}
	return report.Experiment{
		ID: "baseline",
		Aggregate: report.Aggregate{
			Outcomes: outcomes, Latency: statistics,
			Phases:      map[string]report.Statistics{"first_byte_wait": {Count: outcomes.Attempted, P50Millis: &phaseP50, P95Millis: &phaseP95}},
			Rates:       report.Rates{CompletedPerSecond: throughput},
			Connections: metrics.ConnectionMetrics{Observed: outcomes.Attempted, New: 10, Reused: outcomes.Attempted - 10},
		},
	}
}

func indexMetrics(values []MetricDelta) map[string]MetricDelta {
	result := make(map[string]MetricDelta, len(values))
	for _, value := range values {
		result[value.Name] = value
	}
	return result
}

func assertDelta(t *testing.T, value MetricDelta, absolute, percent float64, direction Direction) {
	t.Helper()
	if value.Absolute == nil || math.Abs(*value.Absolute-absolute) > 1e-9 || value.PercentChange == nil || math.Abs(*value.PercentChange-percent) > 1e-9 || value.Direction != direction {
		t.Fatalf("delta = %+v, want absolute=%g percent=%g direction=%s", value, absolute, percent, direction)
	}
}
