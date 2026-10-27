package cli

import (
	"bytes"
	"os"
	"path/filepath"
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
	"github.com/ansonnchan/sloth/internal/report"
	"github.com/ansonnchan/sloth/internal/runner"
)

func TestCompareCommandSupportsRelativePathsAndDescriptiveOutput(t *testing.T) {
	directory := t.TempDir()
	writeComparisonFixture(t, filepath.Join(directory, "before.json"), 50, 100, 120, false, "a")
	writeComparisonFixture(t, filepath.Join(directory, "after.json"), 40, 80, 100, false, "a")
	t.Chdir(directory)
	var stdout, stderr bytes.Buffer
	code := Run([]string{"compare", "before.json", "after.json"}, &stdout, &stderr, buildinfo.Info{})
	if code != int(ExitSuccess) || !strings.Contains(stdout.String(), "result=improved") || stderr.Len() != 0 {
		t.Fatalf("Run() = %d, streams = (%q, %q)", code, stdout.String(), stderr.String())
	}
}

func TestCompareRegressionGateExitCodes(t *testing.T) {
	directory := t.TempDir()
	before := filepath.Join(directory, "before.json")
	writeComparisonFixture(t, before, 50, 100, 120, false, "a")
	tests := []struct {
		name     string
		p95      time.Duration
		wantCode ExitCode
	}{
		{"below threshold", 119 * time.Millisecond, ExitSuccess},
		{"above threshold", 125 * time.Millisecond, ExitRegression},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			after := filepath.Join(directory, strings.ReplaceAll(test.name, " ", "-")+".json")
			writeComparisonFixture(t, after, 50, test.p95.Milliseconds(), 140, false, "a")
			var stdout, stderr bytes.Buffer
			code := Run([]string{"compare", "--fail-on-regression", "20", before, after}, &stdout, &stderr, buildinfo.Info{})
			if code != int(test.wantCode) {
				t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
			}
			if test.wantCode == ExitRegression && (!strings.Contains(stderr.String(), "regression gate failed") || !strings.Contains(stdout.String(), "regressed")) {
				t.Fatalf("streams = (%q, %q)", stdout.String(), stderr.String())
			}
		})
	}
}

func TestCompareRejectsMalformedIncompatibleAndInadequateReports(t *testing.T) {
	directory := t.TempDir()
	valid := filepath.Join(directory, "valid.json")
	writeComparisonFixture(t, valid, 50, 100, 120, false, "a")
	tests := []struct {
		name   string
		before string
		after  func() string
		args   []string
	}{
		{"malformed", valid, func() string {
			path := filepath.Join(directory, "malformed.json")
			if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
			return path
		}, nil},
		{"incompatible", valid, func() string {
			path := filepath.Join(directory, "incompatible.json")
			writeComparisonFixture(t, path, 50, 100, 120, false, "b")
			return path
		}, []string{"--fail-on-regression", "20"}},
		{"zero baseline", func() string {
			path := filepath.Join(directory, "zero.json")
			writeComparisonFixture(t, path, 0, 0, 0, false, "a")
			return path
		}(), func() string { return valid }, []string{"--fail-on-regression", "20"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"compare"}, test.args...)
			args = append(args, test.before, test.after())
			var stdout, stderr bytes.Buffer
			if code := Run(args, &stdout, &stderr, buildinfo.Info{}); code != int(ExitUsage) {
				t.Fatalf("Run() = %d, streams = (%q, %q)", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestCompareInterruptedReportReturns130(t *testing.T) {
	directory := t.TempDir()
	before := filepath.Join(directory, "before.json")
	after := filepath.Join(directory, "after.json")
	writeComparisonFixture(t, before, 50, 100, 120, true, "a")
	writeComparisonFixture(t, after, 50, 100, 120, false, "a")
	var stdout, stderr bytes.Buffer
	if code := Run([]string{"compare", before, after}, &stdout, &stderr, buildinfo.Info{}); code != int(ExitInterrupted) {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
}

func writeComparisonFixture(t *testing.T, path string, p50, p95, p99 int64, interrupted bool, fingerprintCharacter string) {
	t.Helper()
	started := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	options := config.Defaults()
	options.Requests = 10
	options.Warmups = 0
	options.ConcurrencyStages = []int{1}
	effective := config.Resolve(options)
	plan := experiment.NewPlan(experiment.PlanInput{ID: "baseline", Kind: experiment.KindBaseline, Config: effective.Snapshot(), Requests: 10, Workers: 1, Rate: effective.Rate()})
	execution := experiment.Execution{StartedAt: started, FinishedAt: started.Add(time.Second), OfferedStarts: 10, AchievedStarts: 10, Interrupted: interrupted}
	if interrupted {
		execution.StopReason = experiment.StopCancelled
	}
	latency := metrics.LatencyStatistics{
		Available: true, Count: 10, Min: time.Duration(p50) * time.Millisecond, Max: time.Duration(p99) * time.Millisecond,
		Mean: time.Duration(p50) * time.Millisecond, Median: time.Duration(p50) * time.Millisecond,
		P90: time.Duration(p95) * time.Millisecond, P95: time.Duration(p95) * time.Millisecond,
		P99: time.Duration(p99) * time.Millisecond,
	}
	result := diagnose.Result{
		Suite: diagnosis.Suite{Baseline: runner.ExperimentReport{
			Result: experiment.Result{Plan: plan, Execution: execution},
			Aggregate: metrics.Aggregate{
				Outcomes: metrics.OutcomeCounts{Attempted: 10, Completed: 10, Successful: 10}, Latency: latency,
				Phases: map[string]metrics.LatencyStatistics{}, StatusClasses: map[string]int{"2xx": 10}, Protocols: map[string]int{"HTTP/1.1": 10},
				Rates:       metrics.RateMetrics{Duration: time.Second, OfferedStarts: 10, AchievedStarts: 10, OfferedPerSec: 5, AchievedPerSec: 5, Throughput: 5},
				Concurrency: metrics.ConcurrencyMetrics{Peak: 1, TimeWeightedInFlight: .5},
				Connections: metrics.ConnectionMetrics{Observed: 10, New: 1, Reused: 9}, Interrupted: interrupted,
			},
		}},
		Diagnosis: diagnosis.Report{RuleVersion: diagnosis.CurrentRuleVersion, Findings: []diagnosis.Finding{}},
	}
	value, err := report.Build(report.BuildInput{
		Result: result, Configuration: effective.Snapshot(), Tool: buildinfo.Info{Version: "test"},
		Target: httpmeasure.TargetIdentity{
			Scheme: "https", Host: "example.com", Path: "/", RedactedURL: "https://example.com/",
			Fingerprint: strings.Repeat(fingerprintCharacter, 64),
		},
		RunID: "compare-fixture", CreatedAt: started, Environment: report.Environment{OS: "test", Arch: "test", GoVersion: "test", CPUs: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writeExportReport(path, value, false); err != nil {
		t.Fatal(err)
	}
}
