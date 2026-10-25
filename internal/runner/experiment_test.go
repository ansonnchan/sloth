package runner

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

func TestRunExperimentSeparatesWarmupsAndMeasuredSamples(t *testing.T) {
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, _ := NewStartLimiter(1000, clock)
	budget, _ := NewRequestBudget(5)
	plan := testPlan(3, 2)
	var calls atomic.Int32
	executor := ExecutorFunc(func(_ context.Context, arrival StartedArrival) (experiment.Sample, error) {
		call := calls.Add(1)
		category := httpmeasure.OutcomeSuccess
		if call == 1 {
			category = httpmeasure.OutcomeTimeout
		}
		return experiment.Sample{Sequence: arrival.Sequence, Outcome: httpmeasure.Outcome{Category: category}}, nil
	})

	result, err := RunExperiment(context.Background(), plan, executor, limiter, budget, clock)
	if err != nil {
		t.Fatalf("RunExperiment() error = %v", err)
	}
	if len(result.Warmups) != 2 || len(result.Measured) != 3 || budget.Used() != 5 {
		t.Fatalf("result counts = %d warm-up, %d measured, %d budget", len(result.Warmups), len(result.Measured), budget.Used())
	}
	if result.WarmupSummary[httpmeasure.OutcomeTimeout] != 1 {
		t.Fatalf("warm-up failures not surfaced: %#v", result.WarmupSummary)
	}
	for _, sample := range result.Measured {
		if sample.Outcome.Category != httpmeasure.OutcomeSuccess || sample.ExperimentID != plan.ID() {
			t.Fatalf("measured sample = %#v", sample)
		}
	}
	if stats := limiter.Stats(); stats.Achieved != 5 {
		t.Fatalf("limiter stats = %#v, want all warm-ups and measurements", stats)
	}
}

func TestRunExperimentStopsAfterFatalWarmupFailure(t *testing.T) {
	clock := newRecordingClock(time.Unix(100, 0))
	limiter, _ := NewStartLimiter(1000, clock)
	budget, _ := NewRequestBudget(5)
	wantErr := errors.New("executor unavailable")
	executor := ExecutorFunc(func(_ context.Context, arrival StartedArrival) (experiment.Sample, error) {
		return experiment.Sample{Sequence: arrival.Sequence}, wantErr
	})

	result, err := RunExperiment(context.Background(), testPlan(3, 2), executor, limiter, budget, clock)
	if !errors.Is(err, wantErr) || len(result.Warmups) != 1 || len(result.Measured) != 0 {
		t.Fatalf("RunExperiment() = (%#v, %v)", result, err)
	}
	if result.Execution.StopReason != experiment.StopExecutionFailure || !result.Execution.Interrupted {
		t.Fatalf("execution = %#v", result.Execution)
	}
}

func testPlan(requests, warmups int) experiment.Plan {
	effective := config.Resolve(config.Defaults())
	return experiment.NewPlan(experiment.PlanInput{
		ID: "baseline", Kind: experiment.KindBaseline, Config: effective.Snapshot(),
		Requests: requests, Warmups: warmups, Workers: 1, Rate: 1000,
	})
}
