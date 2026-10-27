// Package report defines Sloth's persisted, versioned evidence contract.
package report

import (
	"time"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/diagnosis"
	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/metrics"
)

const SchemaVersion = "sloth.report/v1.0"

const MaxExportedSamples = 10_000

// Report contains only redacted metadata and bounded aggregate evidence by
// default. It deliberately has no fields for headers or request/response bodies.
type Report struct {
	SchemaVersion string                     `json:"schema_version"`
	ToolVersion   string                     `json:"tool_version"`
	RuleVersion   string                     `json:"rule_version"`
	Tool          Tool                       `json:"tool"`
	Run           Run                        `json:"run"`
	Target        httpmeasure.TargetIdentity `json:"target"`
	Configuration config.Snapshot            `json:"configuration"`
	Environment   Environment                `json:"environment"`
	Overall       Overall                    `json:"overall"`
	Experiments   []Experiment               `json:"experiments"`
	Diagnostics   []diagnosis.Finding        `json:"diagnostics"`
	Limitations   []string                   `json:"limitations"`
	SampleExport  *SampleExport              `json:"sample_export,omitempty"`
	Samples       []Sample                   `json:"samples,omitempty"`
}

type Tool struct {
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
	Dirty     bool   `json:"dirty"`
}

type Run struct {
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	StartedAt   time.Time `json:"started_at"`
	FinishedAt  time.Time `json:"finished_at"`
	Interrupted bool      `json:"interrupted"`
}

// Environment is intentionally bounded and excludes hostname, username,
// interfaces, process environment, and other potentially identifying values.
type Environment struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	GoVersion string `json:"go_version"`
	CPUs      int    `json:"cpus"`
}

// Overall reconciles all measured attempts while keeping warm-up outcomes
// separate. It intentionally has no latency distribution.
type Overall struct {
	MeasuredOutcomes       metrics.OutcomeCounts `json:"measured_outcomes"`
	WarmupAttempts         int                   `json:"warmup_attempts"`
	WarmupOutcomes         map[string]int        `json:"warmup_outcomes"`
	ResponseBytes          int64                 `json:"response_bytes"`
	InterruptedExperiments int                   `json:"interrupted_experiments"`
}

type Experiment struct {
	ID               string          `json:"id"`
	Kind             experiment.Kind `json:"kind"`
	RequestedSamples int             `json:"requested_samples"`
	WarmupRequests   int             `json:"warmup_requests"`
	Workers          int             `json:"workers"`
	RatePerSecond    float64         `json:"rate_per_second"`
	WarmupOutcomes   map[string]int  `json:"warmup_outcomes"`
	Execution        Execution       `json:"execution"`
	Aggregate        Aggregate       `json:"aggregate"`
	Limitations      []string        `json:"limitations"`
}

type Execution struct {
	StartedAt      time.Time             `json:"started_at"`
	FinishedAt     time.Time             `json:"finished_at"`
	OfferedStarts  int                   `json:"offered_starts"`
	AchievedStarts int                   `json:"achieved_starts"`
	MissedArrivals int                   `json:"missed_arrivals"`
	Interrupted    bool                  `json:"interrupted"`
	StopReason     experiment.StopReason `json:"stop_reason,omitempty"`
}

type Aggregate struct {
	Outcomes        metrics.OutcomeCounts      `json:"outcomes"`
	StatusClasses   map[string]int             `json:"status_classes"`
	ResponseBytes   int64                      `json:"response_bytes"`
	Latency         Statistics                 `json:"latency"`
	SchedulingDelay Statistics                 `json:"scheduling_delay"`
	Phases          map[string]Statistics      `json:"phases"`
	Rates           Rates                      `json:"rates"`
	Concurrency     metrics.ConcurrencyMetrics `json:"concurrency"`
	Protocols       map[string]int             `json:"protocols"`
	Connections     metrics.ConnectionMetrics  `json:"connections"`
	Interrupted     bool                       `json:"interrupted"`
}

// Statistics uses explicit millisecond field names and null for unavailable
// values. Count remains meaningful even when all numeric values are unavailable.
type Statistics struct {
	Count                  int      `json:"count"`
	MinMillis              *float64 `json:"min_ms"`
	MaxMillis              *float64 `json:"max_ms"`
	MeanMillis             *float64 `json:"mean_ms"`
	P50Millis              *float64 `json:"p50_ms"`
	P90Millis              *float64 `json:"p90_ms"`
	P95Millis              *float64 `json:"p95_ms"`
	P99Millis              *float64 `json:"p99_ms"`
	PopulationStdDevMillis *float64 `json:"population_stddev_ms"`
}

type Rates struct {
	DurationMillis     float64 `json:"duration_ms"`
	OfferedStarts      int     `json:"offered_starts"`
	AchievedStarts     int     `json:"achieved_starts"`
	OfferedPerSecond   float64 `json:"offered_per_second"`
	AchievedPerSecond  float64 `json:"achieved_per_second"`
	CompletedPerSecond float64 `json:"completed_per_second"`
}

// SampleExport makes truncation explicit instead of silently implying that a
// bounded sample array contains every measured request.
type SampleExport struct {
	Available int  `json:"available"`
	Included  int  `json:"included"`
	Limit     int  `json:"limit"`
	Truncated bool `json:"truncated"`
}

// Sample contains only timing and outcome evidence. It cannot represent a URL,
// request/response headers, a body, or an underlying error message.
type Sample struct {
	ExperimentID          string                `json:"experiment_id"`
	Sequence              int                   `json:"sequence"`
	PlannedStart          time.Time             `json:"planned_start"`
	ActualStart           time.Time             `json:"actual_start"`
	FinishedAt            time.Time             `json:"finished_at"`
	SchedulingDelayMillis float64               `json:"scheduling_delay_ms"`
	Timings               SampleTimings         `json:"timings"`
	Outcome               SampleOutcome         `json:"outcome"`
	Connection            experiment.Connection `json:"connection"`
	Protocol              string                `json:"protocol,omitempty"`
	StatusCode            *int                  `json:"status_code"`
	ResponseBytes         int64                 `json:"response_bytes"`
}

type SampleTimings struct {
	Total         SampleDuration `json:"total"`
	DNS           SampleDuration `json:"dns"`
	Connect       SampleDuration `json:"connect"`
	TLS           SampleDuration `json:"tls"`
	RequestWrite  SampleDuration `json:"request_write"`
	FirstByteWait SampleDuration `json:"first_byte_wait"`
	BodyTransfer  SampleDuration `json:"body_transfer"`
}

type SampleDuration struct {
	DurationMillis    *float64                      `json:"duration_ms"`
	UnavailableReason httpmeasure.UnavailableReason `json:"unavailable_reason,omitempty"`
}

type SampleOutcome struct {
	Category         httpmeasure.OutcomeCategory `json:"category"`
	FailurePhase     httpmeasure.FailurePhase    `json:"failure_phase,omitempty"`
	ResponseReceived bool                        `json:"response_received"`
	Complete         bool                        `json:"complete"`
}

// ToolFromBuildInfo keeps version separate from build provenance.
func ToolFromBuildInfo(info buildinfo.Info) Tool {
	return Tool{Commit: info.Commit, BuildTime: info.BuildTime, Dirty: info.Dirty}
}
