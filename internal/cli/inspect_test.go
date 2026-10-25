package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/buildinfo"
)

func TestInspectCommandSupportsDocumentedMethods(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			received := make(chan string, 1)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				received <- request.Method
				writer.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			args := []string{"inspect", "--method", method}
			if method != http.MethodGet && method != http.MethodHead {
				args = append(args, "--allow-unsafe-methods")
			}
			args = append(args, server.URL)
			var stdout, stderr bytes.Buffer
			code := Run(args, &stdout, &stderr, buildinfo.Info{Version: "test"})

			if code != 0 {
				t.Fatalf("Run() code = %d, stderr = %q", code, stderr.String())
			}
			if got := <-received; got != method {
				t.Fatalf("server method = %q, want %q", got, method)
			}
			if !strings.Contains(stdout.String(), method+" ") || !strings.Contains(stdout.String(), "Status: 200") {
				t.Fatalf("stdout = %q", stdout.String())
			}
		})
	}
}

func TestInspectCommandHonorsHeadersBodyFileAndRedirects(t *testing.T) {
	bodyPath := filepath.Join(t.TempDir(), "request.txt")
	if err := os.WriteFile(bodyPath, []byte("request body"), 0o600); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	finalRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/start" {
			http.Redirect(writer, request, "/final", http.StatusTemporaryRedirect)
			return
		}
		finalRequests++
		if request.Header.Get("X-Sloth-Test") != "present" {
			t.Error("custom header missing")
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if string(body) != "request body" {
			t.Errorf("request body = %q", body)
		}
		writer.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	args := []string{
		"inspect", "--method", http.MethodPost, "--allow-unsafe-methods",
		"--header", "X-Sloth-Test: present", "--body-file", bodyPath,
		"--follow-redirects", server.URL + "/start",
	}
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr, buildinfo.Info{Version: "test"})
	if code != 0 || finalRequests != 1 {
		t.Fatalf("Run() = %d, final requests = %d, stderr = %q", code, finalRequests, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Status: 201") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestInspectCommandRejectsUnsafeMethodBeforeNetwork(t *testing.T) {
	var requests atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"inspect", "--method", http.MethodPost, server.URL}, &stdout, &stderr, buildinfo.Info{Version: "test"})
	if code != int(ExitUsage) || requests.Load() != 0 || stdout.Len() != 0 {
		t.Fatalf("Run() = %d, requests = %d, stdout = %q", code, requests.Load(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "--allow-unsafe-methods") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestInspectCommandTimesOutPromptly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
		case <-time.After(time.Second):
			writer.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	started := time.Now()
	var stdout, stderr bytes.Buffer
	code := Run([]string{"inspect", "--timeout", "20ms", server.URL}, &stdout, &stderr, buildinfo.Info{Version: "test"})
	if code != int(ExitRuntime) || time.Since(started) > 500*time.Millisecond {
		t.Fatalf("Run() = %d after %s, stderr = %q", code, time.Since(started), stderr.String())
	}
	if stdout.Len() != 0 || stderr.String() != "request timed out\n" {
		t.Fatalf("streams = (%q, %q)", stdout.String(), stderr.String())
	}
}

func TestInspectCommandHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer
	code := RunContext(ctx, []string{"inspect", "https://example.com"}, &stdout, &stderr, buildinfo.Info{Version: "test"})
	if code != int(ExitInterrupted) || stdout.Len() != 0 || stderr.String() != "request was cancelled\n" {
		t.Fatalf("RunContext() = %d, streams = (%q, %q)", code, stdout.String(), stderr.String())
	}
}

func TestInspectCommandValidatesBeforeOpeningBodyFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"inspect", "--body-file", filepath.Join(t.TempDir(), "missing"), "not-a-url",
	}, &stdout, &stderr, buildinfo.Info{Version: "test"})
	if code != int(ExitUsage) || !strings.Contains(stderr.String(), "invalid target") {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
}
