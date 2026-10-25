package metrics

import (
	"errors"
	"math"
	"sort"
	"time"

	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

// OutcomeCounts reconciles every attempted measured request by stable outcome.
type OutcomeCounts struct {
	Attempted      int `json:"attempted"`
	Completed      int `json:"completed"`
	Successful     int `json:"successful"`
	HTTPError      int `json:"http_error"`
	TransportError int `json:"transport_error"`
	Timeout        int `json:"timeout"`
	Cancelled      int `json:"cancelled"`
	BodyLimit      int `json:"body_limit"`
}

// RateMetrics distinguish requested arrivals, achieved starts, and completed
// request throughput over the complete experiment wall interval.
type RateMetrics struct {
	Duration       time.Duration `json:"duration"`
	OfferedStarts  int           `json:"offered_starts"`
	AchievedStarts int           `json:"achieved_starts"`
	OfferedPerSec  float64       `json:"offered_per_second"`
	AchievedPerSec float64       `json:"achieved_per_second"`
	Throughput     float64       `json:"completed_per_second"`
}

type ConcurrencyMetrics struct {
	Peak                 int     `json:"peak"`
	TimeWeightedInFlight float64 `json:"time_weighted_in_flight"`
}

type ConnectionMetrics struct {
	Observed int `json:"observed"`
	New      int `json:"new"`
	Reused   int `json:"reused"`
}

// Aggregate is derived after execution from immutable measured samples.
type Aggregate struct {
	Outcomes        OutcomeCounts                `json:"outcomes"`
	StatusClasses   map[string]int               `json:"status_classes"`
	ResponseBytes   int64                        `json:"response_bytes"`
	Latency         LatencyStatistics            `json:"latency"`
	SchedulingDelay LatencyStatistics            `json:"scheduling_delay"`
	Phases          map[string]LatencyStatistics `json:"phases"`
	Rates           RateMetrics                  `json:"rates"`
	Concurrency     ConcurrencyMetrics           `json:"concurrency"`
	Protocols       map[string]int               `json:"protocols"`
	Connections     ConnectionMetrics            `json:"connections"`
	Interrupted     bool                         `json:"interrupted"`
	Limitations     []string                     `json:"limitations,omitempty"`
}

// AggregateInput supplies execution metadata that is not inferable from an
// empty or partial sample set.
type AggregateInput struct {
	Samples   experiment.MeasuredSamples
	Execution experiment.Execution
}

func CalculateAggregate(input AggregateInput) (Aggregate, error) {
	aggregate := Aggregate{
		StatusClasses: make(map[string]int),
		Phases:        make(map[string]LatencyStatistics),
		Protocols:     make(map[string]int),
		Interrupted:   input.Execution.Interrupted,
	}
	totals := make([]time.Duration, 0, len(input.Samples))
	delays := make([]time.Duration, 0, len(input.Samples))
	phaseValues := map[string][]time.Duration{
		"dns": {}, "connect": {}, "tls": {}, "request_write": {},
		"first_byte_wait": {}, "body_transfer": {},
	}

	for _, sample := range input.Samples {
		aggregate.Outcomes.Attempted++
		countOutcome(&aggregate.Outcomes, sample.Outcome)
		if sample.StatusCode > 0 {
			aggregate.StatusClasses[statusClass(sample.StatusCode)]++
		}
		if sample.ResponseBytes > 0 {
			if math.MaxInt64-aggregate.ResponseBytes < sample.ResponseBytes {
				return Aggregate{}, errors.New("aggregate metrics: response byte count overflow")
			}
			aggregate.ResponseBytes += sample.ResponseBytes
		}
		if sample.Protocol != "" {
			aggregate.Protocols[sample.Protocol]++
		}
		if sample.Connection.Observed {
			aggregate.Connections.Observed++
			if sample.Connection.Reused {
				aggregate.Connections.Reused++
			} else {
				aggregate.Connections.New++
			}
		}
		if value, ok := sample.Timings.Total.Value(); ok {
			totals = append(totals, value)
		}
		delays = append(delays, sample.SchedulingDelay())
		appendPhase(phaseValues, "dns", sample.Timings.DNS)
		appendPhase(phaseValues, "connect", sample.Timings.Connect)
		appendPhase(phaseValues, "tls", sample.Timings.TLS)
		appendPhase(phaseValues, "request_write", sample.Timings.RequestWrite)
		appendPhase(phaseValues, "first_byte_wait", sample.Timings.FirstByteWait)
		appendPhase(phaseValues, "body_transfer", sample.Timings.BodyTransfer)
	}

	var err error
	if aggregate.Latency, err = CalculateLatency(totals); err != nil {
		return Aggregate{}, err
	}
	if aggregate.SchedulingDelay, err = CalculateLatency(delays); err != nil {
		return Aggregate{}, err
	}
	for name, values := range phaseValues {
		statistics, phaseErr := CalculateLatency(values)
		if phaseErr != nil {
			return Aggregate{}, phaseErr
		}
		aggregate.Phases[name] = statistics
	}

	aggregate.Rates = calculateRates(input.Execution, aggregate.Outcomes.Completed)
	aggregate.Concurrency = calculateConcurrency(input.Samples, input.Execution)
	if len(input.Samples) == 0 {
		aggregate.Limitations = append(aggregate.Limitations, "no measured samples")
	}
	if aggregate.Latency.Count != len(input.Samples) {
		aggregate.Limitations = append(aggregate.Limitations, "total latency unavailable for some samples")
	}
	if input.Execution.Interrupted {
		aggregate.Limitations = append(aggregate.Limitations, "experiment was interrupted")
	}
	return aggregate, nil
}

func countOutcome(counts *OutcomeCounts, outcome httpmeasure.Outcome) {
	if outcome.Complete {
		counts.Completed++
	}
	switch outcome.Category {
	case httpmeasure.OutcomeSuccess:
		counts.Successful++
	case httpmeasure.OutcomeHTTPError:
		counts.HTTPError++
	case httpmeasure.OutcomeTransportError:
		counts.TransportError++
	case httpmeasure.OutcomeTimeout:
		counts.Timeout++
	case httpmeasure.OutcomeCancelled:
		counts.Cancelled++
	case httpmeasure.OutcomeBodyLimit:
		counts.BodyLimit++
	}
}

func statusClass(code int) string {
	if code >= 100 && code <= 599 {
		return string(rune('0'+code/100)) + "xx"
	}
	return "other"
}

func appendPhase(values map[string][]time.Duration, name string, phase httpmeasure.PhaseDuration) {
	if value, ok := phase.Value(); ok {
		values[name] = append(values[name], value)
	}
}

func calculateRates(execution experiment.Execution, completed int) RateMetrics {
	duration := execution.FinishedAt.Sub(execution.StartedAt)
	rates := RateMetrics{
		Duration: duration, OfferedStarts: execution.OfferedStarts,
		AchievedStarts: execution.AchievedStarts,
	}
	if duration <= 0 {
		return rates
	}
	seconds := duration.Seconds()
	rates.OfferedPerSec = float64(rates.OfferedStarts) / seconds
	rates.AchievedPerSec = float64(rates.AchievedStarts) / seconds
	rates.Throughput = float64(completed) / seconds
	return rates
}

type concurrencyEvent struct {
	at    time.Time
	delta int
}

func calculateConcurrency(samples experiment.MeasuredSamples, execution experiment.Execution) ConcurrencyMetrics {
	events := make([]concurrencyEvent, 0, len(samples)*2)
	for _, sample := range samples {
		if sample.ActualStart.IsZero() || sample.FinishedAt.IsZero() || sample.FinishedAt.Before(sample.ActualStart) {
			continue
		}
		events = append(events, concurrencyEvent{at: sample.ActualStart, delta: 1}, concurrencyEvent{at: sample.FinishedAt, delta: -1})
	}
	sort.SliceStable(events, func(i, j int) bool {
		if events[i].at.Equal(events[j].at) {
			return events[i].delta < events[j].delta
		}
		return events[i].at.Before(events[j].at)
	})
	result := ConcurrencyMetrics{}
	if len(events) == 0 {
		return result
	}
	current := 0
	previous := execution.StartedAt
	var weighted time.Duration
	for _, event := range events {
		if !previous.IsZero() && event.at.After(previous) {
			weighted += time.Duration(current) * event.at.Sub(previous)
		}
		current += event.delta
		if current > result.Peak {
			result.Peak = current
		}
		previous = event.at
	}
	duration := execution.FinishedAt.Sub(execution.StartedAt)
	if duration > 0 {
		result.TimeWeightedInFlight = float64(weighted) / float64(duration)
	}
	return result
}
