package runner

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/ansonnchan/sloth/internal/experiment"
)

// Executor performs one request after scheduling and rate admission.
// Measurement failures belong in Sample.Outcome; returned errors are reserved
// for runner failures that should stop the experiment.
type Executor interface {
	Execute(context.Context, StartedArrival) (experiment.Sample, error)
}

// ExecutorFunc adapts a function to Executor.
type ExecutorFunc func(context.Context, StartedArrival) (experiment.Sample, error)

func (f ExecutorFunc) Execute(ctx context.Context, arrival StartedArrival) (experiment.Sample, error) {
	return f(ctx, arrival)
}

// RequestBudget enforces one shared request-start ceiling across a suite.
type RequestBudget struct {
	mu    sync.Mutex
	limit int
	used  int
}

func NewRequestBudget(limit int) (*RequestBudget, error) {
	if limit < 0 {
		return nil, errors.New("create request budget: limit must not be negative")
	}
	return &RequestBudget{limit: limit}, nil
}

func (b *RequestBudget) TryTake() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.used >= b.limit {
		return false
	}
	b.used++
	return true
}

func (b *RequestBudget) Used() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.used
}

// Pool owns a fixed number of workers for one experiment stage.
type Pool struct {
	workers  int
	executor Executor
	clock    Clock
}

func NewPool(workers int, executor Executor, clock Clock) (*Pool, error) {
	if workers <= 0 {
		return nil, errors.New("create worker pool: workers must be greater than zero")
	}
	if executor == nil {
		return nil, errors.New("create worker pool: executor is required")
	}
	if clock == nil {
		clock = realClock{}
	}
	return &Pool{workers: workers, executor: executor, clock: clock}, nil
}

type poolResult struct {
	sample experiment.Sample
	err    error
}

// PoolReport contains ordered partial evidence and the reason work stopped.
type PoolReport struct {
	Samples    []experiment.Sample
	Started    int
	StopReason experiment.StopReason
}

// Run executes only the supplied bounded arrival list. The producer closes the
// job channel, workers own no channels, and one waiter closes results after all
// workers exit.
func (p *Pool) Run(ctx context.Context, arrivals []Arrival, limiter *StartLimiter, budget *RequestBudget) (PoolReport, error) {
	if limiter == nil {
		return PoolReport{}, errors.New("run worker pool: start limiter is required")
	}
	if budget == nil {
		return PoolReport{}, errors.New("run worker pool: request budget is required")
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan Arrival)
	results := make(chan poolResult, len(arrivals))
	var workers sync.WaitGroup
	var stateMu sync.Mutex
	stopReason := experiment.StopNone

	setStop := func(reason experiment.StopReason) {
		stateMu.Lock()
		if stopReason == experiment.StopNone {
			stopReason = reason
		}
		stateMu.Unlock()
	}

	for range p.workers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for arrival := range jobs {
				if runCtx.Err() != nil {
					return
				}
				if !budget.TryTake() {
					setStop(experiment.StopBudgetExhausted)
					cancel()
					return
				}
				if err := limiter.Wait(runCtx); err != nil {
					if ctx.Err() != nil {
						setStop(experiment.StopCancelled)
					}
					return
				}
				if runCtx.Err() != nil {
					return
				}
				started := Start(arrival, p.clock.Now())
				sample, err := p.executor.Execute(runCtx, started)
				if sample.Sequence == 0 && arrival.Sequence != 0 {
					sample.Sequence = arrival.Sequence
				}
				if sample.PlannedStart.IsZero() {
					sample.PlannedStart = arrival.PlannedStart
				}
				if sample.ActualStart.IsZero() {
					sample.ActualStart = started.ActualStart
				}
				results <- poolResult{sample: sample, err: err}
				if err != nil {
					setStop(experiment.StopExecutionFailure)
					cancel()
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, arrival := range arrivals {
			select {
			case jobs <- arrival:
			case <-runCtx.Done():
				return
			}
		}
	}()
	go func() {
		workers.Wait()
		close(results)
	}()

	report := PoolReport{}
	var runErr error
	for result := range results {
		report.Started++
		report.Samples = append(report.Samples, result.sample)
		if result.err != nil && runErr == nil {
			runErr = result.err
		}
	}
	if ctx.Err() != nil {
		setStop(experiment.StopCancelled)
		runErr = ctx.Err()
	}
	stateMu.Lock()
	report.StopReason = stopReason
	stateMu.Unlock()
	sort.SliceStable(report.Samples, func(i, j int) bool { return report.Samples[i].Sequence < report.Samples[j].Sequence })
	return report, runErr
}
