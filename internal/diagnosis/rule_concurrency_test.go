package diagnosis

import (
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/runner"
)

func TestConcurrencyRuleMatrix(t *testing.T) {
	tests := []struct {
		name           string
		reference      runner.ExperimentReport
		loaded         runner.ExperimentReport
		wantFinding    bool
		wantSeverity   Severity
		wantConfidence Confidence
		wantLimitation bool
	}{
		{name: "latency sensitivity", reference: concurrencyStage(5, 100, 100, 100, 150, 1, 0),
			loaded: concurrencyStage(10, 100, 100, 100, 240, 1, 0), wantFinding: true,
			wantSeverity: SeverityWarning, wantConfidence: ConfidenceHigh},
		{name: "exact load and latency boundaries", reference: concurrencyStage(4, 100, 100, 100, 100, 1, 0),
			loaded: concurrencyStage(5, 100, 100, 100, 150, 1, 0), wantFinding: true,
			wantSeverity: SeverityWarning, wantConfidence: ConfidenceHigh},
		{name: "unchanged achieved load", reference: concurrencyStage(5, 100, 100, 100, 100, 1, 0),
			loaded: concurrencyStage(6, 100, 100, 100, 200, 1, 0), wantLimitation: true},
		{name: "normal scaling", reference: concurrencyStage(5, 100, 100, 100, 100, 1, 0),
			loaded: concurrencyStage(10, 100, 100, 100, 120, 1, 0)},
		{name: "error sensitivity", reference: concurrencyStage(5, 100, 100, 100, 100, 1, 0),
			loaded: concurrencyStage(10, 100, 98, 100, 120, 1, 2), wantFinding: true,
			wantSeverity: SeverityWarning, wantConfidence: ConfidenceHigh},
		{name: "client scheduling pressure", reference: concurrencyStage(5, 100, 100, 100, 100, 1, 0),
			loaded: concurrencyStage(10, 100, 100, 100, 180, 50, 0), wantFinding: true,
			wantSeverity: SeverityInfo, wantConfidence: ConfidenceMedium},
		{name: "sparse", reference: concurrencyStage(5, 9, 9, 100, 100, 1, 0),
			loaded: concurrencyStage(10, 9, 9, 100, 200, 1, 0), wantLimitation: true},
		{name: "contradictory percentiles", reference: concurrencyStage(5, 100, 100, 100, 100, 1, 0),
			loaded: concurrencyStageWithMedian(10, 100, 100, 50*time.Millisecond, 200*time.Millisecond, 1, 0), wantFinding: true,
			wantSeverity: SeverityWarning, wantConfidence: ConfidenceLow},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := (ConcurrencyRule{}).Evaluate(Suite{Concurrency: []runner.ExperimentReport{test.reference, test.loaded}}, DefaultThresholds())
			if (got.Finding != nil) != test.wantFinding {
				t.Fatalf("finding = %+v, limitations = %v", got.Finding, got.Limitations)
			}
			if got.Finding != nil && (got.Finding.Severity != test.wantSeverity || got.Finding.Confidence != test.wantConfidence) {
				t.Fatalf("finding severity/confidence = %s/%s, want %s/%s", got.Finding.Severity,
					got.Finding.Confidence, test.wantSeverity, test.wantConfidence)
			}
			if test.wantLimitation && len(got.Limitations) == 0 {
				t.Fatal("expected limitation")
			}
		})
	}
}

func TestConcurrencyRuleRequiresTwoStages(t *testing.T) {
	got := (ConcurrencyRule{}).Evaluate(Suite{}, DefaultThresholds())
	if got.Finding != nil || len(got.Limitations) == 0 {
		t.Fatalf("evaluation = %+v, want limitation", got)
	}
}

func concurrencyStage(rate float64, count, successes int, p50, p95, schedulingMS int, errors int) runner.ExperimentReport {
	return concurrencyStageWithMedian(rate, count, successes, time.Duration(p50)*time.Millisecond,
		time.Duration(p95)*time.Millisecond, schedulingMS, errors)
}

func concurrencyStageWithMedian(rate float64, count, successes int, p50, p95 time.Duration, schedulingMS int, errors int) runner.ExperimentReport {
	plan := experiment.NewPlan(experiment.PlanInput{ID: "stage", Kind: experiment.KindConcurrency, Requests: 100})
	return runner.ExperimentReport{
		Result: experiment.Result{Plan: plan},
		Aggregate: metrics.Aggregate{
			Latency:         metrics.LatencyStatistics{Available: true, Count: count, Median: p50, P95: p95},
			SchedulingDelay: metrics.LatencyStatistics{Available: true, Count: count, Median: time.Duration(schedulingMS) * time.Millisecond},
			Rates:           metrics.RateMetrics{OfferedPerSec: rate, AchievedPerSec: rate, Throughput: rate},
			Concurrency:     metrics.ConcurrencyMetrics{Peak: int(rate), TimeWeightedInFlight: rate / 2},
			Outcomes: metrics.OutcomeCounts{
				Attempted: count, Completed: count, Successful: successes,
				Timeout: errors,
			},
		},
	}
}
