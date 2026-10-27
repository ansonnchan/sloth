package diagnosis

import "fmt"

// TailRule detects a large and absolute p95-to-median spread in baseline data.
type TailRule struct{}

func (TailRule) Code() Code { return CodeTailVariability }

func (TailRule) Evaluate(suite Suite, thresholds Thresholds) Evaluation {
	aggregate := suite.Baseline.Aggregate
	latency := aggregate.Latency
	if !latency.Available || latency.Median <= 0 {
		return Evaluation{Limitations: []string{"baseline latency distribution is unavailable"}}
	}
	assessment := AssessEvidence(EvidenceProfile{
		Expected:     suite.Baseline.Result.Plan.Requests(),
		Available:    latency.Count,
		Consistent:   latency.P95 >= latency.Median,
		Contradicted: latency.P99 < latency.P95,
		Interrupted:  aggregate.Interrupted,
	}, thresholds)
	if !assessment.Adequate {
		return Evaluation{Limitations: assessment.Reasons}
	}

	ratio := safeRatio(latency.P95, latency.Median)
	gap := latency.P95 - latency.Median
	if ratio < thresholds.TailRatio || gap < thresholds.TailMinimumGap {
		return Evaluation{Limitations: assessment.Reasons}
	}

	notes := append([]string(nil), assessment.Reasons...)
	invalid := aggregate.Outcomes.Attempted - aggregate.Outcomes.Completed
	if aggregate.Outcomes.Attempted > 0 && float64(invalid)/float64(aggregate.Outcomes.Attempted) > 1-thresholds.MediumCompleteness {
		assessment.Confidence = lowerConfidence(assessment.Confidence)
		notes = append(notes, "more than 20% of attempted requests lacked a completed HTTP outcome")
	}
	notes = append(notes, "p99 is descriptive tail context and needs at least 200 samples for high-confidence interpretation")
	finding := Finding{
		Code:       CodeTailVariability,
		Severity:   SeverityWarning,
		Confidence: assessment.Confidence,
		Title:      "Baseline latency had a wide tail",
		Observation: fmt.Sprintf(
			"Baseline p95 latency was %.1f ms versus a %.1f ms median, a %.1fx ratio and %.1f ms gap.",
			durationMilliseconds(latency.P95), durationMilliseconds(latency.Median), ratio,
			durationMilliseconds(gap),
		),
		PossibleCauses: []string{
			"intermittent endpoint or dependency work",
			"queueing under variable load",
			"network-path variability",
			"runtime pauses or resource contention",
		},
		Recommendation: "Inspect slow individual samples and repeat the same workload to determine whether the tail pattern persists.",
		Evidence: Evidence{
			Metrics: []Metric{
				{Name: "sample_count", Value: float64(latency.Count), Unit: UnitCount},
				{Name: "p50", Value: durationMilliseconds(latency.Median), Unit: UnitMilliseconds},
				{Name: "p95", Value: durationMilliseconds(latency.P95), Unit: UnitMilliseconds},
				{Name: "p99", Value: durationMilliseconds(latency.P99), Unit: UnitMilliseconds},
				{Name: "p95_to_p50_ratio", Value: ratio, Unit: UnitRatio},
				{Name: "p95_to_p50_gap", Value: durationMilliseconds(gap), Unit: UnitMilliseconds},
				{Name: "incomplete_outcomes", Value: float64(invalid), Unit: UnitCount},
			},
			Notes: notes,
		},
	}
	return Evaluation{Finding: &finding}
}
