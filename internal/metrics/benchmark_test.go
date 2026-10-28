package metrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

func BenchmarkCalculateAggregate(b *testing.B) {
	for _, count := range []int{100, 1_000} {
		b.Run(fmt.Sprintf("samples-%d", count), func(b *testing.B) {
			input := aggregateBenchmarkInput(count)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := CalculateAggregate(input); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func aggregateBenchmarkInput(count int) AggregateInput {
	started := time.Unix(100, 0)
	record := httpmeasure.TraceRecord{RequestStart: started, BodyDone: started.Add(10 * time.Millisecond)}
	timings := httpmeasure.DeriveTimingPhases(record, false)
	samples := make(experiment.MeasuredSamples, count)
	for index := range samples {
		actual := started.Add(time.Duration(index) * time.Millisecond)
		samples[index] = experiment.Sample{
			Sequence: index, PlannedStart: actual, ActualStart: actual,
			FinishedAt: actual.Add(10 * time.Millisecond), Timings: timings,
			Outcome:    httpmeasure.ClassifyOutcome(200, 512, nil),
			Connection: experiment.Connection{Observed: true, Reused: index > 0},
			Protocol:   "HTTP/1.1", StatusCode: 200, ResponseBytes: 512,
		}
	}
	return AggregateInput{Samples: samples, Execution: experiment.Execution{
		StartedAt: started, FinishedAt: started.Add(time.Duration(count) * time.Millisecond),
		OfferedStarts: count, AchievedStarts: count,
	}}
}
