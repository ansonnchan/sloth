package runner

import (
	"context"
	"errors"
	"fmt"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/inspect"
	"github.com/ansonnchan/sloth/internal/metrics"
)

// ConcurrencyOptions provides deterministic test seams without changing the
// production transport and lifecycle behavior.
type ConcurrencyOptions struct {
	Clock   Clock
	Factory func(stage int) (inspect.ClientFactory, func(), error)
}

// RunConcurrencyExperiments executes ordered stages under the same aggregate
// limiter and request budget. Completed and partial stage results are retained.
func RunConcurrencyExperiments(
	ctx context.Context,
	spec httpmeasure.RequestSpec,
	effective config.Effective,
	limiter *StartLimiter,
	budget *RequestBudget,
	options ConcurrencyOptions,
) ([]ExperimentReport, error) {
	stages := effective.ConcurrencyStages()
	reports := make([]ExperimentReport, 0, len(stages))
	clock := options.Clock
	if clock == nil {
		clock = realClock{}
	}
	for _, stage := range stages {
		factory, closeStage, err := concurrencyFactory(stage, options.Factory)
		if err != nil {
			return reports, err
		}
		planConfig := effective.Snapshot()
		planConfig.ConcurrencyStages = []int{stage}
		plan := experiment.NewPlan(experiment.PlanInput{
			ID: fmt.Sprintf("concurrency-%d", stage), Kind: experiment.KindConcurrency,
			Config: planConfig, Requests: effective.Requests(), Warmups: effective.Warmups(),
			Workers: stage, Rate: effective.Rate(),
		})
		executor := &httpExecutor{service: inspect.NewService(factory), spec: spec, clock: clock}
		result, runErr := RunExperiment(ctx, plan, executor, limiter, budget, clock)
		closeStage()
		aggregate, aggregateErr := metrics.CalculateAggregate(metrics.AggregateInput{Samples: result.Measured, Execution: result.Execution})
		reports = append(reports, ExperimentReport{Result: result, Aggregate: aggregate})
		if err := errors.Join(runErr, aggregateErr); err != nil {
			return reports, err
		}
		if result.Execution.Interrupted {
			return reports, nil
		}
	}
	return reports, nil
}

func concurrencyFactory(stage int, injected func(int) (inspect.ClientFactory, func(), error)) (inspect.ClientFactory, func(), error) {
	if injected != nil {
		return injected(stage)
	}
	owner, err := httpmeasure.NewTransport(httpmeasure.TransportReuse)
	if err != nil {
		return nil, nil, err
	}
	factory := func(settings inspect.ClientSettings) (inspect.ClientSession, error) {
		// Clients are request-local because CheckRedirect carries request-local
		// hop state. Their shared transport remains safe for concurrent use.
		client := owner.Client(settings.Timeout)
		client.CheckRedirect = settings.CheckRedirect
		return inspect.ClientSession{Client: client, Close: func() {}}, nil
	}
	return factory, owner.Close, nil
}
