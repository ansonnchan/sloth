package runner

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

func TestRunConcurrencyExperimentsRecordsAchievedLoad(t *testing.T) {
	var active atomic.Int32
	var serverPeak atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		current := active.Add(1)
		updatePeak(&serverPeak, current)
		defer active.Add(-1)
		time.Sleep(20 * time.Millisecond)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	effective, spec := concurrencyTestInputs(t, server.URL, []int{1, 3}, 6, 0)
	limiter, _ := NewStartLimiter(effective.Rate(), nil)
	budget, _ := NewRequestBudget(12)

	reports, err := RunConcurrencyExperiments(context.Background(), spec, effective, limiter, budget, ConcurrencyOptions{})
	if err != nil {
		t.Fatalf("RunConcurrencyExperiments() error = %v", err)
	}
	if len(reports) != 2 || reports[0].Result.Plan.Workers() != 1 || reports[1].Result.Plan.Workers() != 3 {
		t.Fatalf("stage reports = %#v", reports)
	}
	if reports[0].Aggregate.Concurrency.Peak != 1 || reports[1].Aggregate.Concurrency.Peak < 2 || reports[1].Aggregate.Concurrency.Peak > 3 {
		t.Fatalf("actual concurrency = %#v then %#v", reports[0].Aggregate.Concurrency, reports[1].Aggregate.Concurrency)
	}
	if reports[0].Result.Plan.Rate() != reports[1].Result.Plan.Rate() || limiter.Stats().Achieved != 12 || budget.Used() != 12 {
		t.Fatalf("shared controls changed: limiter %#v budget %d", limiter.Stats(), budget.Used())
	}
	if serverPeak.Load() > 3 {
		t.Fatalf("server observed %d in flight, want at most 3", serverPeak.Load())
	}
}

func TestRunConcurrencyExperimentsReturnsOrderedPartialStages(t *testing.T) {
	var requests atomic.Int32
	startedSecond := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if requests.Add(1) > 2 {
			startedSecond <- struct{}{}
			<-request.Context().Done()
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	effective, spec := concurrencyTestInputs(t, server.URL, []int{1, 2}, 2, 0)
	limiter, _ := NewStartLimiter(effective.Rate(), nil)
	budget, _ := NewRequestBudget(4)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-startedSecond
		cancel()
	}()

	reports, err := RunConcurrencyExperiments(ctx, spec, effective, limiter, budget, ConcurrencyOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunConcurrencyExperiments() error = %v", err)
	}
	if len(reports) != 2 || reports[0].Result.Plan.Workers() != 1 || reports[1].Result.Plan.Workers() != 2 {
		t.Fatalf("ordered partial reports = %#v", reports)
	}
	if !reports[1].Result.Execution.Interrupted {
		t.Fatalf("partial stage not marked interrupted: %#v", reports[1].Result.Execution)
	}
}

func concurrencyTestInputs(t *testing.T, target string, stages []int, requests, warmups int) (config.Effective, httpmeasure.RequestSpec) {
	t.Helper()
	options := config.Defaults()
	options.Requests, options.Warmups, options.Rate = requests, warmups, 1000
	options.ConcurrencyStages = stages
	options.Timeout = time.Second
	effective := config.Resolve(options)
	spec, err := httpmeasure.NewRequestSpec(httpmeasure.RequestSpecInput{
		Target: target, Method: http.MethodGet,
		Options: httpmeasure.RequestOptions{Timeout: options.Timeout, BodyLimit: 1024, RedirectLimit: 1},
	})
	if err != nil {
		t.Fatalf("NewRequestSpec() error = %v", err)
	}
	return effective, spec
}

func updatePeak(peak *atomic.Int32, current int32) {
	for {
		previous := peak.Load()
		if current <= previous || peak.CompareAndSwap(previous, current) {
			return
		}
	}
}
