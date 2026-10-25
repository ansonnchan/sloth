// Package experiment defines runner-facing plans, samples, and results.
package experiment

import (
	"time"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

// Kind identifies a stable experiment protocol.
type Kind string

const (
	KindBaseline         Kind = "baseline"
	KindFreshConnection  Kind = "fresh_connection"
	KindReusedConnection Kind = "reused_connection"
	KindConcurrency      Kind = "concurrency"
)

// PlanInput contains the complete inputs needed to construct an immutable plan.
type PlanInput struct {
	ID       string
	Kind     Kind
	Config   config.Snapshot
	Requests int
	Warmups  int
	Workers  int
	Rate     float64
}

// Plan is an immutable description of one experiment arm.
type Plan struct {
	id       string
	kind     Kind
	config   config.Snapshot
	requests int
	warmups  int
	workers  int
	rate     float64
}

// NewPlan snapshots input so later caller mutation cannot change execution.
func NewPlan(input PlanInput) Plan {
	input.Config.ConcurrencyStages = append([]int(nil), input.Config.ConcurrencyStages...)
	return Plan{
		id: input.ID, kind: input.Kind, config: input.Config,
		requests: input.Requests, warmups: input.Warmups,
		workers: input.Workers, rate: input.Rate,
	}
}

func (p Plan) ID() string    { return p.id }
func (p Plan) Kind() Kind    { return p.kind }
func (p Plan) Requests() int { return p.requests }
func (p Plan) Warmups() int  { return p.warmups }
func (p Plan) Workers() int  { return p.workers }
func (p Plan) Rate() float64 { return p.rate }

// Config returns a defensive copy of the safe effective configuration.
func (p Plan) Config() config.Snapshot {
	snapshot := p.config
	snapshot.ConcurrencyStages = append([]int(nil), snapshot.ConcurrencyStages...)
	return snapshot
}

// Connection records observed connection state rather than requested mode.
type Connection struct {
	Observed bool `json:"observed"`
	Reused   bool `json:"reused"`
	WasIdle  bool `json:"was_idle"`
}

// Sample contains one request's scheduling and HTTP evidence.
type Sample struct {
	ExperimentID  string                   `json:"experiment_id"`
	Sequence      int                      `json:"sequence"`
	PlannedStart  time.Time                `json:"planned_start"`
	ActualStart   time.Time                `json:"actual_start"`
	FinishedAt    time.Time                `json:"finished_at"`
	Timings       httpmeasure.TimingPhases `json:"timings"`
	Outcome       httpmeasure.Outcome      `json:"outcome"`
	Connection    Connection               `json:"connection"`
	Protocol      string                   `json:"protocol,omitempty"`
	StatusCode    int                      `json:"status_code,omitempty"`
	ResponseBytes int64                    `json:"response_bytes"`
}

// SchedulingDelay is distinct from the client-observed HTTP duration.
func (s Sample) SchedulingDelay() time.Duration {
	if s.PlannedStart.IsZero() || s.ActualStart.Before(s.PlannedStart) {
		return 0
	}
	return s.ActualStart.Sub(s.PlannedStart)
}

// WarmupSamples and MeasuredSamples are deliberately distinct types. Metrics
// APIs accept only MeasuredSamples, making accidental warm-up aggregation hard.
type WarmupSamples []Sample
type MeasuredSamples []Sample

// StopReason records why an experiment produced a partial result.
type StopReason string

const (
	StopNone             StopReason = ""
	StopCancelled        StopReason = "cancelled"
	StopBudgetExhausted  StopReason = "request_budget_exhausted"
	StopExecutionFailure StopReason = "execution_failure"
)

// Execution describes the complete lifecycle of an experiment arm.
type Execution struct {
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     time.Time  `json:"finished_at"`
	OfferedStarts  int        `json:"offered_starts"`
	AchievedStarts int        `json:"achieved_starts"`
	MissedArrivals int        `json:"missed_arrivals"`
	Interrupted    bool       `json:"interrupted"`
	StopReason     StopReason `json:"stop_reason,omitempty"`
}

// Result preserves warm-up evidence and partial execution separately from the
// measured samples consumed by statistical aggregation.
type Result struct {
	Plan      Plan            `json:"-"`
	Execution Execution       `json:"execution"`
	Warmups   WarmupSamples   `json:"warmups"`
	Measured  MeasuredSamples `json:"measured"`
}
