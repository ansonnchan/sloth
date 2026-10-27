package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/diagnose"
	"github.com/ansonnchan/sloth/internal/diagnosis"
	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/runner"
)

func TestBundledSchemaMatchesPublishedSchema(t *testing.T) {
	published, err := os.ReadFile(filepath.Join("..", "..", "docs", "schema", "report-v1.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(published) != string(Schema()) {
		t.Fatal("bundled and published schemas differ")
	}
}

func TestPublishedExamplesValidate(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "docs", "examples", "report-v1*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no report examples found")
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := ValidateJSON(data); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
}

func TestSchemaAcceptsReportVariants(t *testing.T) {
	variants := map[string]Report{
		"full":        schemaReport(false, true),
		"aggregate":   schemaReport(false, false),
		"partial":     schemaReport(true, false),
		"interrupted": schemaReport(true, true),
	}
	for name, value := range variants {
		t.Run(name, func(t *testing.T) {
			if err := Validate(value); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestSchemaRejectsInvalidContracts(t *testing.T) {
	valid, err := json.Marshal(schemaReport(false, true))
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(valid, &object); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"missing required", func(value map[string]any) { delete(value, "target") }},
		{"invalid duration range", func(value map[string]any) {
			value["configuration"].(map[string]any)["timeout_ms"] = -1.0
		}},
		{"invalid unit", func(value map[string]any) {
			value["diagnostics"].([]any)[0].(map[string]any)["evidence"].(map[string]any)["metrics"].([]any)[0].(map[string]any)["unit"] = "seconds"
		}},
		{"samples without metadata", func(value map[string]any) { value["samples"] = []any{} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			copyBytes, _ := json.Marshal(object)
			var candidate map[string]any
			_ = json.Unmarshal(copyBytes, &candidate)
			test.mutate(candidate)
			data, _ := json.Marshal(candidate)
			if err := ValidateJSON(data); err == nil {
				t.Fatalf("ValidateJSON() accepted %s", data)
			}
		})
	}
}

func schemaReport(interrupted, withFinding bool) Report {
	started := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	plan := experiment.NewPlan(experiment.PlanInput{
		ID: "baseline", Kind: experiment.KindBaseline, Config: config.Resolve(config.Defaults()).Snapshot(),
		Requests: 100, Warmups: 5, Workers: 1, Rate: 5,
	})
	execution := experiment.Execution{
		StartedAt: started, FinishedAt: started.Add(time.Second), OfferedStarts: 100,
		AchievedStarts: 100, Interrupted: interrupted,
	}
	if interrupted {
		execution.StopReason = experiment.StopCancelled
	}
	result := diagnose.Result{
		Suite: diagnosis.Suite{Baseline: runner.ExperimentReport{
			Result: experiment.Result{Plan: plan, Execution: execution},
			Aggregate: metrics.Aggregate{
				Outcomes: metrics.OutcomeCounts{Attempted: 100, Completed: 100, Successful: 100},
				Latency: metrics.LatencyStatistics{Available: true, Count: 100, Min: time.Millisecond, Max: time.Second,
					Mean: 10 * time.Millisecond, Median: 9 * time.Millisecond, P90: 12 * time.Millisecond,
					P95: 15 * time.Millisecond, P99: 20 * time.Millisecond, PopulationStdDev: 2 * time.Millisecond},
				SchedulingDelay: metrics.LatencyStatistics{}, Phases: map[string]metrics.LatencyStatistics{},
				StatusClasses: map[string]int{"2xx": 100}, Protocols: map[string]int{"HTTP/1.1": 100},
				Rates:       metrics.RateMetrics{Duration: time.Second, OfferedStarts: 100, AchievedStarts: 100, OfferedPerSec: 5, AchievedPerSec: 5, Throughput: 5},
				Concurrency: metrics.ConcurrencyMetrics{Peak: 1, TimeWeightedInFlight: .5},
				Connections: metrics.ConnectionMetrics{Observed: 100, New: 1, Reused: 99}, Interrupted: interrupted,
			},
		}},
		Diagnosis: diagnosis.Report{RuleVersion: diagnosis.CurrentRuleVersion, Findings: []diagnosis.Finding{}},
	}
	if withFinding {
		result.Diagnosis.Findings = []diagnosis.Finding{{
			Code: diagnosis.CodeTailVariability, RuleVersion: diagnosis.CurrentRuleVersion,
			Severity: diagnosis.SeverityWarning, Confidence: diagnosis.ConfidenceMedium,
			Title: "Tail variability", Observation: "p95 exceeded p50.", Recommendation: "Repeat the workload.",
			Evidence: diagnosis.Evidence{Metrics: []diagnosis.Metric{{Name: "sample_count", Value: 100, Unit: diagnosis.UnitCount}}},
		}}
	}
	report, err := Build(BuildInput{
		Result:        result,
		Target:        httpmeasure.TargetIdentity{Scheme: "https", Host: "example.com", Path: "/", RedactedURL: "https://example.com/", Fingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		Configuration: config.Resolve(config.Defaults()).Snapshot(), Tool: buildinfo.Info{Version: "test", Commit: "abc", BuildTime: "now"},
		RunID: "fixed-run", CreatedAt: started, Environment: Environment{OS: "test", Arch: "test", GoVersion: "go-test", CPUs: 2},
	})
	if err != nil {
		panic(err)
	}
	if interrupted && !report.Run.Interrupted {
		panic("interrupted report not marked")
	}
	return report
}
