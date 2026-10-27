package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ansonnchan/sloth/internal/buildinfo"
)

func TestDiagnoseCommandRunsStandardSuiteAndShowsPlanFirst(t *testing.T) {
	var requests atomic.Int64
	var planSeen atomic.Bool
	var planBeforeTraffic atomic.Bool
	stderr := &trackingBuffer{needle: "50 total planned requests", seen: &planSeen}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			planBeforeTraffic.Store(planSeen.Load())
		}
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	code := Run([]string{
		"diagnose", "--requests", "10", "--warmups", "0", "--rate", "1000",
		"--concurrency", "1,2", "--allow-high-load", server.URL,
	}, &stdout, stderr, buildinfo.Info{Version: "test"})
	if code != int(ExitSuccess) {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
	if requests.Load() != 50 || !planBeforeTraffic.Load() {
		t.Fatalf("requests = %d, plan seen before traffic = %t; stderr = %q", requests.Load(), planBeforeTraffic.Load(), stderr.String())
	}
	for _, content := range []string{"Configuration:", "minimum schedule time", "Running baseline", "Running connection comparison", "Running concurrency stages"} {
		if !strings.Contains(stderr.String(), content) {
			t.Fatalf("stderr = %q, want %q", stderr.String(), content)
		}
	}
	if !strings.Contains(stdout.String(), "Sloth diagnosis") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestDiagnoseJSONKeepsProgressOffStdout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"diagnose", "--json", "--color", "always", "--requests", "10", "--warmups", "0",
		"--rate", "1000", "--concurrency", "1,2", "--allow-high-load", server.URL,
	}, &stdout, &stderr, buildinfo.Info{Version: "test"})
	if code != int(ExitSuccess) {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
	var document struct {
		SchemaVersion string `json:"schema_version"`
		Experiments   []any  `json:"experiments"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("JSON = %q: %v", stdout.String(), err)
	}
	if document.SchemaVersion != "sloth.diagnose/v1" || len(document.Experiments) != 5 {
		t.Fatalf("document = %+v", document)
	}
	if strings.Contains(stdout.String(), "Running ") || strings.Contains(stdout.String(), "\x1b[") || !strings.Contains(stderr.String(), "Running baseline") {
		t.Fatalf("streams = (%q, %q)", stdout.String(), stderr.String())
	}
}

type trackingBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
	needle string
	seen   *atomic.Bool
}

func (buffer *trackingBuffer) Write(value []byte) (int, error) {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	if strings.Contains(string(value), buffer.needle) {
		buffer.seen.Store(true)
	}
	return buffer.buffer.Write(value)
}

func (buffer *trackingBuffer) String() string {
	buffer.mu.Lock()
	defer buffer.mu.Unlock()
	return buffer.buffer.String()
}

func TestDiagnoseCommandValidatesPlanBeforeNetwork(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { requests.Add(1) }))
	defer server.Close()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"diagnose", "--requests", "1", "--concurrency", "1,2", server.URL},
		&stdout, &stderr, buildinfo.Info{Version: "test"})
	if code != int(ExitUsage) || requests.Load() != 0 || stdout.Len() != 0 {
		t.Fatalf("Run() = %d, requests = %d, stdout = %q", code, requests.Load(), stdout.String())
	}
}

func TestDiagnoseCommandReturnsPartialDiagnosisOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := RunContext(ctx, []string{
		"diagnose", "--requests", "10", "--warmups", "0", "--concurrency", "1,2", "https://example.com",
	}, &stdout, &stderr, buildinfo.Info{Version: "test"})
	if code != int(ExitInterrupted) {
		t.Fatalf("RunContext() = %d, stderr = %q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "INSUFFICIENT_EVIDENCE") || !strings.Contains(stderr.String(), "context canceled") {
		t.Fatalf("streams = (%q, %q)", stdout.String(), stderr.String())
	}
}
