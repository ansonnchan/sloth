// Package diagnosis turns bounded experiment evidence into deterministic findings.
package diagnosis

import (
	"time"

	"github.com/ansonnchan/sloth/internal/runner"
)

// Code is a stable machine-readable diagnostic identifier.
type Code string

const (
	CodeHighFirstByteWait      Code = "HIGH_FIRST_BYTE_WAIT"
	CodeConnectionOverhead     Code = "CONNECTION_OVERHEAD"
	CodeTailVariability        Code = "TAIL_VARIABILITY"
	CodeConcurrencySensitivity Code = "CONCURRENCY_SENSITIVITY"
	CodeErrorRate              Code = "ERROR_RATE"
	CodeInsufficientEvidence   Code = "INSUFFICIENT_EVIDENCE"
)

// Severity describes the operational importance of an observation.
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityHigh    Severity = "high"
)

// Confidence describes evidence completeness and consistency, not probability.
type Confidence string

const (
	ConfidenceLow    Confidence = "low"
	ConfidenceMedium Confidence = "medium"
	ConfidenceHigh   Confidence = "high"
)

// Unit gives every numeric evidence value an explicit interpretation.
type Unit string

const (
	UnitCount             Unit = "count"
	UnitRatio             Unit = "ratio"
	UnitMilliseconds      Unit = "milliseconds"
	UnitRequestsPerSecond Unit = "requests_per_second"
)

// Metric is one typed numeric observation supporting a finding.
type Metric struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  Unit    `json:"unit"`
}

// Evidence contains explicit measurements and nonnumeric qualifications.
// Ordered slices keep both terminal and machine output deterministic.
type Evidence struct {
	Metrics []Metric `json:"metrics,omitempty"`
	Notes   []string `json:"notes,omitempty"`
}

// Finding separates what Sloth observed from hypotheses about why it happened.
type Finding struct {
	Code           Code       `json:"code"`
	RuleVersion    string     `json:"rule_version"`
	Severity       Severity   `json:"severity"`
	Confidence     Confidence `json:"confidence"`
	Title          string     `json:"title"`
	Observation    string     `json:"observation"`
	PossibleCauses []string   `json:"possible_causes,omitempty"`
	Recommendation string     `json:"recommendation"`
	Evidence       Evidence   `json:"evidence"`
}

// Suite is the complete in-memory evidence boundary for diagnosis. Rules read
// it only after all available experiments have stopped.
type Suite struct {
	Baseline    runner.ExperimentReport   `json:"baseline"`
	Connection  runner.ConnectionReport   `json:"connection"`
	Concurrency []runner.ExperimentReport `json:"concurrency"`
}

// Report is the deterministic result of evaluating one suite.
type Report struct {
	RuleVersion string    `json:"rule_version"`
	Findings    []Finding `json:"findings"`
	EvaluatedAt time.Time `json:"evaluated_at,omitempty"`
}
