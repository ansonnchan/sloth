package compare

import (
	"testing"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/report"
)

func TestCheckCompatibleReportsPairsEveryExperiment(t *testing.T) {
	result := Check(compatibleReport(), compatibleReport(), ModeGate)
	if !result.GateEligible || !result.DescriptiveEligible || len(result.Pairs) != 2 || len(result.Fatal) != 0 || len(result.Warnings) != 0 {
		t.Fatalf("compatibility = %+v", result)
	}
}

func TestCheckEvaluatesMaterialFieldsIndependently(t *testing.T) {
	tests := []struct {
		name  string
		field string
		alter func(*report.Report)
	}{
		{"schema", "schema_version", func(value *report.Report) { value.SchemaVersion = "sloth.report/v1.1" }},
		{"target", "target.fingerprint", func(value *report.Report) { value.Target.Fingerprint = "different" }},
		{"method", "configuration.method", func(value *report.Report) { value.Configuration.Method = "POST" }},
		{"timeout", "configuration.timeout_ms", func(value *report.Report) { value.Configuration.TimeoutMillis++ }},
		{"redirect policy", "configuration.follow_redirects", func(value *report.Report) { value.Configuration.FollowRedirects = true }},
		{"body policy", "configuration.body_limit_bytes", func(value *report.Report) { value.Configuration.BodyLimitBytes++ }},
		{"unsafe policy", "configuration.allow_unsafe_methods", func(value *report.Report) { value.Configuration.AllowUnsafeMethods = true }},
		{"rate", "configuration.rate_per_second", func(value *report.Report) { value.Configuration.Rate++ }},
		{"stages", "configuration.concurrency_stages", func(value *report.Report) { value.Configuration.ConcurrencyStages = []int{1, 4} }},
		{"rules", "rule_version", func(value *report.Report) { value.RuleVersion = "rules-2" }},
		{"kind", "experiments.kind", func(value *report.Report) { value.Experiments[0].Kind = experiment.KindConcurrency }},
		{"sample size", "experiments.requested_samples", func(value *report.Report) { value.Experiments[0].RequestedSamples++ }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := compatibleReport()
			after := compatibleReport()
			test.alter(&after)
			result := Check(before, after, ModeGate)
			if result.GateEligible || !containsField(result.Fatal, test.field) {
				t.Fatalf("compatibility = %+v, want fatal %q", result, test.field)
			}
		})
	}
}

func TestDescriptiveModeRetainsCompatibleSubsetWithWarnings(t *testing.T) {
	before := compatibleReport()
	after := compatibleReport()
	after.Configuration.TimeoutMillis++
	after.RuleVersion = "rules-2"
	after.Experiments[0].Kind = experiment.KindConcurrency
	result := Check(before, after, ModeDescriptive)
	if !result.DescriptiveEligible || result.GateEligible || len(result.Pairs) != 1 || result.Pairs[0].ID != "concurrency-2" {
		t.Fatalf("compatibility = %+v", result)
	}
	for _, field := range []string{"configuration.timeout_ms", "rule_version", "experiments.kind"} {
		if !containsField(result.Warnings, field) {
			t.Fatalf("warnings = %+v, want %q", result.Warnings, field)
		}
	}
}

func TestGlobalIdentityMismatchPreventsDescriptivePairs(t *testing.T) {
	before := compatibleReport()
	after := compatibleReport()
	after.Target.Fingerprint = "different"
	result := Check(before, after, ModeDescriptive)
	if result.DescriptiveEligible || len(result.Pairs) != 0 || !containsField(result.Fatal, "target.fingerprint") {
		t.Fatalf("compatibility = %+v", result)
	}
}

func TestGateModeRejectsCombinedMismatches(t *testing.T) {
	before := compatibleReport()
	after := compatibleReport()
	after.Configuration.TimeoutMillis++
	after.Configuration.Rate++
	after.Experiments = after.Experiments[:1]
	result := Check(before, after, ModeGate)
	for _, field := range []string{"configuration.timeout_ms", "configuration.rate_per_second", "experiments.presence"} {
		if !containsField(result.Fatal, field) {
			t.Fatalf("fatal = %+v, want %q", result.Fatal, field)
		}
	}
}

func compatibleReport() report.Report {
	configuration := config.Snapshot{
		Method: "GET", TimeoutMillis: 30_000, Requests: 100, Warmups: 5, Rate: 5,
		ConcurrencyStages: []int{1, 2}, RedirectLimit: 5, BodyLimitBytes: 1 << 20,
	}
	return report.Report{
		SchemaVersion: report.SchemaVersion, RuleVersion: "rules-1",
		Target: httpmeasure.TargetIdentity{Fingerprint: "same"}, Configuration: configuration,
		Experiments: []report.Experiment{
			{ID: "baseline", Kind: experiment.KindBaseline, RequestedSamples: 100, WarmupRequests: 5, Workers: 1, RatePerSecond: 5},
			{ID: "concurrency-2", Kind: experiment.KindConcurrency, RequestedSamples: 100, WarmupRequests: 5, Workers: 2, RatePerSecond: 5},
		},
	}
}

func containsField(values []Mismatch, field string) bool {
	for _, value := range values {
		if value.Field == field {
			return true
		}
	}
	return false
}
