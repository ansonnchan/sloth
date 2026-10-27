package diagnosis

import (
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/runner"
)

func TestFirstByteRuleMatrix(t *testing.T) {
	thresholds := DefaultThresholds()
	tests := []struct {
		name           string
		total          time.Duration
		firstByte      time.Duration
		count          int
		phaseCount     int
		interrupted    bool
		wantFinding    bool
		wantConfidence Confidence
		wantLimitation bool
	}{
		{
			name: "positive", total: 200 * time.Millisecond, firstByte: 150 * time.Millisecond,
			count: 100, phaseCount: 100, wantFinding: true, wantConfidence: ConfidenceHigh,
		},
		{
			name: "absolute boundary", total: 160 * time.Millisecond, firstByte: thresholds.FirstByteMinimum,
			count: 100, phaseCount: 100, wantFinding: true, wantConfidence: ConfidenceHigh,
		},
		{
			name: "share boundary", total: 200 * time.Millisecond,
			firstByte: time.Duration(float64(200*time.Millisecond) * thresholds.FirstByteShare),
			count:     100, phaseCount: 100, wantFinding: true, wantConfidence: ConfidenceHigh,
		},
		{
			name: "below absolute threshold", total: 150 * time.Millisecond,
			firstByte: thresholds.FirstByteMinimum - time.Nanosecond,
			count:     100, phaseCount: 100,
		},
		{
			name: "below share threshold", total: 200 * time.Millisecond,
			firstByte: 119 * time.Millisecond, count: 100, phaseCount: 100,
		},
		{
			name: "sparse", total: 200 * time.Millisecond, firstByte: 150 * time.Millisecond,
			count: 9, phaseCount: 9, wantLimitation: true,
		},
		{
			name: "contradictory", total: 100 * time.Millisecond, firstByte: 120 * time.Millisecond,
			count: 100, phaseCount: 100, wantFinding: true, wantConfidence: ConfidenceLow,
		},
		{
			name: "partial phase availability", total: 200 * time.Millisecond, firstByte: 150 * time.Millisecond,
			count: 100, phaseCount: 80, wantFinding: true, wantConfidence: ConfidenceLow,
		},
		{
			name: "interrupted", total: 200 * time.Millisecond, firstByte: 150 * time.Millisecond,
			count: 100, phaseCount: 100, interrupted: true, wantFinding: true, wantConfidence: ConfidenceLow,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			suite := firstByteSuite(100, test.count, test.phaseCount, test.total, test.firstByte, test.interrupted)
			got := (FirstByteRule{}).Evaluate(suite, thresholds)
			if (got.Finding != nil) != test.wantFinding {
				t.Fatalf("finding = %+v, limitations = %v", got.Finding, got.Limitations)
			}
			if got.Finding != nil && got.Finding.Confidence != test.wantConfidence {
				t.Fatalf("confidence = %s, want %s", got.Finding.Confidence, test.wantConfidence)
			}
			if test.wantLimitation && len(got.Limitations) == 0 {
				t.Fatal("expected an evidence limitation")
			}
		})
	}
}

func TestFirstByteRuleSuppressesUnavailableTiming(t *testing.T) {
	suite := firstByteSuite(100, 100, 0, 200*time.Millisecond, 0, false)
	suite.Baseline.Aggregate.Phases["first_byte_wait"] = metrics.LatencyStatistics{}
	got := (FirstByteRule{}).Evaluate(suite, DefaultThresholds())
	if got.Finding != nil || len(got.Limitations) == 0 {
		t.Fatalf("evaluation = %+v, want an unavailable-evidence limitation", got)
	}
}

func firstByteSuite(expected, count, phaseCount int, total, firstByte time.Duration, interrupted bool) Suite {
	plan := experiment.NewPlan(experiment.PlanInput{ID: "baseline", Kind: experiment.KindBaseline, Requests: expected})
	return Suite{Baseline: runner.ExperimentReport{
		Result: experiment.Result{Plan: plan},
		Aggregate: metrics.Aggregate{
			Latency: metrics.LatencyStatistics{Available: true, Count: count, Median: total},
			Phases: map[string]metrics.LatencyStatistics{
				"first_byte_wait": {Available: phaseCount > 0, Count: phaseCount, Median: firstByte},
			},
			Interrupted: interrupted,
		},
	}}
}
