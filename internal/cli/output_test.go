package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestOutputSeparatesDataAndDiagnostics(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	output := Output{Data: &stdout, Diagnostics: &stderr}

	if err := output.WriteJSON(map[string]string{"status": "\x1b[32mok"}); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	if err := output.Progressf("measuring\n"); err != nil {
		t.Fatalf("Progressf() error = %v", err)
	}
	if err := output.Warningf("warning\n"); err != nil {
		t.Fatalf("Warningf() error = %v", err)
	}

	if got := stdout.String(); got != "{\"status\":\"\\u001b[32mok\"}\n" || strings.Contains(got, "\x1b[") {
		t.Fatalf("stdout = %q", got)
	}
	if got := stderr.String(); got != "measuring\nwarning\n" {
		t.Fatalf("stderr = %q", got)
	}
}

func TestQuietSuppressesProgressButNotWarnings(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	output := Output{Data: &stdout, Diagnostics: &stderr, Quiet: true}

	if err := output.Progressf("hidden\n"); err != nil {
		t.Fatalf("Progressf() error = %v", err)
	}
	if err := output.Warningf("visible\n"); err != nil {
		t.Fatalf("Warningf() error = %v", err)
	}

	if stdout.Len() != 0 || stderr.String() != "visible\n" {
		t.Fatalf("stdout = %q, stderr = %q", stdout.String(), stderr.String())
	}
}
