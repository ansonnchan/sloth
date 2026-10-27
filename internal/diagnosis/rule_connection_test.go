package diagnosis

import (
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/runner"
)

func TestConnectionRuleMatrix(t *testing.T) {
	thresholds := DefaultThresholds()
	tests := []struct {
		name           string
		fresh          time.Duration
		reused         time.Duration
		freshP95       time.Duration
		reusedP95      time.Duration
		count          int
		sufficient     bool
		limitations    []string
		wantFinding    bool
		wantConfidence Confidence
		wantLimitation bool
	}{
		{name: "positive", fresh: 100 * time.Millisecond, reused: 50 * time.Millisecond,
			freshP95: 120 * time.Millisecond, reusedP95: 70 * time.Millisecond, count: 100,
			sufficient: true, wantFinding: true, wantConfidence: ConfidenceHigh},
		{name: "exact boundaries", fresh: 100 * time.Millisecond, reused: 80 * time.Millisecond,
			freshP95: 120 * time.Millisecond, reusedP95: 90 * time.Millisecond, count: 100,
			sufficient: true, wantFinding: true, wantConfidence: ConfidenceHigh},
		{name: "below absolute", fresh: 100 * time.Millisecond, reused: 80*time.Millisecond + time.Nanosecond,
			freshP95: 120 * time.Millisecond, reusedP95: 90 * time.Millisecond, count: 100, sufficient: true},
		{name: "below ratio", fresh: 200 * time.Millisecond, reused: 170 * time.Millisecond,
			freshP95: 220 * time.Millisecond, reusedP95: 190 * time.Millisecond, count: 100, sufficient: true},
		{name: "sparse", fresh: 100 * time.Millisecond, reused: 50 * time.Millisecond,
			freshP95: 120 * time.Millisecond, reusedP95: 70 * time.Millisecond, count: 4,
			sufficient: true, wantLimitation: true},
		{name: "unobserved arms", fresh: 100 * time.Millisecond, reused: 50 * time.Millisecond,
			count: 100, wantLimitation: true},
		{name: "contradictory tail", fresh: 100 * time.Millisecond, reused: 50 * time.Millisecond,
			freshP95: 120 * time.Millisecond, reusedP95: 140 * time.Millisecond, count: 100,
			sufficient: true, wantFinding: true, wantConfidence: ConfidenceLow},
		{name: "http2 ambiguity", fresh: 100 * time.Millisecond, reused: 50 * time.Millisecond,
			freshP95: 120 * time.Millisecond, reusedP95: 70 * time.Millisecond, count: 100,
			sufficient: true, limitations: []string{"HTTP/2 connection evidence may include multiplexed streams"},
			wantFinding: true, wantConfidence: ConfidenceMedium},
		{name: "proxy ambiguity", fresh: 100 * time.Millisecond, reused: 50 * time.Millisecond,
			freshP95: 120 * time.Millisecond, reusedP95: 70 * time.Millisecond, count: 100,
			sufficient: true, limitations: []string{"proxy tunnel connection state is ambiguous"},
			wantFinding: true, wantConfidence: ConfidenceMedium},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := (ConnectionRule{}).Evaluate(connectionSuite(test.fresh, test.reused, test.freshP95,
				test.reusedP95, test.count, test.sufficient, test.limitations), thresholds)
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

func connectionSuite(fresh, reused, freshP95, reusedP95 time.Duration, count int, sufficient bool, limitations []string) Suite {
	plan := experiment.NewPlan(experiment.PlanInput{ID: "connection", Kind: experiment.KindFreshConnection, Requests: 100})
	arm := func(median, p95 time.Duration, newConnections, reusedConnections int) runner.ExperimentReport {
		return runner.ExperimentReport{
			Result: experiment.Result{Plan: plan},
			Aggregate: metrics.Aggregate{
				Latency: metrics.LatencyStatistics{Available: true, Count: count, Median: median, P95: p95},
				Phases: map[string]metrics.LatencyStatistics{
					"connect": {Available: true, Count: count, Median: 15 * time.Millisecond},
					"tls":     {Available: true, Count: count, Median: 20 * time.Millisecond},
				},
				Connections: metrics.ConnectionMetrics{Observed: count, New: newConnections, Reused: reusedConnections},
			},
		}
	}
	return Suite{Connection: runner.ConnectionReport{
		Fresh: arm(fresh, freshP95, count, 0), Reused: arm(reused, reusedP95, 0, count),
		Sufficient: sufficient, Limitations: limitations,
	}}
}
