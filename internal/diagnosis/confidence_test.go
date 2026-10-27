package diagnosis

import "testing"

func TestAssessEvidenceBoundaries(t *testing.T) {
	thresholds := DefaultThresholds()
	tests := []struct {
		name       string
		profile    EvidenceProfile
		adequate   bool
		confidence Confidence
	}{
		{
			name: "below minimum samples", profile: EvidenceProfile{
				Expected: 100, Available: thresholds.MinimumSamples - 1, Consistent: true,
			}, confidence: ConfidenceLow,
		},
		{
			name: "at minimum samples", profile: EvidenceProfile{
				Expected: thresholds.MinimumSamples, Available: thresholds.MinimumSamples, Consistent: true,
			}, adequate: true, confidence: ConfidenceLow,
		},
		{
			name: "below minimum completeness", profile: EvidenceProfile{
				Expected: 100, Available: 49, Consistent: true,
			}, confidence: ConfidenceLow,
		},
		{
			name: "at medium transition", profile: EvidenceProfile{
				Expected: 30, Available: 30, Consistent: true,
			}, adequate: true, confidence: ConfidenceMedium,
		},
		{
			name: "sample count blocks medium", profile: EvidenceProfile{
				Expected: 29, Available: 29, Consistent: true,
			}, adequate: true, confidence: ConfidenceLow,
		},
		{
			name: "completeness blocks medium", profile: EvidenceProfile{
				Expected: 40, Available: 31, Consistent: true,
			}, adequate: true, confidence: ConfidenceLow,
		},
		{
			name: "at high transition", profile: EvidenceProfile{
				Expected: 100, Available: 100, Consistent: true,
			}, adequate: true, confidence: ConfidenceHigh,
		},
		{
			name: "completeness blocks high", profile: EvidenceProfile{
				Expected: 106, Available: 100, Consistent: true,
			}, adequate: true, confidence: ConfidenceMedium,
		},
		{
			name: "p99 caps default suite", profile: EvidenceProfile{
				Expected: 100, Available: 100, Consistent: true, UsesP99: true,
			}, adequate: true, confidence: ConfidenceMedium,
		},
		{
			name: "p99 permits high at boundary", profile: EvidenceProfile{
				Expected: thresholds.HighPercentileSamples, Available: thresholds.HighPercentileSamples,
				Consistent: true, UsesP99: true,
			}, adequate: true, confidence: ConfidenceHigh,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := AssessEvidence(test.profile, thresholds)
			if got.Adequate != test.adequate || got.Confidence != test.confidence {
				t.Fatalf("assessment = %+v, want adequate=%t confidence=%s", got, test.adequate, test.confidence)
			}
		})
	}
}

func TestAssessEvidenceDowngradesConfoundingEvidence(t *testing.T) {
	base := EvidenceProfile{Expected: 200, Available: 200, Consistent: true}
	tests := []struct {
		name   string
		mutate func(*EvidenceProfile)
	}{
		{"inconsistent", func(profile *EvidenceProfile) { profile.Consistent = false }},
		{"contradicted", func(profile *EvidenceProfile) { profile.Contradicted = true }},
		{"interrupted", func(profile *EvidenceProfile) { profile.Interrupted = true }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			profile := base
			test.mutate(&profile)
			got := AssessEvidence(profile, DefaultThresholds())
			if !got.Adequate || got.Confidence != ConfidenceLow || len(got.Reasons) == 0 {
				t.Fatalf("assessment = %+v, want adequate low-confidence result with a reason", got)
			}
		})
	}
}

func TestAssessEvidenceSuppressesInvalidCounts(t *testing.T) {
	for _, profile := range []EvidenceProfile{
		{},
		{Expected: 10, Available: -1, Consistent: true},
		{Expected: 10, Available: 11, Consistent: true},
	} {
		got := AssessEvidence(profile, DefaultThresholds())
		if got.Adequate || len(got.Reasons) == 0 {
			t.Fatalf("assessment = %+v, want suppressed result with a reason", got)
		}
	}
}
