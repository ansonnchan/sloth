package metrics

import (
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

func TestCalculateAggregateReconcilesEvidence(t *testing.T) {
	start := time.Unix(100, 0)
	samples := experiment.MeasuredSamples{
		aggregateSample(0, start, 0, 2*time.Second, httpmeasure.OutcomeSuccess, 200, "HTTP/2.0", false),
		aggregateSample(1, start, time.Second, 3*time.Second, httpmeasure.OutcomeHTTPError, 503, "HTTP/2.0", true),
		aggregateSample(2, start, 3*time.Second, 4*time.Second, httpmeasure.OutcomeTimeout, 0, "", false),
	}
	execution := experiment.Execution{
		StartedAt: start, FinishedAt: start.Add(5 * time.Second),
		OfferedStarts: 4, AchievedStarts: 3,
	}

	got, err := CalculateAggregate(AggregateInput{Samples: samples, Execution: execution})
	if err != nil {
		t.Fatalf("CalculateAggregate() error = %v", err)
	}
	wantCounts := OutcomeCounts{Attempted: 3, Completed: 2, Successful: 1, HTTPError: 1, Timeout: 1}
	if got.Outcomes != wantCounts {
		t.Fatalf("outcomes = %#v, want %#v", got.Outcomes, wantCounts)
	}
	if got.StatusClasses["2xx"] != 1 || got.StatusClasses["5xx"] != 1 || got.Protocols["HTTP/2.0"] != 2 {
		t.Fatalf("distributions = status %#v protocol %#v", got.StatusClasses, got.Protocols)
	}
	if got.Connections.Observed != 3 || got.Connections.Reused != 1 || got.Connections.New != 2 {
		t.Fatalf("connections = %#v", got.Connections)
	}
	if got.ResponseBytes != 600 || got.Rates.OfferedPerSec != 0.8 || got.Rates.AchievedPerSec != 0.6 || got.Rates.Throughput != 0.4 {
		t.Fatalf("bytes/rates = %d %#v", got.ResponseBytes, got.Rates)
	}
	if got.Concurrency.Peak != 2 || got.Concurrency.TimeWeightedInFlight != 1 {
		t.Fatalf("concurrency = %#v", got.Concurrency)
	}
	if got.Latency.Count != 3 || got.SchedulingDelay.Count != 3 {
		t.Fatalf("latency counts = %#v %#v", got.Latency, got.SchedulingDelay)
	}
}

func TestCalculateAggregateEmptyAndInterrupted(t *testing.T) {
	start := time.Unix(100, 0)
	got, err := CalculateAggregate(AggregateInput{Execution: experiment.Execution{
		StartedAt: start, FinishedAt: start.Add(time.Second), Interrupted: true,
	}})
	if err != nil {
		t.Fatalf("CalculateAggregate() error = %v", err)
	}
	if got.Latency.Available || !got.Interrupted || len(got.Limitations) != 2 {
		t.Fatalf("empty interrupted aggregate = %#v", got)
	}
}

func TestCalculateAggregateCountsEveryOutcome(t *testing.T) {
	start := time.Unix(100, 0)
	categories := []httpmeasure.OutcomeCategory{
		httpmeasure.OutcomeSuccess, httpmeasure.OutcomeHTTPError,
		httpmeasure.OutcomeTransportError, httpmeasure.OutcomeTimeout,
		httpmeasure.OutcomeCancelled, httpmeasure.OutcomeBodyLimit,
	}
	samples := make(experiment.MeasuredSamples, len(categories))
	for index, category := range categories {
		samples[index] = aggregateSample(index, start, time.Duration(index)*time.Second, time.Duration(index+1)*time.Second, category, 0, "", false)
	}
	got, err := CalculateAggregate(AggregateInput{Samples: samples, Execution: experiment.Execution{StartedAt: start, FinishedAt: start.Add(10 * time.Second)}})
	if err != nil {
		t.Fatalf("CalculateAggregate() error = %v", err)
	}
	if got.Outcomes.Attempted != 6 || got.Outcomes.Successful != 1 || got.Outcomes.HTTPError != 1 || got.Outcomes.TransportError != 1 || got.Outcomes.Timeout != 1 || got.Outcomes.Cancelled != 1 || got.Outcomes.BodyLimit != 1 {
		t.Fatalf("outcomes do not reconcile: %#v", got.Outcomes)
	}
}

func aggregateSample(sequence int, origin time.Time, offset, endOffset time.Duration, category httpmeasure.OutcomeCategory, status int, protocol string, reused bool) experiment.Sample {
	actual := origin.Add(offset)
	finished := origin.Add(endOffset)
	complete := category == httpmeasure.OutcomeSuccess || category == httpmeasure.OutcomeHTTPError
	return experiment.Sample{
		Sequence: sequence, PlannedStart: actual.Add(-10 * time.Millisecond), ActualStart: actual,
		FinishedAt: finished, StatusCode: status, Protocol: protocol,
		ResponseBytes: 200,
		Outcome:       httpmeasure.Outcome{Category: category, Complete: complete},
		Connection:    experiment.Connection{Observed: true, Reused: reused},
		Timings: httpmeasure.DeriveTimingPhases(httpmeasure.TraceRecord{
			RequestStart: actual, BodyDone: finished,
		}, false),
	}
}
