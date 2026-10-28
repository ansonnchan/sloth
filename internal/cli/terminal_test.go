package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ansonnchan/sloth/internal/buildinfo"
)

func TestNoColorFlagOverridesColorAlways(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"inspect", "--color", "always", "--no-color", server.URL}, &stdout, &stderr, buildinfo.Info{})
	if code != int(ExitSuccess) {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("--no-color output contains ANSI: %q", stdout.String())
	}
}

func TestNoColorEnvironmentDisablesExplicitColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"inspect", "--color", "always", server.URL}, &stdout, &stderr, buildinfo.Info{})
	if code != int(ExitSuccess) {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "\x1b[") {
		t.Fatalf("NO_COLOR output contains ANSI: %q", stdout.String())
	}
}

func TestPresentationFlagsAreConsistentPersistentOptions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"--help"}, &stdout, &stderr, buildinfo.Info{})
	if code != int(ExitSuccess) {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
	for _, flag := range []string{"--color", "--no-color", "--ascii"} {
		if !strings.Contains(stdout.String(), flag) {
			t.Fatalf("help lacks %s:\n%s", flag, stdout.String())
		}
	}
}
