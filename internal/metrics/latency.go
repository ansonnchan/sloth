// Package metrics derives deterministic aggregates from immutable samples.
package metrics

import (
	"errors"
	"math"
	"slices"
	"time"
)

// LatencyStatistics summarizes one bounded exact duration sample set.
type LatencyStatistics struct {
	Available        bool          `json:"available"`
	Count            int           `json:"count"`
	Min              time.Duration `json:"min"`
	Max              time.Duration `json:"max"`
	Mean             time.Duration `json:"mean"`
	Median           time.Duration `json:"median"`
	P90              time.Duration `json:"p90"`
	P95              time.Duration `json:"p95"`
	P99              time.Duration `json:"p99"`
	PopulationStdDev time.Duration `json:"population_stddev"`
}

// CalculateLatency sorts a copy of the complete bounded sample set. Percentiles
// use linear interpolation at (n-1)*p and deviation is population deviation.
func CalculateLatency(values []time.Duration) (LatencyStatistics, error) {
	statistics := LatencyStatistics{Count: len(values)}
	if len(values) == 0 {
		return statistics, nil
	}

	sorted := slices.Clone(values)
	for _, value := range sorted {
		if value < 0 {
			return LatencyStatistics{}, errors.New("calculate latency: duration must not be negative")
		}
	}
	slices.Sort(sorted)

	mean, m2 := 0.0, 0.0
	for index, value := range values {
		x := float64(value)
		delta := x - mean
		mean += delta / float64(index+1)
		m2 += delta * (x - mean)
	}
	deviation := math.Sqrt(m2 / float64(len(values)))

	statistics.Available = true
	statistics.Min = sorted[0]
	statistics.Max = sorted[len(sorted)-1]
	statistics.Mean = roundedDuration(mean)
	statistics.Median = percentile(sorted, 0.50)
	statistics.P90 = percentile(sorted, 0.90)
	statistics.P95 = percentile(sorted, 0.95)
	statistics.P99 = percentile(sorted, 0.99)
	statistics.PopulationStdDev = roundedDuration(deviation)
	return statistics, nil
}

func percentile(sorted []time.Duration, probability float64) time.Duration {
	position := float64(len(sorted)-1) * probability
	lower := int(math.Floor(position))
	upper := int(math.Ceil(position))
	if lower == upper {
		return sorted[lower]
	}
	fraction := position - float64(lower)
	value := float64(sorted[lower]) + (float64(sorted[upper])-float64(sorted[lower]))*fraction
	return roundedDuration(value)
}

func roundedDuration(nanoseconds float64) time.Duration {
	if nanoseconds >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(math.Round(nanoseconds))
}
