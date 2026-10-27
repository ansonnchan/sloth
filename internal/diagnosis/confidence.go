package diagnosis

import "fmt"

// EvidenceProfile describes evidence quality independently of whether a rule's
// diagnostic threshold was crossed.
type EvidenceProfile struct {
	Expected     int
	Available    int
	Consistent   bool
	Contradicted bool
	Interrupted  bool
	UsesP99      bool
}

// Assessment tells a rule whether it may speak and how strongly. Reasons are
// stable user-facing limitations suitable for an inconclusive finding.
type Assessment struct {
	Adequate   bool
	Confidence Confidence
	Reasons    []string
}

// AssessEvidence applies only evidence-quality policy. It does not inspect
// latency or error values and therefore cannot decide whether a rule matches.
func AssessEvidence(profile EvidenceProfile, thresholds Thresholds) Assessment {
	assessment := Assessment{Confidence: ConfidenceLow}
	if profile.Expected <= 0 {
		assessment.Reasons = append(assessment.Reasons, "expected sample count is unavailable")
		return assessment
	}
	if profile.Available < 0 || profile.Available > profile.Expected {
		assessment.Reasons = append(assessment.Reasons, "available sample count is inconsistent with the experiment plan")
		return assessment
	}

	completeness := float64(profile.Available) / float64(profile.Expected)
	if profile.Available < thresholds.MinimumSamples {
		assessment.Reasons = append(assessment.Reasons,
			fmt.Sprintf("only %d samples are available; at least %d are required", profile.Available, thresholds.MinimumSamples))
	}
	if completeness < thresholds.MinimumCompleteness {
		assessment.Reasons = append(assessment.Reasons,
			fmt.Sprintf("sample completeness %.0f%% is below the %.0f%% minimum", completeness*100, thresholds.MinimumCompleteness*100))
	}
	if len(assessment.Reasons) > 0 {
		return assessment
	}

	assessment.Adequate = true
	if profile.Available >= thresholds.HighSamples && completeness >= thresholds.HighCompleteness {
		assessment.Confidence = ConfidenceHigh
	} else if profile.Available >= thresholds.MediumSamples && completeness >= thresholds.MediumCompleteness {
		assessment.Confidence = ConfidenceMedium
	}

	if profile.UsesP99 && profile.Available < thresholds.HighPercentileSamples && assessment.Confidence == ConfidenceHigh {
		assessment.Confidence = ConfidenceMedium
		assessment.Reasons = append(assessment.Reasons,
			fmt.Sprintf("p99 confidence is capped below %d samples", thresholds.HighPercentileSamples))
	}
	if !profile.Consistent {
		assessment.Confidence = ConfidenceLow
		assessment.Reasons = append(assessment.Reasons, "evidence is not consistent across the evaluated measurements")
	}
	if profile.Contradicted {
		assessment.Confidence = ConfidenceLow
		assessment.Reasons = append(assessment.Reasons, "another observed metric contradicts the diagnostic signal")
	}
	if profile.Interrupted {
		assessment.Confidence = ConfidenceLow
		assessment.Reasons = append(assessment.Reasons, "the experiment was interrupted before its planned evidence was complete")
	}
	return assessment
}
