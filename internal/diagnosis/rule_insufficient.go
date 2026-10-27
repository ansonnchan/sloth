package diagnosis

import (
	"fmt"
	"strings"
)

// InsufficientEvidenceRule turns concrete rule limitations into one honest,
// actionable inconclusive result. It never proposes a root cause.
type InsufficientEvidenceRule struct{}

func (InsufficientEvidenceRule) Code() Code { return CodeInsufficientEvidence }

// Evaluate exists to satisfy Rule. The engine invokes EvaluateSummary so this
// rule can consider prior deterministic evaluations.
func (InsufficientEvidenceRule) Evaluate(Suite, Thresholds) Evaluation {
	return Evaluation{Limitations: []string{"insufficient-evidence rule requires prior rule evaluations"}}
}

func (InsufficientEvidenceRule) EvaluateSummary(_ Suite, _ Thresholds, prior []Evaluation) Evaluation {
	reasons := make([]string, 0)
	hasSupportedFinding := false
	for _, evaluation := range prior {
		if evaluation.Finding != nil && evaluation.Finding.Confidence != ConfidenceLow {
			hasSupportedFinding = true
		}
		for _, limitation := range evaluation.Limitations {
			reasons = appendUnique(reasons, limitation)
		}
	}
	if hasSupportedFinding || len(reasons) == 0 {
		return Evaluation{}
	}

	recommendation := nextMeasurement(reasons)
	finding := Finding{
		Code:           CodeInsufficientEvidence,
		Severity:       SeverityInfo,
		Confidence:     ConfidenceLow,
		Title:          "The available experiments do not support a stronger diagnosis",
		Observation:    fmt.Sprintf("Diagnosis was inconclusive for %d evidence limitation(s).", len(reasons)),
		PossibleCauses: nil,
		Recommendation: recommendation,
		Evidence: Evidence{
			Metrics: []Metric{{Name: "limitation_count", Value: float64(len(reasons)), Unit: UnitCount}},
			Notes:   reasons,
		},
	}
	return Evaluation{Finding: &finding}
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func nextMeasurement(reasons []string) string {
	joined := strings.ToLower(strings.Join(reasons, " "))
	switch {
	case strings.Contains(joined, "achieved-load"):
		return "Repeat with safely separated offered rates that produce a measurable achieved-load change."
	case strings.Contains(joined, "fresh") || strings.Contains(joined, "reused") || strings.Contains(joined, "connection arms"):
		return "Repeat the alternating connection experiment and verify that both fresh and reused states are observed."
	case strings.Contains(joined, "interrupted"):
		return "Repeat the same bounded suite without interruption and retain all partial results until then."
	case strings.Contains(joined, "unavailable"):
		return "Repeat the bounded suite with the required timing phases available and inspect transport limitations."
	default:
		return "Repeat the same bounded suite with enough complete samples to satisfy the documented evidence minimums."
	}
}
