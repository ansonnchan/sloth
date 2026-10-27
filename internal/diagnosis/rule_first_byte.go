package diagnosis

import "fmt"

// FirstByteRule detects baseline latency dominated by waiting for response
// headers. It makes no claim about activity inside the remote service.
type FirstByteRule struct{}

func (FirstByteRule) Code() Code { return CodeHighFirstByteWait }

func (FirstByteRule) Evaluate(suite Suite, thresholds Thresholds) Evaluation {
	aggregate := suite.Baseline.Aggregate
	total := aggregate.Latency
	firstByte := aggregate.Phases["first_byte_wait"]
	expected := suite.Baseline.Result.Plan.Requests()
	if !total.Available || !firstByte.Available || total.Median <= 0 {
		return Evaluation{Limitations: []string{"baseline total latency or first-byte timing is unavailable"}}
	}

	contradicted := firstByte.Median > total.Median || firstByte.Count > total.Count
	assessment := AssessEvidence(EvidenceProfile{
		Expected:     expected,
		Available:    min(total.Count, firstByte.Count),
		Consistent:   total.Count == firstByte.Count,
		Contradicted: contradicted,
		Interrupted:  aggregate.Interrupted,
	}, thresholds)
	if !assessment.Adequate {
		return Evaluation{Limitations: assessment.Reasons}
	}

	share := safeRatio(firstByte.Median, total.Median)
	if firstByte.Median < thresholds.FirstByteMinimum || share < thresholds.FirstByteShare {
		return Evaluation{}
	}

	observation := fmt.Sprintf(
		"Baseline median first-byte wait was %.1f ms, %.0f%% of %.1f ms median total latency.",
		durationMilliseconds(firstByte.Median), share*100, durationMilliseconds(total.Median),
	)
	finding := Finding{
		Code:        CodeHighFirstByteWait,
		Severity:    SeverityWarning,
		Confidence:  assessment.Confidence,
		Title:       "Most baseline latency occurred before the first response byte",
		Observation: observation,
		PossibleCauses: []string{
			"request processing before response headers",
			"upstream dependency latency",
			"server-side queueing",
			"network delay not isolated by client-visible phases",
		},
		Recommendation: "Correlate this interval with server and dependency timing, then repeat the same bounded workload.",
		Evidence: Evidence{
			Metrics: []Metric{
				{Name: "sample_count", Value: float64(min(total.Count, firstByte.Count)), Unit: UnitCount},
				{Name: "first_byte_median", Value: durationMilliseconds(firstByte.Median), Unit: UnitMilliseconds},
				{Name: "total_median", Value: durationMilliseconds(total.Median), Unit: UnitMilliseconds},
				{Name: "first_byte_share", Value: share, Unit: UnitRatio},
			},
			Notes: assessment.Reasons,
		},
	}
	return Evaluation{Finding: &finding}
}
