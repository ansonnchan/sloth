package diagnosis

import (
	"fmt"

	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/runner"
)

// ConcurrencyRule evaluates response behavior only against achieved load. It
// reports client scheduling pressure separately from endpoint observations.
type ConcurrencyRule struct{}

func (ConcurrencyRule) Code() Code { return CodeConcurrencySensitivity }

func (ConcurrencyRule) Evaluate(suite Suite, thresholds Thresholds) Evaluation {
	if len(suite.Concurrency) < 2 {
		return Evaluation{Limitations: []string{"at least two concurrency stages are required"}}
	}
	reference, loaded := achievedLoadExtremes(suite.Concurrency)
	if reference.Aggregate.Rates.AchievedPerSec <= 0 {
		return Evaluation{Limitations: []string{"achieved request rate is unavailable for the reference stage"}}
	}
	loadChange := (loaded.Aggregate.Rates.AchievedPerSec - reference.Aggregate.Rates.AchievedPerSec) /
		reference.Aggregate.Rates.AchievedPerSec
	if loadChange < thresholds.MinimumLoadChange {
		return Evaluation{Limitations: []string{"concurrency stages did not produce a meaningful achieved-load change"}}
	}

	expected, available, interrupted := concurrencyEvidenceCounts(suite.Concurrency)
	refLatency, loadLatency := reference.Aggregate.Latency, loaded.Aggregate.Latency
	if !refLatency.Available || !loadLatency.Available || refLatency.P95 <= 0 || loadLatency.Median <= 0 {
		return Evaluation{Limitations: []string{"latency is unavailable for one or more concurrency stages"}}
	}
	assessment := AssessEvidence(EvidenceProfile{
		Expected: expected, Available: available,
		Consistent:   concurrencyCountsConsistent(suite.Concurrency),
		Contradicted: loadLatency.Median < refLatency.Median && loadLatency.P95 >= refLatency.P95,
		Interrupted:  interrupted,
	}, thresholds)
	if !assessment.Adequate {
		return Evaluation{Limitations: assessment.Reasons}
	}

	p95Ratio := safeRatio(loadLatency.P95, refLatency.P95)
	refErrors := errorRate(reference.Aggregate.Outcomes)
	loadErrors := errorRate(loaded.Aggregate.Outcomes)
	errorIncrease := loadErrors-refErrors >= thresholds.WarningErrorRate
	scheduleShare := safeRatio(loaded.Aggregate.SchedulingDelay.Median, loadLatency.Median)
	missedShare := missedStartShare(loaded.Aggregate.Rates)
	schedulingPressure := scheduleShare >= thresholds.SchedulingShare || missedShare >= thresholds.SchedulingShare
	latencySensitive := p95Ratio >= thresholds.ConcurrencyP95Ratio
	if !latencySensitive && !errorIncrease && !schedulingPressure {
		return Evaluation{}
	}

	title := "Endpoint behavior changed as achieved concurrency increased"
	severity := SeverityWarning
	causes := []string{"endpoint queueing or resource contention", "dependency saturation", "workload-sensitive response time"}
	recommendation := "Correlate the loaded stage with server capacity signals and repeat at bounded intermediate stages."
	if schedulingPressure {
		title = "Client scheduling pressure affected the concurrency experiment"
		severity = SeverityInfo
		causes = []string{"local CPU or runtime contention", "client connection limits", "the offered schedule exceeding achieved client capacity"}
		recommendation = "Reduce offered rate or concurrency until scheduling delay is small, then repeat before attributing changes to the endpoint."
		assessment.Confidence = lowerConfidence(assessment.Confidence)
		assessment.Reasons = append(assessment.Reasons, "client scheduling pressure confounds endpoint interpretation")
	}

	finding := Finding{
		Code:       CodeConcurrencySensitivity,
		Severity:   severity,
		Confidence: assessment.Confidence,
		Title:      title,
		Observation: fmt.Sprintf(
			"Achieved rate increased %.0f%%; loaded-stage p95 was %.2fx reference, error rate changed from %.1f%% to %.1f%%, and scheduling delay was %.0f%% of median latency.",
			loadChange*100, p95Ratio, refErrors*100, loadErrors*100, scheduleShare*100,
		),
		PossibleCauses: causes,
		Recommendation: recommendation,
		Evidence:       Evidence{Metrics: concurrencyMetrics(suite.Concurrency), Notes: assessment.Reasons},
	}
	return Evaluation{Finding: &finding}
}

func achievedLoadExtremes(reports []runner.ExperimentReport) (runner.ExperimentReport, runner.ExperimentReport) {
	low, high := reports[0], reports[0]
	for _, report := range reports[1:] {
		if report.Aggregate.Rates.AchievedPerSec < low.Aggregate.Rates.AchievedPerSec {
			low = report
		}
		if report.Aggregate.Rates.AchievedPerSec > high.Aggregate.Rates.AchievedPerSec {
			high = report
		}
	}
	return low, high
}

func concurrencyEvidenceCounts(reports []runner.ExperimentReport) (expected, available int, interrupted bool) {
	for _, report := range reports {
		expected += report.Result.Plan.Requests()
		available += report.Aggregate.Latency.Count
		interrupted = interrupted || report.Aggregate.Interrupted
	}
	return expected, available, interrupted
}

func concurrencyCountsConsistent(reports []runner.ExperimentReport) bool {
	if len(reports) == 0 {
		return false
	}
	count := reports[0].Aggregate.Latency.Count
	for _, report := range reports[1:] {
		if report.Aggregate.Latency.Count != count {
			return false
		}
	}
	return true
}

func errorRate(outcomes metrics.OutcomeCounts) float64 {
	if outcomes.Attempted <= 0 {
		return 0
	}
	errors := outcomes.Timeout + outcomes.TransportError + outcomes.HTTPError + outcomes.BodyLimit
	return float64(errors) / float64(outcomes.Attempted)
}

func missedStartShare(rates metrics.RateMetrics) float64 {
	if rates.OfferedPerSec <= 0 || rates.AchievedPerSec >= rates.OfferedPerSec {
		return 0
	}
	return (rates.OfferedPerSec - rates.AchievedPerSec) / rates.OfferedPerSec
}

func concurrencyMetrics(reports []runner.ExperimentReport) []Metric {
	values := make([]Metric, 0, len(reports)*9)
	for index, report := range reports {
		prefix := fmt.Sprintf("stage_%d_", index+1)
		aggregate := report.Aggregate
		errors := aggregate.Outcomes.Timeout + aggregate.Outcomes.TransportError + aggregate.Outcomes.HTTPError + aggregate.Outcomes.BodyLimit
		values = append(values,
			Metric{Name: prefix + "offered_rate", Value: aggregate.Rates.OfferedPerSec, Unit: UnitRequestsPerSecond},
			Metric{Name: prefix + "achieved_rate", Value: aggregate.Rates.AchievedPerSec, Unit: UnitRequestsPerSecond},
			Metric{Name: prefix + "actual_concurrency", Value: aggregate.Concurrency.TimeWeightedInFlight, Unit: UnitCount},
			Metric{Name: prefix + "peak_concurrency", Value: float64(aggregate.Concurrency.Peak), Unit: UnitCount},
			Metric{Name: prefix + "p50", Value: durationMilliseconds(aggregate.Latency.Median), Unit: UnitMilliseconds},
			Metric{Name: prefix + "p95", Value: durationMilliseconds(aggregate.Latency.P95), Unit: UnitMilliseconds},
			Metric{Name: prefix + "successes", Value: float64(aggregate.Outcomes.Successful), Unit: UnitCount},
			Metric{Name: prefix + "errors", Value: float64(errors), Unit: UnitCount},
			Metric{Name: prefix + "scheduling_p50", Value: durationMilliseconds(aggregate.SchedulingDelay.Median), Unit: UnitMilliseconds},
		)
	}
	return values
}
