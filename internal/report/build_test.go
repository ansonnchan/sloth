package report

import (
	"encoding/json"
	"strings"
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

func TestBuildCreatesCanonicalRedactedReport(t *testing.T) {
	start := time.Date(2026, 10, 27, 1, 2, 3, 0, time.FixedZone("test", -7*60*60))
	plan := experiment.NewPlan(experiment.PlanInput{
		ID: "baseline", Kind: experiment.KindBaseline, Requests: 2, Warmups: 1, Workers: 1, Rate: 5,
	})
	result := diagnose.Result{
		Suite: diagnosis.Suite{Baseline: runner.ExperimentReport{
			Result: experiment.Result{
				Plan: plan, WarmupSummary: experiment.OutcomeSummary{httpmeasure.OutcomeSuccess: 1},
				Execution: experiment.Execution{StartedAt: start, FinishedAt: start.Add(time.Second)},
			},
			Aggregate: metrics.Aggregate{
				Outcomes:      metrics.OutcomeCounts{Attempted: 2, Completed: 2, Successful: 1, HTTPError: 1},
				ResponseBytes: 25,
				Latency: metrics.LatencyStatistics{Available: true, Count: 2, Min: time.Millisecond, Max: 2 * time.Millisecond,
					Mean: 1500 * time.Microsecond, Median: 1500 * time.Microsecond, P90: 1900 * time.Microsecond,
					P95: 1950 * time.Microsecond, P99: 1990 * time.Microsecond, PopulationStdDev: 500 * time.Microsecond},
				Phases: map[string]metrics.LatencyStatistics{}, StatusClasses: map[string]int{"2xx": 1, "4xx": 1},
				Protocols: map[string]int{"HTTP/1.1": 2},
			},
		}},
		Diagnosis: diagnosis.Report{RuleVersion: "rules-1", Findings: []diagnosis.Finding{}},
	}
	target := httpmeasure.TargetIdentity{
		Scheme: "https", Host: "example.com", Path: "/api", QueryKeys: []string{"token"},
		RedactedURL: "https://example.com/api?token=%5BREDACTED%5D", Fingerprint: strings.Repeat("a", 64),
	}
	got, err := Build(BuildInput{
		Result: result, Target: target, Configuration: config.Resolve(config.Defaults()).Snapshot(),
		Tool:  buildinfo.Info{Version: "v1.2.3", Commit: "abc", BuildTime: "now"},
		RunID: "fixed-run", CreatedAt: start, Environment: Environment{OS: "test", Arch: "test", GoVersion: "go-test", CPUs: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || got.ToolVersion != "v1.2.3" || got.RuleVersion != "rules-1" {
		t.Fatalf("versions = %q/%q/%q", got.SchemaVersion, got.ToolVersion, got.RuleVersion)
	}
	if got.Run.CreatedAt.Location() != time.UTC || got.Run.ID != "fixed-run" || got.Target.Fingerprint != target.Fingerprint {
		t.Fatalf("metadata = %+v target=%+v", got.Run, got.Target)
	}
	if got.Overall.MeasuredOutcomes.Attempted != 2 || got.Overall.WarmupAttempts != 1 || got.Overall.ResponseBytes != 25 {
		t.Fatalf("overall = %+v", got.Overall)
	}
	if len(got.Experiments) != 1 || got.Experiments[0].Aggregate.Latency.P95Millis == nil || *got.Experiments[0].Aggregate.Latency.P95Millis != 1.95 {
		t.Fatalf("experiments = %+v", got.Experiments)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, forbidden := range []string{"secret", "headers", "request_body", "response_body"} {
		if strings.Contains(strings.ToLower(text), forbidden) {
			t.Fatalf("report contains forbidden %q: %s", forbidden, text)
		}
	}
}

func TestBuildKeepsUnavailableStatisticsNull(t *testing.T) {
	plan := experiment.NewPlan(experiment.PlanInput{ID: "baseline", Kind: experiment.KindBaseline, Requests: 1})
	value, err := Build(BuildInput{
		Result: diagnose.Result{Suite: diagnosis.Suite{Baseline: runner.ExperimentReport{
			Result: experiment.Result{Plan: plan}, Aggregate: metrics.Aggregate{Latency: metrics.LatencyStatistics{Count: 1}},
		}}},
		RunID: "run", CreatedAt: time.Unix(1, 0), Environment: Environment{OS: "test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	statistics := value.Experiments[0].Aggregate.Latency
	if statistics.Count != 1 || statistics.MinMillis != nil || statistics.P99Millis != nil {
		t.Fatalf("statistics = %+v", statistics)
	}
}

func TestBuildGeneratesRunIDAndBoundedEnvironment(t *testing.T) {
	value, err := Build(BuildInput{CreatedAt: time.Unix(1, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if len(value.Run.ID) != 32 || value.Environment.OS == "" || value.Environment.Arch == "" || value.Environment.CPUs < 1 {
		t.Fatalf("run/environment = %+v/%+v", value.Run, value.Environment)
	}
}

func TestBuildIncludesOnlyBoundedPrivacySafeSamplesWhenRequested(t *testing.T) {
	started := time.Date(2026, 10, 27, 1, 2, 3, 0, time.UTC)
	measured := experiment.MeasuredSamples{}
	for sequence := range 3 {
		measured = append(measured, experiment.Sample{
			ExperimentID: "baseline", Sequence: sequence,
			PlannedStart: started, ActualStart: started.Add(time.Millisecond), FinishedAt: started.Add(2 * time.Millisecond),
			Outcome:    httpmeasure.Outcome{Category: httpmeasure.OutcomeSuccess, StatusCode: 200, ResponseReceived: true, Complete: true},
			Connection: experiment.Connection{Observed: true, Reused: sequence > 0}, Protocol: "HTTP/1.1", StatusCode: 200, ResponseBytes: 42,
		})
	}
	options := config.Defaults()
	options.IncludeSamples = true
	options.SampleLimit = 2
	effective := config.Resolve(options)
	plan := experiment.NewPlan(experiment.PlanInput{ID: "baseline", Kind: experiment.KindBaseline, Config: effective.Snapshot(), Requests: 3, Workers: 1, Rate: 5})
	value, err := Build(BuildInput{
		Result: diagnose.Result{
			Suite: diagnosis.Suite{Baseline: runner.ExperimentReport{
				Result:    experiment.Result{Plan: plan, Execution: experiment.Execution{StartedAt: started, FinishedAt: started.Add(time.Second)}, Measured: measured},
				Aggregate: metrics.Aggregate{Phases: map[string]metrics.LatencyStatistics{}},
			}},
			Diagnosis: diagnosis.Report{RuleVersion: diagnosis.CurrentRuleVersion},
		},
		Target:        httpmeasure.TargetIdentity{Scheme: "https", Host: "example.com", Path: "/", RedactedURL: "https://example.com/", Fingerprint: strings.Repeat("a", 64)},
		Configuration: effective.Snapshot(), Tool: buildinfo.Info{Version: "test"}, RunID: "sample-run", CreatedAt: started,
		Environment: Environment{OS: "test", Arch: "test", GoVersion: "go-test", CPUs: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if value.SampleExport == nil || value.SampleExport.Available != 3 || value.SampleExport.Included != 2 || !value.SampleExport.Truncated || len(value.Samples) != 2 {
		t.Fatalf("sample export = %+v, samples = %d", value.SampleExport, len(value.Samples))
	}
	if value.Samples[0].StatusCode == nil || *value.Samples[0].StatusCode != 200 || value.Samples[0].SchedulingDelayMillis != 1 {
		t.Fatalf("sample = %+v", value.Samples[0])
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	sampleBytes, _ := json.Marshal(value.Samples)
	for _, forbidden := range []string{"https://", "secret", "\"headers\"", "\"request_body\"", "\"response_body\"", "\"error\""} {
		if strings.Contains(strings.ToLower(string(sampleBytes)), forbidden) {
			t.Fatalf("samples contain forbidden %q: %s", forbidden, sampleBytes)
		}
	}
	if err := ValidateJSON(encoded); err != nil {
		t.Fatal(err)
	}
	parsed, err := Parse(encoded)
	if err != nil || len(parsed.Report.Samples) != 2 {
		t.Fatalf("round trip = %+v, %v", parsed, err)
	}
}

func TestBuildOmitsSamplesByDefaultAndRejectsUnboundedLimit(t *testing.T) {
	value, err := Build(BuildInput{CreatedAt: time.Unix(1, 0)})
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(value)
	if value.SampleExport != nil || value.Samples != nil || strings.Contains(string(encoded), "\"samples\":") {
		t.Fatalf("default report contains sample data: %s", encoded)
	}
	_, err = Build(BuildInput{Configuration: config.Snapshot{IncludeSamples: true, SampleLimit: MaxExportedSamples + 1}})
	if err == nil || !strings.Contains(err.Error(), "sample limit") {
		t.Fatalf("error = %v", err)
	}
}
