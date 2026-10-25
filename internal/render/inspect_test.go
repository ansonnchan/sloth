package render

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/inspect"
)

func TestInspectWideNoColorSnapshot(t *testing.T) {
	view := NewInspectView(exampleResult())
	var output bytes.Buffer
	if err := Inspect(&output, view, Options{Width: 100, ASCII: true}); err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	want := "GET https://example.com/api?token=%5BREDACTED%5D\n" +
		"Status: 200 OK\n" +
		"Protocol: HTTP/2.0\n" +
		"Connection: new\n" +
		"Response: 2.00 KiB\n" +
		"Total: 100.00 ms\n\n" +
		"Phases:\n" +
		"  DNS: 5.00 ms\n" +
		"  Connect: 10.00 ms\n" +
		"  TLS: 15.00 ms\n" +
		"  Request write: 1.00 ms\n" +
		"  First-byte wait: 60.00 ms\n" +
		"  Body transfer: 9.00 ms\n\n" +
		"Interpretation: The longest observed phase was the wait for the first response byte; it can include\n" +
		"                server work, network, queueing, and upstream delay.\n"
	if output.String() != want {
		t.Fatalf("output snapshot:\n%s\nwant:\n%s", output.String(), want)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatal("no-color snapshot contains ANSI")
	}
}

func TestInspectNarrowASCIIRemainsReadable(t *testing.T) {
	view := NewInspectView(exampleResult())
	var output bytes.Buffer
	if err := Inspect(&output, view, Options{Width: 36, ASCII: true}); err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}

	text := output.String()
	for _, required := range []string{"GET\n", "  https://example.com/api?token=", "Status: 200 OK", "First-byte wait: 60.00 ms", "Interpretation:"} {
		if !strings.Contains(text, required) {
			t.Fatalf("narrow output lacks %q:\n%s", required, text)
		}
	}
	if strings.Contains(text, "\x1b[") {
		t.Fatal("ASCII snapshot contains ANSI")
	}
}

func TestInspectRendersUnavailableAsNA(t *testing.T) {
	result := exampleResult()
	result.Timings.DNS = unavailablePhase(t)
	view := NewInspectView(result)
	if view.Phases[0].Value != "N/A (connection reused)" {
		t.Fatalf("DNS view = %q", view.Phases[0].Value)
	}
}

func TestInspectColorIsOptional(t *testing.T) {
	var output bytes.Buffer
	if err := Inspect(&output, NewInspectView(exampleResult()), Options{Width: 100, Color: true}); err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if !strings.Contains(output.String(), "\x1b[36m") {
		t.Fatal("color output lacks ANSI styling")
	}
}

func TestInterpretationDoesNotCallFirstByteServerTime(t *testing.T) {
	interpretation := NewInspectView(exampleResult()).Interpretation
	if strings.Contains(strings.ToLower(interpretation), "server time") || strings.Contains(strings.ToLower(interpretation), "server execution time") {
		t.Fatalf("interpretation overstates first-byte evidence: %q", interpretation)
	}
	if !strings.Contains(interpretation, "can include server work") {
		t.Fatalf("interpretation lacks caveat: %q", interpretation)
	}
}

func exampleResult() inspect.Result {
	target := httpmeasure.TargetIdentity{RedactedURL: "https://example.com/api?token=%5BREDACTED%5D"}
	return inspect.Result{
		Request:    httpmeasure.RequestMetadata{Method: "GET", Target: target},
		Outcome:    httpmeasure.ClassifyOutcome(200, 2048, nil),
		Protocol:   "HTTP/2.0",
		Connection: inspect.Connection{Observed: true},
		Body:       httpmeasure.BodyReadResult{TransferredBytes: 2048, Complete: true, ReuseEligible: true},
		Timings: httpmeasure.TimingPhases{
			Total:         measuredPhase(100 * time.Millisecond),
			DNS:           measuredPhase(5 * time.Millisecond),
			Connect:       measuredPhase(10 * time.Millisecond),
			TLS:           measuredPhase(15 * time.Millisecond),
			RequestWrite:  measuredPhase(time.Millisecond),
			FirstByteWait: measuredPhase(60 * time.Millisecond),
			BodyTransfer:  measuredPhase(9 * time.Millisecond),
		},
	}
}

func measuredPhase(value time.Duration) httpmeasure.PhaseDuration {
	base := time.Unix(1, 0)
	return httpmeasure.DeriveTimingPhases(httpmeasure.TraceRecord{
		RequestStart: base,
		BodyDone:     base.Add(value),
	}, false).Total
}

func unavailablePhase(t *testing.T) httpmeasure.PhaseDuration {
	t.Helper()
	base := time.Unix(1, 0)
	return httpmeasure.DeriveTimingPhases(httpmeasure.TraceRecord{
		RequestStart: base,
		BodyDone:     base.Add(time.Millisecond),
		Connections:  []httpmeasure.ConnectionObservation{{Reused: true}},
	}, true).DNS
}
