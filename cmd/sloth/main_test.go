package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := execute([]string{"--help"}, &stdout, &stderr); err != nil {
		t.Fatalf("execute --help: %v", err)
	}

	if !strings.Contains(stdout.String(), "Sloth explains client-observable API latency") {
		t.Fatalf("help output does not describe Sloth:\n%s", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestVersion(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if err := execute([]string{"--version"}, &stdout, &stderr); err != nil {
		t.Fatalf("execute --version: %v", err)
	}

	if got, want := stdout.String(), "sloth dev\n"; got != want {
		t.Fatalf("version output = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}
