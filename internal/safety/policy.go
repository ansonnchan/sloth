// Package safety enforces Sloth's bounded traffic policy.
package safety

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/ansonnchan/sloth/internal/config"
)

// Ceilings define one tier of traffic and resource limits.
type Ceilings struct {
	RequestsPerExperiment int
	TotalRequests         int
	Rate                  float64
	Concurrency           int
	Timeout               time.Duration
	BodyLimit             int64
	SampleLimit           int
	RedirectLimit         int
}

var (
	normalCeilings = Ceilings{
		RequestsPerExperiment: 1_000,
		TotalRequests:         10_000,
		Rate:                  100,
		Concurrency:           50,
		Timeout:               time.Minute,
		BodyLimit:             10 << 20,
		SampleLimit:           1_000,
		RedirectLimit:         5,
	}
	absoluteCeilings = Ceilings{
		RequestsPerExperiment: 10_000,
		TotalRequests:         50_000,
		Rate:                  1_000,
		Concurrency:           config.AbsoluteConcurrency,
		Timeout:               5 * time.Minute,
		BodyLimit:             100 << 20,
		SampleLimit:           10_000,
		RedirectLimit:         10,
	}
)

// Experiment describes the traffic contributed by one experiment arm.
type Experiment struct {
	Name     string
	Measured int
	Warmups  int
}

// Budget is the complete request cost of a planned run.
type Budget struct {
	ExperimentArms int
	Measured       int
	Warmups        int
	Total          int
	MinimumTime    time.Duration
}

func (b Budget) String() string {
	return fmt.Sprintf("%d total planned requests (%d measured + %d warm-up) across %d experiment arms", b.Total, b.Measured, b.Warmups, b.ExperimentArms)
}

// StandardExperiments returns the planned diagnostic arms for a configuration.
func StandardExperiments(effective config.Effective) []Experiment {
	experiments := []Experiment{
		{Name: "baseline", Measured: effective.Requests(), Warmups: effective.Warmups()},
		{Name: "fresh-connection", Measured: effective.Requests(), Warmups: effective.Warmups()},
		{Name: "reused-connection", Measured: effective.Requests(), Warmups: effective.Warmups()},
	}
	for _, concurrency := range effective.ConcurrencyStages() {
		experiments = append(experiments, Experiment{
			Name:     fmt.Sprintf("concurrency-%d", concurrency),
			Measured: effective.Requests(),
			Warmups:  effective.Warmups(),
		})
	}
	return experiments
}

// CalculateBudget accounts for measured and warm-up traffic in every arm.
func CalculateBudget(experiments []Experiment, rate float64) Budget {
	budget := Budget{ExperimentArms: len(experiments)}
	for _, experiment := range experiments {
		budget.Measured += experiment.Measured
		budget.Warmups += experiment.Warmups
	}
	budget.Total = budget.Measured + budget.Warmups
	if budget.Total > 0 && rate > 0 && !math.IsInf(rate, 0) && !math.IsNaN(rate) {
		budget.MinimumTime = time.Duration(float64(time.Second) * float64(budget.Total) / rate)
	}
	return budget
}

// ErrorKind identifies which safety acknowledgement or ceiling failed.
type ErrorKind string

const (
	UnsafeMethod     ErrorKind = "unsafe_method"
	HighLoadRequired ErrorKind = "high_load_acknowledgement_required"
	AbsoluteLimit    ErrorKind = "absolute_limit"
)

// PolicyError is a deterministic safety-policy failure.
type PolicyError struct {
	Kind    ErrorKind
	Setting string
	Message string
}

func (e *PolicyError) Error() string {
	return e.Message
}

// Evaluate enforces unsafe-method acknowledgement and both ceiling tiers.
func Evaluate(effective config.Effective, experiments []Experiment) (Budget, error) {
	budget := CalculateBudget(experiments, effective.Rate())
	if setting, exceeded := firstExceeded(effective, budget, absoluteCeilings); exceeded {
		return budget, &PolicyError{
			Kind:    AbsoluteLimit,
			Setting: setting,
			Message: fmt.Sprintf("%s exceeds Sloth's absolute safety ceiling and cannot be acknowledged", setting),
		}
	}

	if !isSafeMethod(effective.Method()) && !effective.AllowUnsafeMethods() {
		return budget, &PolicyError{
			Kind:    UnsafeMethod,
			Setting: "method",
			Message: fmt.Sprintf("%s may change server state; rerun with --allow-unsafe-methods after reviewing the %s", effective.Method(), budget),
		}
	}

	if setting, exceeded := firstExceeded(effective, budget, normalCeilings); exceeded && !effective.AllowHighLoad() {
		return budget, &PolicyError{
			Kind:    HighLoadRequired,
			Setting: setting,
			Message: fmt.Sprintf("%s exceeds Sloth's normal safety ceiling; rerun with --allow-high-load after reviewing the %s", setting, budget),
		}
	}
	return budget, nil
}

func firstExceeded(effective config.Effective, budget Budget, ceilings Ceilings) (string, bool) {
	maximumConcurrency := 0
	if stages := effective.ConcurrencyStages(); len(stages) > 0 {
		maximumConcurrency = slices.Max(stages)
	}

	checks := []struct {
		setting  string
		exceeded bool
	}{
		{"requests per experiment", effective.Requests() > ceilings.RequestsPerExperiment},
		{"total planned requests", budget.Total > ceilings.TotalRequests},
		{"request rate", effective.Rate() > ceilings.Rate},
		{"concurrency", maximumConcurrency > ceilings.Concurrency},
		{"timeout", effective.Timeout() > ceilings.Timeout},
		{"response-body limit", effective.BodyLimit() > ceilings.BodyLimit},
		{"sample limit", effective.IncludeSamples() && effective.SampleLimit() > ceilings.SampleLimit},
		{"redirect limit", effective.FollowRedirects() && effective.RedirectLimit() > ceilings.RedirectLimit},
	}
	for _, check := range checks {
		if check.exceeded {
			return check.setting, true
		}
	}
	return "", false
}

func isSafeMethod(method string) bool {
	return slices.Contains([]string{"GET", "HEAD", "OPTIONS"}, strings.ToUpper(method))
}
