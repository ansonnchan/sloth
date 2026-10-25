package runner

import (
	"context"
	"errors"
	"time"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/inspect"
	"github.com/ansonnchan/sloth/internal/metrics"
)

// ExperimentReport combines raw bounded evidence with post-run aggregates.
type ExperimentReport struct {
	Result    experiment.Result `json:"result"`
	Aggregate metrics.Aggregate `json:"aggregate"`
}

// BaselineOptions exposes only deterministic test seams. Production callers
// leave these fields zero-valued.
type BaselineOptions struct {
	Clock   Clock
	Factory inspect.ClientFactory
}

// RunBaseline executes the sequential reference arm with a reusable transport.
func RunBaseline(
	ctx context.Context,
	spec httpmeasure.RequestSpec,
	effective config.Effective,
	limiter *StartLimiter,
	budget *RequestBudget,
	options BaselineOptions,
) (ExperimentReport, error) {
	factory := options.Factory
	var closeTransport func()
	if factory == nil {
		owner, err := httpmeasure.NewTransport(httpmeasure.TransportReuse)
		if err != nil {
			return ExperimentReport{}, err
		}
		closeTransport = owner.Close
		client := owner.Client(effective.Timeout())
		factory = func(settings inspect.ClientSettings) (inspect.ClientSession, error) {
			client.Timeout = settings.Timeout
			client.CheckRedirect = settings.CheckRedirect
			return inspect.ClientSession{Client: client, Close: func() {}}, nil
		}
	} else {
		closeTransport = func() {}
	}
	defer closeTransport()

	plan := experiment.NewPlan(experiment.PlanInput{
		ID: "baseline", Kind: experiment.KindBaseline, Config: effective.Snapshot(),
		Requests: effective.Requests(), Warmups: effective.Warmups(),
		Workers: 1, Rate: effective.Rate(),
	})
	executor := &httpExecutor{service: inspect.NewService(factory), spec: spec, clock: options.Clock}
	result, runErr := RunExperiment(ctx, plan, executor, limiter, budget, options.Clock)
	aggregate, aggregateErr := metrics.CalculateAggregate(metrics.AggregateInput{Samples: result.Measured, Execution: result.Execution})
	report := ExperimentReport{Result: result, Aggregate: aggregate}
	return report, errors.Join(runErr, aggregateErr)
}

type httpExecutor struct {
	service *inspect.Service
	spec    httpmeasure.RequestSpec
	clock   Clock
}

func (e *httpExecutor) Execute(ctx context.Context, arrival StartedArrival) (experiment.Sample, error) {
	result, err := e.service.Inspect(ctx, e.spec)
	finishedAt := time.Now()
	if e.clock != nil {
		finishedAt = e.clock.Now()
	}
	sample := experiment.Sample{
		Sequence: arrival.Sequence, PlannedStart: arrival.PlannedStart,
		ActualStart: arrival.ActualStart, FinishedAt: finishedAt,
		Timings: result.Timings, Outcome: result.Outcome,
		Connection: experiment.Connection{
			Observed: result.Connection.Observed,
			Reused:   result.Connection.Reused,
			WasIdle:  result.Connection.WasIdle,
		},
		Protocol: result.Protocol, StatusCode: result.Outcome.StatusCode,
		ResponseBytes: result.Body.TransferredBytes,
	}
	var requestErr *httpmeasure.RequestError
	if err == nil || errors.As(err, &requestErr) {
		return sample, nil
	}
	return sample, err
}
