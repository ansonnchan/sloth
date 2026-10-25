package runner

import (
	"context"
	"fmt"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/inspect"
	"github.com/ansonnchan/sloth/internal/metrics"
)

type ConnectionArm string

const (
	ConnectionFresh       ConnectionArm = "fresh"
	ConnectionReused      ConnectionArm = "reused"
	DefaultConnectionSeed uint64        = 0x534c4f5448
)

type ConnectionOptions struct {
	Seed         uint64
	Clock        Clock
	FreshFactory inspect.ClientFactory
	ReuseFactory inspect.ClientFactory
}

// ConnectionReport records the deterministic arm order and only considers a
// comparison sufficient when observed connection state supports both modes.
type ConnectionReport struct {
	Seed        uint64           `json:"seed"`
	Order       []ConnectionArm  `json:"order"`
	Fresh       ExperimentReport `json:"fresh"`
	Reused      ExperimentReport `json:"reused"`
	Sufficient  bool             `json:"sufficient"`
	Limitations []string         `json:"limitations,omitempty"`
}

// RunConnectionExperiment alternates measured fresh and reused requests under
// one shared limiter. Each arm owns a distinct transport and pool.
func RunConnectionExperiment(
	ctx context.Context,
	spec httpmeasure.RequestSpec,
	effective config.Effective,
	limiter *StartLimiter,
	budget *RequestBudget,
	options ConnectionOptions,
) (ConnectionReport, error) {
	seed := options.Seed
	if seed == 0 {
		seed = DefaultConnectionSeed
	}
	report := ConnectionReport{Seed: seed, Order: connectionOrder(effective.Requests(), seed)}
	clock := options.Clock
	if clock == nil {
		clock = realClock{}
	}

	freshFactory, closeFresh, err := connectionFactory(effective, httpmeasure.TransportFresh, options.FreshFactory)
	if err != nil {
		return report, err
	}
	defer closeFresh()
	reuseFactory, closeReuse, err := connectionFactory(effective, httpmeasure.TransportReuse, options.ReuseFactory)
	if err != nil {
		return report, err
	}
	defer closeReuse()

	freshExecutor := &httpExecutor{service: inspect.NewService(freshFactory), spec: spec, clock: clock}
	reuseExecutor := &httpExecutor{service: inspect.NewService(reuseFactory), spec: spec, clock: clock}
	freshPlan := connectionPlan("fresh-connection", experiment.KindFreshConnection, effective)
	reusePlan := connectionPlan("reused-connection", experiment.KindReusedConnection, effective)
	report.Fresh.Result.Plan = freshPlan
	report.Reused.Result.Plan = reusePlan

	if err := runConnectionWarmups(ctx, &report, freshPlan, freshExecutor, reusePlan, reuseExecutor, limiter, budget, clock); err != nil {
		return finalizeConnectionReport(report), err
	}

	measuredStarted := clock.Now()
	dispatcher := &connectionDispatcher{order: report.Order, fresh: freshExecutor, reused: reuseExecutor}
	pool, err := NewPool(1, dispatcher, clock)
	if err != nil {
		return report, err
	}
	schedule, err := NewSchedule(measuredStarted, len(report.Order), effective.Rate())
	if err != nil {
		return report, err
	}
	poolReport, runErr := pool.Run(ctx, schedule.Arrivals(), limiter, budget)
	finished := clock.Now()
	freshSamples, reusedSamples := splitConnectionSamples(poolReport.Samples, report.Order)
	report.Fresh.Result.Measured = freshSamples
	report.Reused.Result.Measured = reusedSamples
	for _, target := range []*experiment.Result{&report.Fresh.Result, &report.Reused.Result} {
		target.Execution = experiment.Execution{
			StartedAt: measuredStarted, FinishedAt: finished,
			OfferedStarts: len(target.Measured), AchievedStarts: len(target.Measured),
			Interrupted: poolReport.StopReason != experiment.StopNone,
			StopReason:  poolReport.StopReason,
		}
	}
	report = finalizeConnectionReport(report)
	return report, runErr
}

func connectionFactory(effective config.Effective, profile httpmeasure.TransportProfile, injected inspect.ClientFactory) (inspect.ClientFactory, func(), error) {
	if injected != nil {
		return injected, func() {}, nil
	}
	owner, err := httpmeasure.NewTransport(profile)
	if err != nil {
		return nil, nil, err
	}
	client := owner.Client(effective.Timeout())
	factory := func(settings inspect.ClientSettings) (inspect.ClientSession, error) {
		client.Timeout = settings.Timeout
		client.CheckRedirect = settings.CheckRedirect
		return inspect.ClientSession{Client: client, Close: func() {}}, nil
	}
	return factory, owner.Close, nil
}

func connectionPlan(id string, kind experiment.Kind, effective config.Effective) experiment.Plan {
	return experiment.NewPlan(experiment.PlanInput{
		ID: id, Kind: kind, Config: effective.Snapshot(), Requests: effective.Requests(),
		Warmups: effective.Warmups(), Workers: 1, Rate: effective.Rate(),
	})
}

func connectionOrder(count int, seed uint64) []ConnectionArm {
	order := make([]ConnectionArm, 0, count*2)
	first, second := ConnectionFresh, ConnectionReused
	if mixSeed(seed)&1 == 1 {
		first, second = second, first
	}
	for range count {
		order = append(order, first, second)
	}
	return order
}

func mixSeed(value uint64) uint64 {
	value ^= value >> 12
	value ^= value << 25
	value ^= value >> 27
	return value * 2685821657736338717
}

func runConnectionWarmups(
	ctx context.Context,
	report *ConnectionReport,
	freshPlan experiment.Plan,
	fresh Executor,
	reusePlan experiment.Plan,
	reused Executor,
	limiter *StartLimiter,
	budget *RequestBudget,
	clock Clock,
) error {
	for _, arm := range []struct {
		result   *experiment.Result
		plan     experiment.Plan
		executor Executor
	}{
		{&report.Fresh.Result, freshPlan, fresh},
		{&report.Reused.Result, reusePlan, reused},
	} {
		pool, err := NewPool(1, arm.executor, clock)
		if err != nil {
			return err
		}
		phase, err := runPhase(ctx, pool, arm.plan.Warmups(), arm.plan.Rate(), limiter, budget, clock)
		arm.result.Warmups = experiment.WarmupSamples(phase.Samples)
		setExperimentID(arm.result.Warmups, arm.plan.ID())
		arm.result.WarmupSummary = experiment.SummarizeOutcomes(arm.result.Warmups)
		if err != nil || phase.StopReason != experiment.StopNone {
			arm.result.Execution.Interrupted = true
			arm.result.Execution.StopReason = phase.StopReason
			return err
		}
	}
	return nil
}

type connectionDispatcher struct {
	order         []ConnectionArm
	fresh, reused Executor
}

func (d *connectionDispatcher) Execute(ctx context.Context, arrival StartedArrival) (experiment.Sample, error) {
	if arrival.Sequence < 0 || arrival.Sequence >= len(d.order) {
		return experiment.Sample{}, fmt.Errorf("execute connection experiment: sequence %d is out of range", arrival.Sequence)
	}
	if d.order[arrival.Sequence] == ConnectionFresh {
		return d.fresh.Execute(ctx, arrival)
	}
	return d.reused.Execute(ctx, arrival)
}

func splitConnectionSamples(samples []experiment.Sample, order []ConnectionArm) (experiment.MeasuredSamples, experiment.MeasuredSamples) {
	fresh := make(experiment.MeasuredSamples, 0, len(order)/2)
	reused := make(experiment.MeasuredSamples, 0, len(order)/2)
	for _, sample := range samples {
		if sample.Sequence < 0 || sample.Sequence >= len(order) {
			continue
		}
		if order[sample.Sequence] == ConnectionFresh {
			sample.Sequence = len(fresh)
			sample.ExperimentID = "fresh-connection"
			fresh = append(fresh, sample)
		} else {
			sample.Sequence = len(reused)
			sample.ExperimentID = "reused-connection"
			reused = append(reused, sample)
		}
	}
	return fresh, reused
}

func finalizeConnectionReport(report ConnectionReport) ConnectionReport {
	report.Fresh.Aggregate, _ = metrics.CalculateAggregate(metrics.AggregateInput{Samples: report.Fresh.Result.Measured, Execution: report.Fresh.Result.Execution})
	report.Reused.Aggregate, _ = metrics.CalculateAggregate(metrics.AggregateInput{Samples: report.Reused.Result.Measured, Execution: report.Reused.Result.Execution})
	freshEvidence := len(report.Fresh.Result.Measured) > 0 && report.Fresh.Aggregate.Connections.New == len(report.Fresh.Result.Measured)
	reuseEvidence := len(report.Reused.Result.Measured) > 0 && report.Reused.Aggregate.Connections.Reused > 0
	report.Sufficient = freshEvidence && reuseEvidence
	if !freshEvidence {
		report.Limitations = append(report.Limitations, "fresh arm did not observe a new connection for every measured request")
	}
	if !reuseEvidence {
		report.Limitations = append(report.Limitations, "reused arm did not observe connection reuse")
	}
	if report.Fresh.Aggregate.Protocols[httpVersion2] > 0 || report.Reused.Aggregate.Protocols[httpVersion2] > 0 {
		report.Limitations = append(report.Limitations, "HTTP/2 connection evidence may include multiplexed streams")
	}
	return report
}

const httpVersion2 = "HTTP/2.0"
