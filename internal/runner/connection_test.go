package runner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

func TestConnectionOrderIsSeededAlternatingAndBalanced(t *testing.T) {
	first := connectionOrder(4, 1)
	second := connectionOrder(4, 2)
	if slices.Equal(first, second) {
		t.Fatalf("different seeds produced identical order: %v", first)
	}
	for _, order := range [][]ConnectionArm{first, second} {
		fresh, reused := 0, 0
		for index, arm := range order {
			if index > 0 && arm == order[index-1] {
				t.Fatalf("order is not alternating: %v", order)
			}
			if arm == ConnectionFresh {
				fresh++
			} else {
				reused++
			}
		}
		if fresh != 4 || reused != 4 {
			t.Fatalf("unbalanced order: %v", order)
		}
	}
}

func TestRunConnectionExperimentUsesObservedConnectionState(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("ok"))
	}))
	defer server.Close()
	effective, spec := connectionTestInputs(t, server.URL, 3, 1)
	limiter, _ := NewStartLimiter(effective.Rate(), nil)
	budget, _ := NewRequestBudget(8)

	report, err := RunConnectionExperiment(context.Background(), spec, effective, limiter, budget, ConnectionOptions{Seed: 7})
	if err != nil {
		t.Fatalf("RunConnectionExperiment() error = %v", err)
	}
	if len(report.Fresh.Result.Measured) != 3 || len(report.Reused.Result.Measured) != 3 {
		t.Fatalf("arm counts = %d fresh, %d reused", len(report.Fresh.Result.Measured), len(report.Reused.Result.Measured))
	}
	if report.Fresh.Aggregate.Connections.New != 3 || report.Reused.Aggregate.Connections.Reused == 0 || !report.Sufficient {
		t.Fatalf("connection evidence = fresh %#v reused %#v limitations %v", report.Fresh.Aggregate.Connections, report.Reused.Aggregate.Connections, report.Limitations)
	}
	if budget.Used() != 8 || limiter.Stats().Achieved != 8 {
		t.Fatalf("shared controls = budget %d limiter %#v", budget.Used(), limiter.Stats())
	}
}

func TestConnectionExperimentReportsInsufficientReuseEvidence(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Connection", "close")
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	effective, spec := connectionTestInputs(t, server.URL, 2, 0)
	limiter, _ := NewStartLimiter(effective.Rate(), nil)
	budget, _ := NewRequestBudget(4)

	report, err := RunConnectionExperiment(context.Background(), spec, effective, limiter, budget, ConnectionOptions{})
	if err != nil {
		t.Fatalf("RunConnectionExperiment() error = %v", err)
	}
	if report.Sufficient || len(report.Limitations) == 0 {
		t.Fatalf("report should be insufficient: %#v", report)
	}
}

func connectionTestInputs(t *testing.T, target string, requests, warmups int) (config.Effective, httpmeasure.RequestSpec) {
	t.Helper()
	options := config.Defaults()
	options.Requests, options.Warmups, options.Rate = requests, warmups, 1000
	options.Timeout = 2 * time.Second
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
