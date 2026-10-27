package cli

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/ansonnchan/sloth/internal/report"
)

func TestExportCommandWritesValidatedReportAndOnlyPathToStdout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "run.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export", "--output", path, "--requests", "10", "--warmups", "0", "--rate", "1000",
		"--concurrency", "1,2", "--allow-high-load", "--include-samples", "--sample-limit", "3",
		server.URL + "/api?token=secret",
	}, &stdout, &stderr, buildinfo.Info{Version: "test", Commit: "abc"})
	if code != int(ExitSuccess) {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
	if stdout.String() != path+"\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "Running baseline") || strings.Contains(stdout.String(), "Running") {
		t.Fatalf("streams = (%q, %q)", stdout.String(), stderr.String())
	}
	parsed, err := report.Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Report.ToolVersion != "test" || parsed.Report.Target.QueryKeys[0] != "token" || strings.Contains(parsed.Report.Target.RedactedURL, "secret") {
		t.Fatalf("report metadata = %+v / %+v", parsed.Report.Tool, parsed.Report.Target)
	}
	if parsed.Report.SampleExport == nil || parsed.Report.SampleExport.Included != 3 || !parsed.Report.SampleExport.Truncated || len(parsed.Report.Samples) != 3 {
		t.Fatalf("sample export = %+v, samples = %d", parsed.Report.SampleExport, len(parsed.Report.Samples))
	}
}

func TestExportCommandRequiresForceToReplace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	directory := t.TempDir()
	path := filepath.Join(directory, "run.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{
		"export", "--quiet", "--output", path, "--requests", "10", "--warmups", "0", "--rate", "1000",
		"--concurrency", "1,2", "--allow-high-load", server.URL,
	}
	var stdout, stderr bytes.Buffer
	if code := Run(args, &stdout, &stderr, buildinfo.Info{Version: "test"}); code != int(ExitRuntime) {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
	contents, err := os.ReadFile(path)
	if err != nil || string(contents) != "original" {
		t.Fatalf("contents = %q, error = %v", contents, err)
	}
	stdout.Reset()
	stderr.Reset()
	args = append(args[:1], append([]string{"--force"}, args[1:]...)...)
	if code := Run(args, &stdout, &stderr, buildinfo.Info{Version: "test"}); code != int(ExitSuccess) {
		t.Fatalf("forced Run() = %d, stderr = %q", code, stderr.String())
	}
	if _, err := report.Read(path); err != nil {
		t.Fatalf("forced report: %v", err)
	}
}

func TestExportCancellationLeavesNoFinalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cancelled.json")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := RunContext(ctx, []string{
		"export", "--output", path, "--requests", "10", "--warmups", "0", "--concurrency", "1,2", "https://example.com",
	}, &stdout, &stderr, buildinfo.Info{Version: "test"})
	if code != int(ExitInterrupted) {
		t.Fatalf("RunContext() = %d, stderr = %q", code, stderr.String())
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("final file exists: %v", err)
	}
}

func TestWriteExportReportRejectsInvalidValueWithoutFinalFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	err := writeExportReport(path, report.Report{}, false)
	if err == nil || !strings.Contains(err.Error(), "validate export report") {
		t.Fatalf("error = %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("final file exists: %v", statErr)
	}
}
