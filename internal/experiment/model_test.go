package experiment

import (
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/config"
)

func TestPlanSnapshotsConfiguration(t *testing.T) {
	snapshot := config.Defaults()
	effective := config.Resolve(snapshot).Snapshot()
	plan := NewPlan(PlanInput{ID: "baseline", Kind: KindBaseline, Config: effective, Requests: 10, Warmups: 2, Workers: 1, Rate: 5})

	effective.ConcurrencyStages[0] = 99
	got := plan.Config()
	got.ConcurrencyStages[0] = 42
	if plan.Config().ConcurrencyStages[0] != 1 {
		t.Fatal("plan configuration changed through an external slice")
	}
	if plan.ID() != "baseline" || plan.Kind() != KindBaseline || plan.Requests() != 10 || plan.Warmups() != 2 || plan.Workers() != 1 || plan.Rate() != 5 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestSampleSchedulingDelay(t *testing.T) {
	planned := time.Unix(100, 0)
	sample := Sample{PlannedStart: planned, ActualStart: planned.Add(25 * time.Millisecond)}
	if got := sample.SchedulingDelay(); got != 25*time.Millisecond {
		t.Fatalf("SchedulingDelay() = %v, want 25ms", got)
	}

	sample.ActualStart = planned.Add(-time.Millisecond)
	if got := sample.SchedulingDelay(); got != 0 {
		t.Fatalf("negative SchedulingDelay() = %v, want zero", got)
	}
}

func TestPartialResultRecordsStopReason(t *testing.T) {
	result := Result{
		Warmups:  WarmupSamples{{Sequence: 0}},
		Measured: MeasuredSamples{{Sequence: 0}},
		Execution: Execution{
			Interrupted: true,
			StopReason:  StopCancelled,
		},
	}
	if !result.Execution.Interrupted || result.Execution.StopReason != StopCancelled {
		t.Fatalf("partial execution = %#v", result.Execution)
	}
	if len(result.Warmups) != 1 || len(result.Measured) != 1 {
		t.Fatalf("sample roles were not preserved: %#v", result)
	}
}
