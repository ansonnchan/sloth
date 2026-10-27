package diagnosis

import (
	"strings"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/metrics"
)

func TestInsufficientEvidenceRuleMatrix(t *testing.T) {
	rule := InsufficientEvidenceRule{}
	tests := []struct {
		name        string
		prior       []Evaluation
		wantFinding bool
		wantText    string
	}{
		{name: "sparse", prior: []Evaluation{{Limitations: []string{"only 3 samples are available; at least 10 are required"}}},
			wantFinding: true, wantText: "enough complete samples"},
		{name: "conflicting", prior: []Evaluation{{Limitations: []string{"another observed metric contradicts the diagnostic signal"}}},
			wantFinding: true, wantText: "enough complete samples"},
		{name: "interrupted", prior: []Evaluation{{Limitations: []string{"the experiment was interrupted before its planned evidence was complete"}}},
			wantFinding: true, wantText: "without interruption"},
		{name: "unchanged load", prior: []Evaluation{{Limitations: []string{"concurrency stages did not produce a meaningful achieved-load change"}}},
			wantFinding: true, wantText: "offered rates"},
		{name: "adequate normal", prior: []Evaluation{{}, {}, {}}, wantFinding: false},
		{name: "supported positive suppresses inconclusive", prior: []Evaluation{
			{Finding: &Finding{Code: CodeTailVariability, Confidence: ConfidenceMedium}},
			{Limitations: []string{"connection arms are unavailable"}},
		}, wantFinding: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := rule.EvaluateSummary(Suite{}, DefaultThresholds(), test.prior)
			if (got.Finding != nil) != test.wantFinding {
				t.Fatalf("finding = %+v", got.Finding)
			}
			if got.Finding != nil {
				if len(got.Finding.PossibleCauses) != 0 {
					t.Fatalf("possible causes = %v, want none", got.Finding.PossibleCauses)
				}
				if !strings.Contains(got.Finding.Recommendation, test.wantText) {
					t.Fatalf("recommendation = %q, want %q", got.Finding.Recommendation, test.wantText)
				}
			}
		})
	}
}

func TestDefaultEngineAddsInsufficientEvidenceForSparseSuite(t *testing.T) {
	suite := firstByteSuite(100, 3, 3, 200*time.Millisecond, 150*time.Millisecond, false)
	suite.Baseline.Aggregate.Outcomes = metrics.OutcomeCounts{Attempted: 3, Successful: 3, Completed: 3}
	report, err := DefaultEngine().Evaluate(suite)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Findings) != 1 || report.Findings[0].Code != CodeInsufficientEvidence {
		t.Fatalf("findings = %+v, want one insufficient-evidence finding", report.Findings)
	}
}

func TestDefaultEngineDoesNotAddInsufficientEvidenceBesideStrongFinding(t *testing.T) {
	suite := firstByteSuite(100, 100, 100, 200*time.Millisecond, 150*time.Millisecond, false)
	suite.Baseline.Aggregate.Outcomes = metrics.OutcomeCounts{Attempted: 100, Successful: 100, Completed: 100}
	report, err := DefaultEngine().Evaluate(suite)
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range report.Findings {
		if finding.Code == CodeInsufficientEvidence {
			t.Fatalf("unexpected insufficient-evidence finding: %+v", finding)
		}
	}
}
