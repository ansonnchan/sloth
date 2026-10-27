package diagnose

import (
	"strconv"
	"time"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/diagnosis"
	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/runner"
	"github.com/ansonnchan/sloth/internal/safety"
)

const SchemaVersion = "sloth.diagnose/v1"

// Document is the bounded, aggregate-only diagnose output. Raw request samples
// remain in memory for later report-model work and are not printed by default.
type Document struct {
	SchemaVersion string              `json:"schema_version"`
	RuleVersion   string              `json:"rule_version"`
	Configuration config.Snapshot     `json:"configuration"`
	Plan          PlanSummary         `json:"plan"`
	Experiments   []ExperimentSummary `json:"experiments"`
	Findings      []diagnosis.Finding `json:"findings"`
	Limitations   []string            `json:"limitations,omitempty"`
}

type PlanSummary struct {
	ExperimentArms        int     `json:"experiment_arms"`
	MeasuredRequests      int     `json:"measured_requests"`
	WarmupRequests        int     `json:"warmup_requests"`
	TotalRequests         int     `json:"total_requests"`
	MinimumDurationMillis float64 `json:"minimum_duration_ms"`
}

type ExperimentSummary struct {
	Name              string   `json:"name"`
	Samples           int      `json:"samples"`
	P50Millis         *float64 `json:"p50_ms"`
	P95Millis         *float64 `json:"p95_ms"`
	P99Millis         *float64 `json:"p99_ms"`
	Errors            int      `json:"errors"`
	OfferedPerSecond  float64  `json:"offered_per_second"`
	AchievedPerSecond float64  `json:"achieved_per_second"`
	ActualConcurrency float64  `json:"actual_concurrency"`
	Interrupted       bool     `json:"interrupted"`
}

func NewDocument(result Result, effective config.Effective, budget safety.Budget) Document {
	document := Document{
		SchemaVersion: SchemaVersion,
		RuleVersion:   result.Diagnosis.RuleVersion,
		Configuration: effective.Snapshot(),
		Plan: PlanSummary{
			ExperimentArms: budget.ExperimentArms, MeasuredRequests: budget.Measured,
			WarmupRequests: budget.Warmups, TotalRequests: budget.Total,
			MinimumDurationMillis: durationMillis(budget.MinimumTime),
		},
		Findings: append([]diagnosis.Finding(nil), result.Diagnosis.Findings...),
	}
	appendExperiment := func(name string, report runner.ExperimentReport) {
		if report.Result.Plan.Requests() == 0 && report.Aggregate.Outcomes.Attempted == 0 {
			return
		}
		document.Experiments = append(document.Experiments, summarizeExperiment(name, report.Aggregate))
		for _, limitation := range report.Aggregate.Limitations {
			document.Limitations = appendUnique(document.Limitations, name+": "+limitation)
		}
	}
	appendExperiment("baseline", result.Suite.Baseline)
	appendExperiment("fresh connection", result.Suite.Connection.Fresh)
	appendExperiment("reused connection", result.Suite.Connection.Reused)
	for index, report := range result.Suite.Concurrency {
		name := report.Result.Plan.ID()
		if name == "" {
			name = "concurrency stage " + strconv.Itoa(index+1)
		}
		appendExperiment(name, report)
	}
	for _, limitation := range result.Suite.Connection.Limitations {
		document.Limitations = appendUnique(document.Limitations, "connection comparison: "+limitation)
	}
	for _, finding := range document.Findings {
		if finding.Code != diagnosis.CodeInsufficientEvidence {
			continue
		}
		for _, limitation := range finding.Evidence.Notes {
			document.Limitations = appendUnique(document.Limitations, limitation)
		}
	}
	if document.Experiments == nil {
		document.Experiments = []ExperimentSummary{}
	}
	if document.Findings == nil {
		document.Findings = []diagnosis.Finding{}
	}
	return document
}

func summarizeExperiment(name string, aggregate metrics.Aggregate) ExperimentSummary {
	summary := ExperimentSummary{
		Name: name, Samples: aggregate.Latency.Count,
		Errors:           aggregate.Outcomes.Timeout + aggregate.Outcomes.TransportError + aggregate.Outcomes.HTTPError + aggregate.Outcomes.BodyLimit,
		OfferedPerSecond: aggregate.Rates.OfferedPerSec, AchievedPerSecond: aggregate.Rates.AchievedPerSec,
		ActualConcurrency: aggregate.Concurrency.TimeWeightedInFlight, Interrupted: aggregate.Interrupted,
	}
	if aggregate.Latency.Available {
		summary.P50Millis = durationMillisPointer(aggregate.Latency.Median)
		summary.P95Millis = durationMillisPointer(aggregate.Latency.P95)
		summary.P99Millis = durationMillisPointer(aggregate.Latency.P99)
	}
	return summary
}

func durationMillis(value time.Duration) float64 {
	return float64(value) / float64(time.Millisecond)
}

func durationMillisPointer(value time.Duration) *float64 {
	milliseconds := durationMillis(value)
	return &milliseconds
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}
