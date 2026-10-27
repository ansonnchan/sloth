// Package diagnose orchestrates the bounded experiment suite and pure rules.
package diagnose

import (
	"context"
	"errors"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/diagnosis"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/runner"
	"github.com/ansonnchan/sloth/internal/safety"
)

type Stage string

const (
	StageBaseline    Stage = "baseline"
	StageConnection  Stage = "connection comparison"
	StageConcurrency Stage = "concurrency stages"
)

// ProgressFunc observes coarse experiment boundaries. It is presentation-only
// and never receives request data or secrets.
type ProgressFunc func(Stage)

// Result retains partial suite evidence alongside deterministic findings.
type Result struct {
	Suite     diagnosis.Suite  `json:"suite"`
	Diagnosis diagnosis.Report `json:"diagnosis"`
}

type Service struct {
	engine   diagnosis.Engine
	progress ProgressFunc
}

func NewService(progress ProgressFunc) *Service {
	return &Service{engine: diagnosis.DefaultEngine(), progress: progress}
}

// Run executes all arms under one limiter and one hard request budget. If a
// stage stops early, it diagnoses the retained partial evidence before return.
func (service *Service) Run(ctx context.Context, spec httpmeasure.RequestSpec, effective config.Effective) (Result, error) {
	var result Result
	planned := safety.CalculateBudget(safety.StandardExperiments(effective), effective.Rate())
	limiter, err := runner.NewStartLimiter(effective.Rate(), nil)
	if err != nil {
		return result, err
	}
	budget, err := runner.NewRequestBudget(planned.Total)
	if err != nil {
		return result, err
	}

	service.notify(StageBaseline)
	result.Suite.Baseline, err = runner.RunBaseline(ctx, spec, effective, limiter, budget, runner.BaselineOptions{})
	if err != nil || result.Suite.Baseline.Result.Execution.Interrupted {
		return service.finish(result, err)
	}

	service.notify(StageConnection)
	result.Suite.Connection, err = runner.RunConnectionExperiment(ctx, spec, effective, limiter, budget, runner.ConnectionOptions{})
	if err != nil || result.Suite.Connection.Fresh.Result.Execution.Interrupted || result.Suite.Connection.Reused.Result.Execution.Interrupted {
		return service.finish(result, err)
	}

	service.notify(StageConcurrency)
	result.Suite.Concurrency, err = runner.RunConcurrencyExperiments(ctx, spec, effective, limiter, budget, runner.ConcurrencyOptions{})
	return service.finish(result, err)
}

func (service *Service) finish(result Result, runErr error) (Result, error) {
	report, diagnosisErr := service.engine.Evaluate(result.Suite)
	result.Diagnosis = report
	return result, errors.Join(runErr, diagnosisErr)
}

func (service *Service) notify(stage Stage) {
	if service.progress != nil {
		service.progress(stage)
	}
}
