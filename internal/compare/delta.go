package compare

import (
	"errors"
	"fmt"
	"sort"

	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/report"
)

type Direction string

const (
	LowerIsBetter  Direction = "lower_is_better"
	HigherIsBetter Direction = "higher_is_better"
	Neutral        Direction = "neutral"
)

type MetricDelta struct {
	Name          string    `json:"name"`
	Unit          string    `json:"unit"`
	Direction     Direction `json:"direction"`
	Before        *float64  `json:"before"`
	After         *float64  `json:"after"`
	Absolute      *float64  `json:"absolute"`
	PercentChange *float64  `json:"percent_change"`
}

type ExperimentDelta struct {
	ID      string          `json:"id"`
	Kind    experiment.Kind `json:"kind"`
	Metrics []MetricDelta   `json:"metrics"`
}

type DeltaReport struct {
	Experiments []ExperimentDelta `json:"experiments"`
}

// Calculate accepts only the result of a compatibility decision. Gate mode
// requires exact eligibility; descriptive mode may calculate retained pairs
// while its caller presents the associated warnings.
func Calculate(compatibility Compatibility) (DeltaReport, error) {
	if !compatibility.DescriptiveEligible {
		return DeltaReport{}, errors.New("calculate deltas: reports have no descriptively compatible experiment pairs")
	}
	if compatibility.Mode == ModeGate && !compatibility.GateEligible {
		return DeltaReport{}, errors.New("calculate deltas: reports are not eligible for a regression gate")
	}
	result := DeltaReport{Experiments: make([]ExperimentDelta, 0, len(compatibility.Pairs))}
	for _, pair := range compatibility.Pairs {
		result.Experiments = append(result.Experiments, calculateExperiment(pair))
	}
	return result, nil
}

func calculateExperiment(pair ExperimentPair) ExperimentDelta {
	before := pair.Before.Aggregate
	after := pair.After.Aggregate
	metrics := []MetricDelta{
		newDelta("latency.mean_ms", "ms", LowerIsBetter, before.Latency.MeanMillis, after.Latency.MeanMillis),
		newDelta("latency.p50_ms", "ms", LowerIsBetter, before.Latency.P50Millis, after.Latency.P50Millis),
		newDelta("latency.p90_ms", "ms", LowerIsBetter, before.Latency.P90Millis, after.Latency.P90Millis),
		newDelta("latency.p95_ms", "ms", LowerIsBetter, before.Latency.P95Millis, after.Latency.P95Millis),
		newDelta("latency.p99_ms", "ms", LowerIsBetter, before.Latency.P99Millis, after.Latency.P99Millis),
		newDelta("scheduling_delay.mean_ms", "ms", LowerIsBetter, before.SchedulingDelay.MeanMillis, after.SchedulingDelay.MeanMillis),
		newDelta("scheduling_delay.p50_ms", "ms", LowerIsBetter, before.SchedulingDelay.P50Millis, after.SchedulingDelay.P50Millis),
		newDelta("scheduling_delay.p95_ms", "ms", LowerIsBetter, before.SchedulingDelay.P95Millis, after.SchedulingDelay.P95Millis),
		newDelta("scheduling_delay.p99_ms", "ms", LowerIsBetter, before.SchedulingDelay.P99Millis, after.SchedulingDelay.P99Millis),
		newDelta("throughput.completed_per_second", "requests/s", HigherIsBetter, availableRate(before), availableRate(after)),
		newDelta("outcomes.errors", "count", LowerIsBetter, errorCount(before), errorCount(after)),
		newDelta("outcomes.error_rate_percent", "%", LowerIsBetter, errorRate(before), errorRate(after)),
		newDelta("outcomes.timeouts", "count", LowerIsBetter, timeoutCount(before), timeoutCount(after)),
		newDelta("outcomes.timeout_rate_percent", "%", LowerIsBetter, timeoutRate(before), timeoutRate(after)),
		newDelta("outcomes.success_rate_percent", "%", HigherIsBetter, successRate(before), successRate(after)),
		newDelta("connections.new", "count", LowerIsBetter, connectionCount(before, true), connectionCount(after, true)),
		newDelta("connections.reused", "count", HigherIsBetter, connectionCount(before, false), connectionCount(after, false)),
		newDelta("connections.reuse_rate_percent", "%", HigherIsBetter, reuseRate(before), reuseRate(after)),
		newDelta("concurrency.time_weighted_in_flight", "requests", Neutral, availableConcurrency(before), availableConcurrency(after)),
	}

	phaseNames := make(map[string]struct{}, len(before.Phases)+len(after.Phases))
	for name := range before.Phases {
		phaseNames[name] = struct{}{}
	}
	for name := range after.Phases {
		phaseNames[name] = struct{}{}
	}
	orderedPhases := make([]string, 0, len(phaseNames))
	for name := range phaseNames {
		orderedPhases = append(orderedPhases, name)
	}
	sort.Strings(orderedPhases)
	for _, name := range orderedPhases {
		beforePhase, beforeOK := before.Phases[name]
		afterPhase, afterOK := after.Phases[name]
		var beforeP50, beforeP95, afterP50, afterP95 *float64
		if beforeOK {
			beforeP50, beforeP95 = beforePhase.P50Millis, beforePhase.P95Millis
		}
		if afterOK {
			afterP50, afterP95 = afterPhase.P50Millis, afterPhase.P95Millis
		}
		metrics = append(metrics,
			newDelta(fmt.Sprintf("phases.%s.p50_ms", name), "ms", LowerIsBetter, beforeP50, afterP50),
			newDelta(fmt.Sprintf("phases.%s.p95_ms", name), "ms", LowerIsBetter, beforeP95, afterP95),
		)
	}
	return ExperimentDelta{ID: pair.ID, Kind: pair.Before.Kind, Metrics: metrics}
}

func newDelta(name, unit string, direction Direction, before, after *float64) MetricDelta {
	result := MetricDelta{Name: name, Unit: unit, Direction: direction, Before: cloneFloat(before), After: cloneFloat(after)}
	if before == nil || after == nil {
		return result
	}
	absolute := *after - *before
	result.Absolute = &absolute
	if *before != 0 {
		percent := absolute / *before * 100
		result.PercentChange = &percent
	}
	return result
}

func availableRate(value report.Aggregate) *float64 {
	if value.Outcomes.Attempted == 0 {
		return nil
	}
	return floatPointer(value.Rates.CompletedPerSecond)
}

func errorCount(value report.Aggregate) *float64 {
	if value.Outcomes.Attempted == 0 {
		return nil
	}
	errors := value.Outcomes.HTTPError + value.Outcomes.TransportError + value.Outcomes.BodyLimit
	return floatPointer(float64(errors))
}

func timeoutCount(value report.Aggregate) *float64 {
	if value.Outcomes.Attempted == 0 {
		return nil
	}
	return floatPointer(float64(value.Outcomes.Timeout))
}

func errorRate(value report.Aggregate) *float64 {
	count := errorCount(value)
	if count == nil {
		return nil
	}
	return floatPointer(*count / float64(value.Outcomes.Attempted) * 100)
}

func timeoutRate(value report.Aggregate) *float64 {
	count := timeoutCount(value)
	if count == nil {
		return nil
	}
	return floatPointer(*count / float64(value.Outcomes.Attempted) * 100)
}

func successRate(value report.Aggregate) *float64 {
	if value.Outcomes.Attempted == 0 {
		return nil
	}
	return floatPointer(float64(value.Outcomes.Successful) / float64(value.Outcomes.Attempted) * 100)
}

func connectionCount(value report.Aggregate, fresh bool) *float64 {
	if value.Connections.Observed == 0 {
		return nil
	}
	count := value.Connections.Reused
	if fresh {
		count = value.Connections.New
	}
	return floatPointer(float64(count))
}

func reuseRate(value report.Aggregate) *float64 {
	if value.Connections.Observed == 0 {
		return nil
	}
	return floatPointer(float64(value.Connections.Reused) / float64(value.Connections.Observed) * 100)
}

func availableConcurrency(value report.Aggregate) *float64 {
	if value.Outcomes.Attempted == 0 {
		return nil
	}
	return floatPointer(value.Concurrency.TimeWeightedInFlight)
}

func floatPointer(value float64) *float64 { return &value }

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	return floatPointer(*value)
}
