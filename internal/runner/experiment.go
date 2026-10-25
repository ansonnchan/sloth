package runner

import (
	"context"
	"time"

	"github.com/ansonnchan/sloth/internal/experiment"
)

// RunExperiment executes warm-ups before measured traffic while sharing the
// same limiter, request budget, executor, cancellation path, and worker bound.
// HTTP-level warm-up failures remain evidence and do not stop measured work.
func RunExperiment(
	ctx context.Context,
	plan experiment.Plan,
	executor Executor,
	limiter *StartLimiter,
	budget *RequestBudget,
	clock Clock,
) (experiment.Result, error) {
	if clock == nil {
		clock = realClock{}
	}
	result := experiment.Result{Plan: plan}
	result.Execution.StartedAt = clock.Now()
	pool, err := NewPool(plan.Workers(), executor, clock)
	if err != nil {
		return result, err
	}

	warmupReport, err := runPhase(ctx, pool, plan.Warmups(), plan.Rate(), limiter, budget, clock)
	result.Warmups = experiment.WarmupSamples(warmupReport.Samples)
	setExperimentID(result.Warmups, plan.ID())
	result.WarmupSummary = experiment.SummarizeOutcomes(result.Warmups)
	applyPoolReport(&result.Execution, warmupReport, startInterval(plan.Rate()))
	if err != nil || warmupReport.StopReason != experiment.StopNone {
		result.Execution.FinishedAt = clock.Now()
		return result, err
	}

	measuredReport, err := runPhase(ctx, pool, plan.Requests(), plan.Rate(), limiter, budget, clock)
	result.Measured = experiment.MeasuredSamples(measuredReport.Samples)
	setExperimentID(result.Measured, plan.ID())
	applyPoolReport(&result.Execution, measuredReport, startInterval(plan.Rate()))
	result.Execution.FinishedAt = clock.Now()
	return result, err
}

func runPhase(
	ctx context.Context,
	pool *Pool,
	count int,
	rate float64,
	limiter *StartLimiter,
	budget *RequestBudget,
	clock Clock,
) (PoolReport, error) {
	schedule, err := NewSchedule(clock.Now(), count, rate)
	if err != nil {
		return PoolReport{}, err
	}
	return pool.Run(ctx, schedule.Arrivals(), limiter, budget)
}

func applyPoolReport(execution *experiment.Execution, report PoolReport, missedThreshold time.Duration) {
	execution.OfferedStarts += report.Started
	execution.AchievedStarts += report.Started
	for _, sample := range report.Samples {
		interval := sample.ActualStart.Sub(sample.PlannedStart)
		if interval >= missedThreshold {
			execution.MissedArrivals++
		}
	}
	if report.StopReason != experiment.StopNone {
		execution.StopReason = report.StopReason
		execution.Interrupted = true
	}
}

func startInterval(rate float64) time.Duration {
	return time.Duration(float64(time.Second) / rate)
}

type sampleCollection interface {
	~[]experiment.Sample
}

func setExperimentID[T sampleCollection](samples T, id string) {
	for index := range samples {
		samples[index].ExperimentID = id
	}
}
