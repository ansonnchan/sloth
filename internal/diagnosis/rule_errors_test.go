package diagnosis

import (
	"testing"

	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/runner"
)

func TestErrorRuleClassMatrix(t *testing.T) {
	tests := []struct {
		name           string
		outcomes       metrics.OutcomeCounts
		statuses       map[string]int
		wantFinding    bool
		wantSeverity   Severity
		wantConfidence Confidence
		wantLimitation bool
	}{
		{name: "timeout boundary", outcomes: outcomes(100, 99, 0, 1, 0, 0),
			wantFinding: true, wantSeverity: SeverityWarning, wantConfidence: ConfidenceHigh},
		{name: "transport high", outcomes: outcomes(100, 94, 0, 0, 6, 0),
			wantFinding: true, wantSeverity: SeverityHigh, wantConfidence: ConfidenceHigh},
		{name: "server error boundary", outcomes: outcomes(100, 99, 1, 0, 0, 0), statuses: map[string]int{"5xx": 1},
			wantFinding: true, wantSeverity: SeverityWarning, wantConfidence: ConfidenceHigh},
		{name: "client errors below separate policy", outcomes: outcomes(100, 91, 9, 0, 0, 0), statuses: map[string]int{"4xx": 9}},
		{name: "client error boundary", outcomes: outcomes(100, 90, 10, 0, 0, 0), statuses: map[string]int{"4xx": 10},
			wantFinding: true, wantSeverity: SeverityWarning, wantConfidence: ConfidenceHigh},
		{name: "user cancellation excluded", outcomes: outcomes(100, 99, 0, 0, 0, 1)},
		{name: "mixed classes", outcomes: outcomes(100, 86, 10, 2, 2, 0), statuses: map[string]int{"4xx": 5, "5xx": 5},
			wantFinding: true, wantSeverity: SeverityHigh, wantConfidence: ConfidenceHigh},
		{name: "sparse", outcomes: outcomes(9, 8, 0, 1, 0, 0), wantLimitation: true},
		{name: "contradictory", outcomes: metrics.OutcomeCounts{Attempted: 100, Successful: 100, Timeout: 1},
			wantFinding: true, wantSeverity: SeverityWarning, wantConfidence: ConfidenceLow},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := (ErrorRule{}).Evaluate(errorSuite(test.outcomes, test.statuses), DefaultThresholds())
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

func outcomes(attempted, successful, httpErrors, timeouts, transport, cancelled int) metrics.OutcomeCounts {
	return metrics.OutcomeCounts{
		Attempted: attempted, Completed: successful + httpErrors, Successful: successful,
		HTTPError: httpErrors, Timeout: timeouts, TransportError: transport, Cancelled: cancelled,
	}
}

func errorSuite(outcome metrics.OutcomeCounts, statuses map[string]int) Suite {
	plan := experiment.NewPlan(experiment.PlanInput{ID: "baseline", Kind: experiment.KindBaseline, Requests: outcome.Attempted})
	return Suite{Baseline: runner.ExperimentReport{
		Result:    experiment.Result{Plan: plan},
		Aggregate: metrics.Aggregate{Outcomes: outcome, StatusClasses: statuses},
	}}
}
