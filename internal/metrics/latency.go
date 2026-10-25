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
	statistics.Median = percentile(sorted, 50)
	statistics.P90 = percentile(sorted, 90)
	statistics.P95 = percentile(sorted, 95)
	statistics.P99 = percentile(sorted, 99)
	statistics.PopulationStdDev = roundedDuration(deviation)
	return statistics, nil
}

func percentile(sorted []time.Duration, percent int64) time.Duration {
	position := int64(len(sorted)-1) * percent
	lower := int(position / 100)
	remainder := position % 100
	if remainder == 0 {
		return sorted[lower]
	}
	delta := int64(sorted[lower+1] - sorted[lower])
	// Splitting the product keeps interpolation exact without overflowing when
	// durations approach time.Duration's maximum value.
	offset := (delta/100)*remainder + ((delta%100)*remainder+50)/100
	return sorted[lower] + time.Duration(offset)
}

func roundedDuration(nanoseconds float64) time.Duration {
	if nanoseconds >= float64(math.MaxInt64) {
		return time.Duration(math.MaxInt64)
	}
	return time.Duration(math.Round(nanoseconds))
}
