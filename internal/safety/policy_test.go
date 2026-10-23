package safety

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/config"
)

func effectiveWith(mutate func(*config.Options)) config.Effective {
	options := config.Defaults()
	if mutate != nil {
		mutate(&options)
	}
	return config.Resolve(options)
}

func TestStandardBudgetIncludesEveryWarmup(t *testing.T) {
	effective := effectiveWith(nil)
	experiments := StandardExperiments(effective)
	budget := CalculateBudget(experiments, effective.Rate())

	if budget.ExperimentArms != 6 || budget.Measured != 600 || budget.Warmups != 30 || budget.Total != 630 {
		t.Fatalf("budget = %+v", budget)
	}
	if budget.MinimumTime != 126*time.Second {
		t.Fatalf("minimum time = %s", budget.MinimumTime)
	}
	if got := budget.String(); got != "630 total planned requests (600 measured + 30 warm-up) across 6 experiment arms" {
		t.Fatalf("budget string = %q", got)
	}
}

func TestDefaultPolicyPasses(t *testing.T) {
	effective := effectiveWith(nil)
	if _, err := Evaluate(effective, StandardExperiments(effective)); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
}

func TestUnsafeMethodsRequireAcknowledgement(t *testing.T) {
	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {
		t.Run(method, func(t *testing.T) {
			effective := effectiveWith(func(options *config.Options) { options.Method = method })
			budget, err := Evaluate(effective, StandardExperiments(effective))
			assertPolicyError(t, err, UnsafeMethod)
			if budget.Total != 630 || !strings.Contains(err.Error(), "630 total planned requests") {
				t.Fatalf("error does not expose complete budget: %v", err)
			}
		})
	}
}

func TestUnsafeMethodAcknowledgementPasses(t *testing.T) {
	effective := effectiveWith(func(options *config.Options) {
		options.Method = "POST"
		options.AllowUnsafeMethods = true
	})
	if _, err := Evaluate(effective, StandardExperiments(effective)); err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
}

func TestNormalCeilingsRequireHighLoadAcknowledgement(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*config.Options)
		plan   func(config.Effective) []Experiment
	}{
		{"requests", func(options *config.Options) { options.Requests = normalCeilings.RequestsPerExperiment + 1 }, StandardExperiments},
		{"total requests", nil, func(effective config.Effective) []Experiment {
			return []Experiment{{Name: "large-suite", Measured: normalCeilings.TotalRequests + 1}}
		}},
		{"rate", func(options *config.Options) { options.Rate = normalCeilings.Rate + 1 }, StandardExperiments},
		{"concurrency", func(options *config.Options) {
			options.Requests = normalCeilings.Concurrency + 1
			options.ConcurrencyStages = []int{1, normalCeilings.Concurrency + 1}
		}, StandardExperiments},
		{"timeout", func(options *config.Options) { options.Timeout = normalCeilings.Timeout + time.Second }, StandardExperiments},
		{"body limit", func(options *config.Options) { options.BodyLimit = normalCeilings.BodyLimit + 1 }, StandardExperiments},
		{"sample limit", func(options *config.Options) {
			options.IncludeSamples = true
			options.SampleLimit = normalCeilings.SampleLimit + 1
		}, StandardExperiments},
		{"redirect limit", func(options *config.Options) {
			options.FollowRedirects = true
			options.RedirectLimit = normalCeilings.RedirectLimit + 1
		}, StandardExperiments},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			effective := effectiveWith(test.mutate)
			_, err := Evaluate(effective, test.plan(effective))
			assertPolicyError(t, err, HighLoadRequired)

			options := config.Defaults()
			if test.mutate != nil {
				test.mutate(&options)
			}
			options.AllowHighLoad = true
			acknowledged := config.Resolve(options)
			if _, err := Evaluate(acknowledged, test.plan(acknowledged)); err != nil {
				t.Fatalf("acknowledged Evaluate() error = %v", err)
			}
		})
	}
}

func TestAbsoluteCeilingsCannotBeBypassed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*config.Options)
		plan   func(config.Effective) []Experiment
	}{
		{"requests", func(options *config.Options) { options.Requests = absoluteCeilings.RequestsPerExperiment + 1 }, StandardExperiments},
		{"total requests", nil, func(effective config.Effective) []Experiment {
			return []Experiment{{Name: "unbounded", Measured: absoluteCeilings.TotalRequests + 1}}
		}},
		{"rate", func(options *config.Options) { options.Rate = absoluteCeilings.Rate + 1 }, StandardExperiments},
		{"concurrency", func(options *config.Options) {
			options.Requests = absoluteCeilings.Concurrency + 1
			options.ConcurrencyStages = []int{1, absoluteCeilings.Concurrency + 1}
		}, StandardExperiments},
		{"timeout", func(options *config.Options) { options.Timeout = absoluteCeilings.Timeout + time.Second }, StandardExperiments},
		{"body limit", func(options *config.Options) { options.BodyLimit = absoluteCeilings.BodyLimit + 1 }, StandardExperiments},
		{"sample limit", func(options *config.Options) {
			options.IncludeSamples = true
			options.SampleLimit = absoluteCeilings.SampleLimit + 1
		}, StandardExperiments},
		{"redirect limit", func(options *config.Options) {
			options.FollowRedirects = true
			options.RedirectLimit = absoluteCeilings.RedirectLimit + 1
		}, StandardExperiments},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			effective := effectiveWith(func(options *config.Options) {
				options.AllowHighLoad = true
				options.AllowUnsafeMethods = true
				if test.mutate != nil {
					test.mutate(options)
				}
			})
			_, err := Evaluate(effective, test.plan(effective))
			assertPolicyError(t, err, AbsoluteLimit)
		})
	}
}

func TestExactCeilingsPass(t *testing.T) {
	effective := effectiveWith(func(options *config.Options) {
		options.Requests = normalCeilings.RequestsPerExperiment
		options.Rate = normalCeilings.Rate
		options.ConcurrencyStages = []int{normalCeilings.Concurrency}
		options.Timeout = normalCeilings.Timeout
		options.BodyLimit = normalCeilings.BodyLimit
		options.IncludeSamples = true
		options.SampleLimit = normalCeilings.SampleLimit
		options.FollowRedirects = true
		options.RedirectLimit = normalCeilings.RedirectLimit
	})
	if _, err := Evaluate(effective, StandardExperiments(effective)); err != nil {
		t.Fatalf("Evaluate() error at exact normal ceilings = %v", err)
	}
}

func assertPolicyError(t *testing.T, err error, want ErrorKind) {
	t.Helper()
	var policyErr *PolicyError
	if !errors.As(err, &policyErr) {
		t.Fatalf("error = %T %v, want *PolicyError", err, err)
	}
	if policyErr.Kind != want {
		t.Fatalf("kind = %q, want %q", policyErr.Kind, want)
	}
}
