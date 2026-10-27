package diagnosis

import (
	"fmt"
	"strings"

	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/runner"
)

// ErrorRule evaluates timeout, transport, 5xx, and 4xx classes separately.
type ErrorRule struct{}

func (ErrorRule) Code() Code { return CodeErrorRate }

func (ErrorRule) Evaluate(suite Suite, thresholds Thresholds) Evaluation {
	counts, expected, interrupted := suiteOutcomes(suite)
	eligible := counts.Attempted - counts.Cancelled
	if eligible <= 0 {
		return Evaluation{Limitations: []string{"no non-cancelled request outcomes are available"}}
	}
	consistent := outcomesConsistent(counts)
	assessment := AssessEvidence(EvidenceProfile{
		Expected: expected, Available: eligible, Consistent: consistent,
		Contradicted: !consistent, Interrupted: interrupted,
	}, thresholds)
	if !assessment.Adequate {
		return Evaluation{Limitations: assessment.Reasons}
	}

	classes := []errorClass{
		{name: "timeouts", count: counts.Timeout, threshold: thresholds.WarningErrorRate,
			cause: "request deadlines shorter than observed work or stalled network activity"},
		{name: "transport errors", count: counts.TransportError, threshold: thresholds.WarningErrorRate,
			cause: "DNS, connection, TLS, or connection-lifecycle failures"},
		{name: "HTTP 5xx responses", count: counts.serverErrors, threshold: thresholds.WarningErrorRate,
			cause: "the endpoint or an upstream dependency returning server errors"},
		{name: "HTTP 4xx responses", count: counts.clientErrors, threshold: thresholds.ClientErrorRate,
			cause: "request validation, authentication, authorization, or target mismatch"},
	}
	crossed := make([]errorClass, 0, len(classes))
	for _, class := range classes {
		class.rate = float64(class.count) / float64(eligible)
		if class.rate >= class.threshold {
			crossed = append(crossed, class)
		}
	}
	if len(crossed) == 0 {
		return Evaluation{}
	}

	names := make([]string, 0, len(crossed))
	causes := make([]string, 0, len(crossed))
	severity := SeverityWarning
	for _, class := range crossed {
		names = append(names, fmt.Sprintf("%s %.1f%%", class.name, class.rate*100))
		causes = append(causes, class.cause)
		if class.rate >= thresholds.HighErrorRate && class.name != "HTTP 4xx responses" {
			severity = SeverityHigh
		}
	}
	finding := Finding{
		Code:       CodeErrorRate,
		Severity:   severity,
		Confidence: assessment.Confidence,
		Title:      "One or more request outcome classes crossed their error threshold",
		Observation: fmt.Sprintf(
			"Across %d non-cancelled attempts, threshold crossings were: %s.", eligible, strings.Join(names, ", "),
		),
		PossibleCauses: causes,
		Recommendation: "Investigate each crossed outcome class separately, correct request or transport failures, and repeat the same bounded workload.",
		Evidence: Evidence{
			Metrics: []Metric{
				{Name: "eligible_attempts", Value: float64(eligible), Unit: UnitCount},
				{Name: "cancelled_excluded", Value: float64(counts.Cancelled), Unit: UnitCount},
				{Name: "timeouts", Value: float64(counts.Timeout), Unit: UnitCount},
				{Name: "timeout_rate", Value: float64(counts.Timeout) / float64(eligible), Unit: UnitRatio},
				{Name: "transport_errors", Value: float64(counts.TransportError), Unit: UnitCount},
				{Name: "transport_error_rate", Value: float64(counts.TransportError) / float64(eligible), Unit: UnitRatio},
				{Name: "http_5xx", Value: float64(counts.serverErrors), Unit: UnitCount},
				{Name: "http_5xx_rate", Value: float64(counts.serverErrors) / float64(eligible), Unit: UnitRatio},
				{Name: "http_4xx", Value: float64(counts.clientErrors), Unit: UnitCount},
				{Name: "http_4xx_rate", Value: float64(counts.clientErrors) / float64(eligible), Unit: UnitRatio},
			},
			Notes: assessment.Reasons,
		},
	}
	return Evaluation{Finding: &finding}
}

type outcomeTotals struct {
	metrics.OutcomeCounts
	clientErrors int
	serverErrors int
}

type errorClass struct {
	name      string
	count     int
	rate      float64
	threshold float64
	cause     string
}

func suiteOutcomes(suite Suite) (outcomeTotals, int, bool) {
	reports := []runner.ExperimentReport{suite.Baseline, suite.Connection.Fresh, suite.Connection.Reused}
	reports = append(reports, suite.Concurrency...)
	var totals outcomeTotals
	expected := 0
	interrupted := false
	for _, report := range reports {
		if report.Result.Plan.Requests() == 0 && report.Aggregate.Outcomes.Attempted == 0 {
			continue
		}
		expected += report.Result.Plan.Requests()
		outcomes := report.Aggregate.Outcomes
		totals.Attempted += outcomes.Attempted
		totals.Completed += outcomes.Completed
		totals.Successful += outcomes.Successful
		totals.HTTPError += outcomes.HTTPError
		totals.TransportError += outcomes.TransportError
		totals.Timeout += outcomes.Timeout
		totals.Cancelled += outcomes.Cancelled
		totals.BodyLimit += outcomes.BodyLimit
		totals.clientErrors += report.Aggregate.StatusClasses["4xx"]
		totals.serverErrors += report.Aggregate.StatusClasses["5xx"]
		interrupted = interrupted || report.Aggregate.Interrupted
	}
	return totals, expected, interrupted
}

func outcomesConsistent(counts outcomeTotals) bool {
	classified := counts.Successful + counts.HTTPError + counts.TransportError + counts.Timeout + counts.Cancelled + counts.BodyLimit
	return counts.Attempted >= 0 && classified <= counts.Attempted && counts.clientErrors+counts.serverErrors <= counts.HTTPError
}
