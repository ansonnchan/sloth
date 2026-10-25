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

func TestRunBaselineSuccessAndReuseMetrics(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("ok"))
	}))
	defer server.Close()

	report, err := runBaselineTest(context.Background(), t, server.URL, 3, 1, 2*time.Second)
	if err != nil {
		t.Fatalf("RunBaseline() error = %v", err)
	}
	if len(report.Result.Warmups) != 1 || len(report.Result.Measured) != 3 {
		t.Fatalf("result counts = %d warm-up, %d measured", len(report.Result.Warmups), len(report.Result.Measured))
	}
	if report.Aggregate.Outcomes.Successful != 3 || report.Aggregate.Latency.Count != 3 {
		t.Fatalf("aggregate = %#v", report.Aggregate)
	}
	if report.Aggregate.Connections.Reused == 0 {
		t.Fatalf("expected observed connection reuse: %#v", report.Aggregate.Connections)
	}
}

func TestRunBaselineMixedHTTPErrors(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if requests.Add(1)%2 == 0 {
			writer.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	report, err := runBaselineTest(context.Background(), t, server.URL, 4, 0, 2*time.Second)
	if err != nil {
		t.Fatalf("RunBaseline() error = %v", err)
	}
	if report.Aggregate.Outcomes.Successful != 2 || report.Aggregate.Outcomes.HTTPError != 2 || report.Aggregate.StatusClasses["5xx"] != 2 {
		t.Fatalf("mixed outcome aggregate = %#v", report.Aggregate)
	}
}

func TestRunBaselineTimeoutIsMeasured(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	report, err := runBaselineTest(context.Background(), t, server.URL, 1, 0, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("RunBaseline() error = %v", err)
	}
	if report.Aggregate.Outcomes.Timeout != 1 || len(report.Result.Measured) != 1 {
		t.Fatalf("timeout aggregate = %#v", report.Aggregate)
	}
}

func TestRunBaselineCancellationReturnsPartialResult(t *testing.T) {
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started <- struct{}{}
		<-request.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-started
		cancel()
	}()

	report, err := runBaselineTest(ctx, t, server.URL, 5, 0, time.Second)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunBaseline() error = %v, want cancellation", err)
	}
	if !report.Result.Execution.Interrupted || len(report.Result.Measured) > 1 {
		t.Fatalf("cancellation result = %#v", report.Result)
	}
}

func runBaselineTest(ctx context.Context, t *testing.T, target string, requests, warmups int, timeout time.Duration) (ExperimentReport, error) {
	t.Helper()
	options := config.Defaults()
	options.Requests = requests
	options.Warmups = warmups
	options.Rate = 1000
	options.Timeout = timeout
	effective := config.Resolve(options)
	spec, err := httpmeasure.NewRequestSpec(httpmeasure.RequestSpecInput{
		Target: target, Method: http.MethodGet,
		Options: httpmeasure.RequestOptions{Timeout: timeout, BodyLimit: 1024, RedirectLimit: 1},
	})
	if err != nil {
		t.Fatalf("NewRequestSpec() error = %v", err)
	}
	limiter, _ := NewStartLimiter(effective.Rate(), nil)
	budget, _ := NewRequestBudget(requests + warmups)
	return RunBaseline(ctx, spec, effective, limiter, budget, BaselineOptions{})
}
