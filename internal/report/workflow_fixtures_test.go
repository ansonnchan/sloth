package report_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/ansonnchan/sloth/internal/compare"
	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/diagnose"
	"github.com/ansonnchan/sloth/internal/diagnosis"
	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/render"
	"github.com/ansonnchan/sloth/internal/report"
	"github.com/ansonnchan/sloth/internal/runner"
)

func TestWorkflowFixturesValidateAgainstSchemaAndTypedParser(t *testing.T) {
	fixtures := map[string]report.Report{
		"aggregate-only": workflowFixture(fixtureOptions{}),
		"samples":        workflowFixture(fixtureOptions{samples: true}),
		"partial":        workflowFixture(fixtureOptions{interrupted: true}),
		"error-heavy":    workflowFixture(fixtureOptions{errorHeavy: true}),
		"incompatible":   workflowFixture(fixtureOptions{fingerprint: strings.Repeat("b", 64)}),
	}
	for name, fixture := range fixtures {
		t.Run(name, func(t *testing.T) {
			encoded, err := json.MarshalIndent(fixture, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := report.ValidateJSON(encoded); err != nil {
				t.Fatalf("schema: %v", err)
			}
			parsed, err := report.Parse(encoded)
			if err != nil {
				t.Fatalf("typed parser: %v", err)
			}
			if parsed.Report.Run.CreatedAt.Location() != time.UTC || parsed.Report.Run.StartedAt.Location() != time.UTC || parsed.Report.Run.FinishedAt.Location() != time.UTC {
				t.Fatalf("timestamps are not UTC: %+v", parsed.Report.Run)
			}
			secondEncoding, err := json.MarshalIndent(parsed.Report, "", "  ")
			if err != nil || !bytes.Equal(encoded, secondEncoding) {
				t.Fatalf("golden ordering changed after round trip: %v", err)
			}
			if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "\"headers\"") || strings.Contains(string(encoded), "\"request_body\"") || strings.Contains(string(encoded), "\"response_body\"") {
				t.Fatalf("fixture contains forbidden request data: %s", encoded)
			}
		})
	}
	if fixtures["samples"].SampleExport == nil || len(fixtures["samples"].Samples) != 2 || !fixtures["samples"].SampleExport.Truncated {
		t.Fatalf("sample fixture = %+v", fixtures["samples"].SampleExport)
	}
	if !fixtures["partial"].Run.Interrupted || fixtures["error-heavy"].Overall.MeasuredOutcomes.Successful >= fixtures["error-heavy"].Overall.MeasuredOutcomes.Attempted {
		t.Fatal("partial/error-heavy fixture semantics were lost")
	}
}

func TestCommittedReportExamplesAreStableGoldenContracts(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "docs", "examples", "report-v1*.json"))
	if err != nil || len(paths) < 2 {
		t.Fatalf("examples = %v, %v", paths, err)
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := report.Parse(data)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		orderedKeys := []string{"\"schema_version\"", "\"tool_version\"", "\"rule_version\"", "\"tool\"", "\"run\"", "\"target\"", "\"configuration\"", "\"environment\"", "\"overall\"", "\"experiments\"", "\"diagnostics\"", "\"limitations\""}
		previous := -1
		for _, key := range orderedKeys {
			position := bytes.Index(data, []byte("\n  "+key))
			if position <= previous {
				t.Fatalf("%s: top-level golden ordering changed near %s", path, key)
			}
			previous = position
		}
		if parsed.Report.Run.CreatedAt.Location() != time.UTC {
			t.Fatalf("%s: created_at is not UTC", path)
		}
	}
}

func TestWorkflowFixturesExerciseComparisonAndGate(t *testing.T) {
	before := workflowFixture(fixtureOptions{})
	after := workflowFixture(fixtureOptions{errorHeavy: true})
	compatibility := compare.Check(before, after, compare.ModeGate)
	if !compatibility.GateEligible {
		t.Fatalf("compatibility = %+v", compatibility)
	}
	deltas, err := compare.Calculate(compatibility)
	if err != nil {
		t.Fatal(err)
	}
	gate, err := compare.EvaluateGate(deltas, compare.GateP95, 20)
	if err != nil || len(gate.Regressions) != 1 {
		t.Fatalf("gate = %+v, %v", gate, err)
	}
	var output bytes.Buffer
	if err := render.Comparison(&output, compatibility, deltas, render.Options{Width: 120, ASCII: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "regressed") || !strings.Contains(output.String(), "outcomes.error_rate_percent") {
		t.Fatalf("comparison output:\n%s", output.String())
	}

	incompatible := compare.Check(before, workflowFixture(fixtureOptions{fingerprint: strings.Repeat("b", 64)}), compare.ModeDescriptive)
	if incompatible.DescriptiveEligible || len(incompatible.Fatal) == 0 {
		t.Fatalf("incompatible fixture passed: %+v", incompatible)
	}
}

type fixtureOptions struct {
	samples     bool
	interrupted bool
	errorHeavy  bool
	fingerprint string
}

func workflowFixture(options fixtureOptions) report.Report {
	started := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	configuration := config.Defaults()
	configuration.Requests = 10
	configuration.Warmups = 0
	configuration.ConcurrencyStages = []int{1}
	configuration.IncludeSamples = options.samples
	configuration.SampleLimit = 2
	effective := config.Resolve(configuration)
	plan := experiment.NewPlan(experiment.PlanInput{
		ID: "baseline", Kind: experiment.KindBaseline, Config: effective.Snapshot(), Requests: 10, Workers: 1, Rate: effective.Rate(),
	})
	attempted := 10
	if options.interrupted {
		attempted = 3
	}
	outcomes := metrics.OutcomeCounts{Attempted: attempted, Completed: attempted, Successful: attempted}
	p95 := 100 * time.Millisecond
	if options.errorHeavy {
		outcomes = metrics.OutcomeCounts{Attempted: 10, Completed: 7, Successful: 3, HTTPError: 4, TransportError: 1, Timeout: 2}
		p95 = 130 * time.Millisecond
	}
	execution := experiment.Execution{StartedAt: started, FinishedAt: started.Add(time.Second), OfferedStarts: attempted, AchievedStarts: attempted, Interrupted: options.interrupted}
	if options.interrupted {
		execution.StopReason = experiment.StopCancelled
	}
	measured := make(experiment.MeasuredSamples, 0, 10)
	if options.samples {
		for sequence := range 10 {
			measured = append(measured, experiment.Sample{
				ExperimentID: "baseline", Sequence: sequence, PlannedStart: started, ActualStart: started,
				FinishedAt: started.Add(10 * time.Millisecond),
				Outcome:    httpmeasure.Outcome{Category: httpmeasure.OutcomeSuccess, StatusCode: 200, ResponseReceived: true, Complete: true},
				Connection: experiment.Connection{Observed: true, Reused: sequence > 0}, Protocol: "HTTP/1.1", StatusCode: 200, ResponseBytes: 100,
			})
		}
	}
	latency := metrics.LatencyStatistics{
		Available: true, Count: attempted, Min: 50 * time.Millisecond, Max: p95,
		Mean: 70 * time.Millisecond, Median: 60 * time.Millisecond, P90: 90 * time.Millisecond,
		P95: p95, P99: p95, PopulationStdDev: 10 * time.Millisecond,
	}
	result := diagnose.Result{
		Suite: diagnosis.Suite{Baseline: runner.ExperimentReport{
			Result: experiment.Result{Plan: plan, Execution: execution, Measured: measured},
			Aggregate: metrics.Aggregate{
				Outcomes: outcomes, Latency: latency, Phases: map[string]metrics.LatencyStatistics{},
				StatusClasses: map[string]int{"2xx": outcomes.Successful, "4xx": outcomes.HTTPError}, Protocols: map[string]int{"HTTP/1.1": attempted},
				Rates:       metrics.RateMetrics{Duration: time.Second, OfferedStarts: attempted, AchievedStarts: attempted, OfferedPerSec: 5, AchievedPerSec: 5, Throughput: float64(outcomes.Completed)},
				Concurrency: metrics.ConcurrencyMetrics{Peak: 1, TimeWeightedInFlight: .5},
				Connections: metrics.ConnectionMetrics{Observed: attempted, New: 1, Reused: attempted - 1}, Interrupted: options.interrupted,
			},
		}},
		Diagnosis: diagnosis.Report{RuleVersion: diagnosis.CurrentRuleVersion, Findings: []diagnosis.Finding{}},
	}
	fingerprint := options.fingerprint
	if fingerprint == "" {
		fingerprint = strings.Repeat("a", 64)
	}
	value, err := report.Build(report.BuildInput{
		Result: result, Configuration: effective.Snapshot(), Tool: buildinfo.Info{Version: "fixture", Commit: "fixed", BuildTime: "fixed"},
		Target: httpmeasure.TargetIdentity{Scheme: "https", Host: "example.com", Path: "/api", RedactedURL: "https://example.com/api", Fingerprint: fingerprint},
		RunID:  "fixture-" + fixtureName(options), CreatedAt: started,
		Environment: report.Environment{OS: "fixture", Arch: "fixture", GoVersion: "fixture", CPUs: 1},
	})
	if err != nil {
		panic(err)
	}
	return value
}

func fixtureName(options fixtureOptions) string {
	switch {
	case options.samples:
		return "samples"
	case options.interrupted:
		return "partial"
	case options.errorHeavy:
		return "error-heavy"
	case options.fingerprint != "":
		return "incompatible"
	default:
		return "aggregate"
	}
}
