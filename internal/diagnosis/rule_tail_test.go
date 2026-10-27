package diagnosis

import (
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/runner"
)

func TestTailRuleMatrix(t *testing.T) {
	thresholds := DefaultThresholds()
	tests := []struct {
		name           string
		p50            time.Duration
		p95            time.Duration
		p99            time.Duration
		count          int
		completed      int
		wantFinding    bool
		wantConfidence Confidence
		wantLimitation bool
	}{
		{name: "positive", p50: 100 * time.Millisecond, p95: 250 * time.Millisecond,
			p99: 300 * time.Millisecond, count: 100, completed: 100, wantFinding: true, wantConfidence: ConfidenceHigh},
		{name: "exact boundaries", p50: 100 * time.Millisecond, p95: 200 * time.Millisecond,
			p99: 220 * time.Millisecond, count: 100, completed: 100, wantFinding: true, wantConfidence: ConfidenceHigh},
		{name: "below ratio", p50: 200 * time.Millisecond, p95: 350 * time.Millisecond,
			p99: 400 * time.Millisecond, count: 100, completed: 100},
		{name: "below gap", p50: 50 * time.Millisecond, p95: 100*time.Millisecond - time.Nanosecond,
			p99: 110 * time.Millisecond, count: 100, completed: 100},
		{name: "sparse", p50: 100 * time.Millisecond, p95: 250 * time.Millisecond,
			p99: 300 * time.Millisecond, count: 9, completed: 9, wantLimitation: true},
		{name: "contradictory percentiles", p50: 100 * time.Millisecond, p95: 250 * time.Millisecond,
			p99: 200 * time.Millisecond, count: 100, completed: 100, wantFinding: true, wantConfidence: ConfidenceLow},
		{name: "invalid outcomes", p50: 100 * time.Millisecond, p95: 250 * time.Millisecond,
			p99: 300 * time.Millisecond, count: 100, completed: 70, wantFinding: true, wantConfidence: ConfidenceMedium},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := (TailRule{}).Evaluate(tailSuite(test.p50, test.p95, test.p99, test.count, test.completed), thresholds)
			if (got.Finding != nil) != test.wantFinding {
				t.Fatalf("finding = %+v, limitations = %v", got.Finding, got.Limitations)
			}
			if got.Finding != nil && got.Finding.Confidence != test.wantConfidence {
				t.Fatalf("confidence = %s, want %s", got.Finding.Confidence, test.wantConfidence)
			}
			if test.wantLimitation && len(got.Limitations) == 0 {
				t.Fatal("expected limitation")
			}
		})
	}
}

func tailSuite(p50, p95, p99 time.Duration, count, completed int) Suite {
	plan := experiment.NewPlan(experiment.PlanInput{ID: "baseline", Kind: experiment.KindBaseline, Requests: 100})
	return Suite{Baseline: runner.ExperimentReport{
		Result: experiment.Result{Plan: plan},
		Aggregate: metrics.Aggregate{
			Latency:  metrics.LatencyStatistics{Available: true, Count: count, Median: p50, P95: p95, P99: p99},
			Outcomes: metrics.OutcomeCounts{Attempted: count, Completed: completed},
		},
	}}
}
