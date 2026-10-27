package diagnosis

import "time"

const CurrentRuleVersion = "1.0.0"

// Thresholds is the single versioned policy surface for diagnostic rules.
// Duration fields use monotonic elapsed durations produced by measurement.
type Thresholds struct {
	RuleVersion string

	MinimumSamples        int
	MediumSamples         int
	HighSamples           int
	HighPercentileSamples int
	MinimumCompleteness   float64
	MediumCompleteness    float64
	HighCompleteness      float64

	FirstByteShare       float64
	FirstByteMinimum     time.Duration
	ConnectionDeltaRatio float64
	ConnectionMinimum    time.Duration
	TailRatio            float64
	TailMinimumGap       time.Duration
	MinimumLoadChange    float64
	ConcurrencyP95Ratio  float64
	SchedulingShare      float64
	WarningErrorRate     float64
	HighErrorRate        float64
	ClientErrorRate      float64
}

// DefaultThresholds returns a fresh copy of the v1 diagnostic policy.
func DefaultThresholds() Thresholds {
	return Thresholds{
		RuleVersion: CurrentRuleVersion,

		MinimumSamples:        10,
		MediumSamples:         30,
		HighSamples:           100,
		HighPercentileSamples: 200,
		MinimumCompleteness:   0.50,
		MediumCompleteness:    0.80,
		HighCompleteness:      0.95,

		FirstByteShare:       0.60,
		FirstByteMinimum:     100 * time.Millisecond,
		ConnectionDeltaRatio: 0.20,
		ConnectionMinimum:    20 * time.Millisecond,
		TailRatio:            2.0,
		TailMinimumGap:       100 * time.Millisecond,
		MinimumLoadChange:    0.25,
		ConcurrencyP95Ratio:  1.50,
		SchedulingShare:      0.20,
		WarningErrorRate:     0.01,
		HighErrorRate:        0.05,
		ClientErrorRate:      0.10,
	}
}
