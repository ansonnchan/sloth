package diagnose

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/diagnosis"
	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/runner"
	"github.com/ansonnchan/sloth/internal/safety"
)

func TestNewDocumentContainsAggregateOnlyStableContract(t *testing.T) {
	options := config.Defaults()
	effective := config.Resolve(options)
	plan := experiment.NewPlan(experiment.PlanInput{ID: "baseline", Kind: experiment.KindBaseline, Requests: 100})
	result := Result{
		Suite: diagnosis.Suite{Baseline: runner.ExperimentReport{
			Result: experiment.Result{Plan: plan, Measured: experiment.MeasuredSamples{{Sequence: 1}}},
			Aggregate: metrics.Aggregate{
				Latency:     metrics.LatencyStatistics{Available: true, Count: 100, Median: 10 * time.Millisecond, P95: 20 * time.Millisecond, P99: 30 * time.Millisecond},
				Outcomes:    metrics.OutcomeCounts{Attempted: 100, Successful: 99, Timeout: 1},
				Rates:       metrics.RateMetrics{OfferedPerSec: 5, AchievedPerSec: 4.9},
				Concurrency: metrics.ConcurrencyMetrics{TimeWeightedInFlight: 1.2},
			},
		}},
		Diagnosis: diagnosis.Report{RuleVersion: diagnosis.CurrentRuleVersion, Findings: []diagnosis.Finding{}},
	}
	budget := safety.CalculateBudget(safety.StandardExperiments(effective), effective.Rate())
	document := NewDocument(result, effective, budget)
	if document.SchemaVersion != SchemaVersion || len(document.Experiments) != 1 || document.Experiments[0].Samples != 100 {
		t.Fatalf("document = %+v", document)
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"measured\"", "planned_start", "timings\"", "\x1b["} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("document contains %q: %s", forbidden, text)
		}
	}
}

func TestNewDocumentUsesNullForUnavailableLatency(t *testing.T) {
	plan := experiment.NewPlan(experiment.PlanInput{ID: "baseline", Kind: experiment.KindBaseline, Requests: 10})
	result := Result{Suite: diagnosis.Suite{Baseline: runner.ExperimentReport{
		Result: experiment.Result{Plan: plan}, Aggregate: metrics.Aggregate{Outcomes: metrics.OutcomeCounts{Attempted: 10}},
	}}}
	document := NewDocument(result, config.Resolve(config.Defaults()), safety.Budget{})
	if document.Experiments[0].P50Millis != nil || document.Experiments[0].P95Millis != nil || document.Experiments[0].P99Millis != nil {
		t.Fatalf("unavailable latency = %+v", document.Experiments[0])
	}
}
