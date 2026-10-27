package diagnose

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

func TestServiceRunsStandardSuiteInOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	options := config.Defaults()
	options.Requests = 10
	options.Warmups = 0
	options.Rate = 1_000
	options.ConcurrencyStages = []int{1, 2}
	effective := config.Resolve(options)
	spec, err := httpmeasure.NewRequestSpec(httpmeasure.RequestSpecInput{
		Target: server.URL, Method: effective.Method(), Options: httpmeasure.RequestOptions{
			Timeout: effective.Timeout(), BodyLimit: effective.BodyLimit(), RedirectLimit: effective.RedirectLimit(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	var stages []Stage
	result, err := NewService(func(stage Stage) { stages = append(stages, stage) }).Run(context.Background(), spec, effective)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Stage{StageBaseline, StageConnection, StageConcurrency}; !reflect.DeepEqual(stages, want) {
		t.Fatalf("stages = %v, want %v", stages, want)
	}
	if result.Suite.Baseline.Aggregate.Outcomes.Attempted != 10 ||
		result.Suite.Connection.Fresh.Aggregate.Outcomes.Attempted != 10 ||
		result.Suite.Connection.Reused.Aggregate.Outcomes.Attempted != 10 || len(result.Suite.Concurrency) != 2 {
		t.Fatalf("suite did not complete planned arms: %+v", result.Suite)
	}
}

func TestServiceDiagnosesCancelledPartialEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	options := config.Defaults()
	options.Requests = 10
	options.Warmups = 0
	options.ConcurrencyStages = []int{1, 2}
	effective := config.Resolve(options)
	spec, err := httpmeasure.NewRequestSpec(httpmeasure.RequestSpecInput{
		Target: "https://example.com", Method: effective.Method(), Options: httpmeasure.RequestOptions{
			Timeout: effective.Timeout(), BodyLimit: effective.BodyLimit(), RedirectLimit: effective.RedirectLimit(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := NewService(nil).Run(ctx, spec, effective)
	if err == nil {
		t.Fatal("Run() error = nil")
	}
	if len(result.Diagnosis.Findings) != 1 || result.Diagnosis.Findings[0].Code != "INSUFFICIENT_EVIDENCE" {
		t.Fatalf("findings = %+v", result.Diagnosis.Findings)
	}
}
