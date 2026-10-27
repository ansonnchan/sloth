package diagnosis

import (
	"fmt"
	"strings"
)

// ConnectionRule compares observed fresh and reused connection arms.
type ConnectionRule struct{}

func (ConnectionRule) Code() Code { return CodeConnectionOverhead }

func (ConnectionRule) Evaluate(suite Suite, thresholds Thresholds) Evaluation {
	report := suite.Connection
	if !report.Sufficient {
		reasons := append([]string(nil), report.Limitations...)
		if len(reasons) == 0 {
			reasons = append(reasons, "connection arms do not contain observed fresh and reused connection evidence")
		}
		return Evaluation{Limitations: reasons}
	}

	fresh, reused := report.Fresh.Aggregate, report.Reused.Aggregate
	if !fresh.Latency.Available || !reused.Latency.Available || fresh.Latency.Median <= 0 {
		return Evaluation{Limitations: []string{"fresh or reused connection latency is unavailable"}}
	}
	expected := report.Fresh.Result.Plan.Requests() + report.Reused.Result.Plan.Requests()
	available := fresh.Latency.Count + reused.Latency.Count
	contradicted := fresh.Latency.P95 < reused.Latency.P95
	assessment := AssessEvidence(EvidenceProfile{
		Expected: expected, Available: available,
		Consistent:   fresh.Latency.Count == reused.Latency.Count,
		Contradicted: contradicted,
		Interrupted:  fresh.Interrupted || reused.Interrupted,
	}, thresholds)
	if !assessment.Adequate {
		return Evaluation{Limitations: assessment.Reasons}
	}

	delta := fresh.Latency.Median - reused.Latency.Median
	deltaRatio := safeRatio(delta, fresh.Latency.Median)
	if delta < thresholds.ConnectionMinimum || deltaRatio < thresholds.ConnectionDeltaRatio {
		return Evaluation{Limitations: assessment.Reasons}
	}

	notes := append([]string(nil), assessment.Reasons...)
	for _, limitation := range report.Limitations {
		if strings.Contains(strings.ToLower(limitation), "http/2") || strings.Contains(strings.ToLower(limitation), "proxy") {
			assessment.Confidence = lowerConfidence(assessment.Confidence)
			notes = append(notes, limitation)
		}
	}
	connect := fresh.Phases["connect"].Median
	tls := fresh.Phases["tls"].Median
	finding := Finding{
		Code:       CodeConnectionOverhead,
		Severity:   SeverityWarning,
		Confidence: assessment.Confidence,
		Title:      "Fresh connections added measurable latency",
		Observation: fmt.Sprintf(
			"Fresh-connection median latency was %.1f ms versus %.1f ms with reuse, a %.1f ms (%.0f%%) difference.",
			durationMilliseconds(fresh.Latency.Median), durationMilliseconds(reused.Latency.Median),
			durationMilliseconds(delta), deltaRatio*100,
		),
		PossibleCauses: []string{
			"TCP connection establishment",
			"TLS handshake work",
			"connection-pool churn or disabled keep-alive",
		},
		Recommendation: "Reuse a bounded HTTP transport and connection pool, then repeat the alternating comparison.",
		Evidence: Evidence{
			Metrics: []Metric{
				{Name: "fresh_samples", Value: float64(fresh.Latency.Count), Unit: UnitCount},
				{Name: "reused_samples", Value: float64(reused.Latency.Count), Unit: UnitCount},
				{Name: "fresh_new_connections", Value: float64(fresh.Connections.New), Unit: UnitCount},
				{Name: "reused_connections", Value: float64(reused.Connections.Reused), Unit: UnitCount},
				{Name: "fresh_median", Value: durationMilliseconds(fresh.Latency.Median), Unit: UnitMilliseconds},
				{Name: "reused_median", Value: durationMilliseconds(reused.Latency.Median), Unit: UnitMilliseconds},
				{Name: "latency_delta", Value: durationMilliseconds(delta), Unit: UnitMilliseconds},
				{Name: "latency_delta_ratio", Value: deltaRatio, Unit: UnitRatio},
				{Name: "fresh_connect_median", Value: durationMilliseconds(connect), Unit: UnitMilliseconds},
				{Name: "fresh_tls_median", Value: durationMilliseconds(tls), Unit: UnitMilliseconds},
			},
			Notes: notes,
		},
	}
	return Evaluation{Finding: &finding}
}
